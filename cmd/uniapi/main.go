package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sooneocean/uniapi/internal/audit"
	"github.com/sooneocean/uniapi/internal/auth"
	"github.com/sooneocean/uniapi/internal/background"
	"github.com/sooneocean/uniapi/internal/cache"
	"github.com/sooneocean/uniapi/internal/config"
	"github.com/sooneocean/uniapi/internal/crypto"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/handler"
	"github.com/sooneocean/uniapi/internal/logger"
	"github.com/sooneocean/uniapi/internal/oauth"
	"github.com/sooneocean/uniapi/internal/plugin"
	"github.com/sooneocean/uniapi/internal/provider"
	pAnthropic "github.com/sooneocean/uniapi/internal/provider/anthropic"
	pGemini "github.com/sooneocean/uniapi/internal/provider/gemini"
	pOpenai "github.com/sooneocean/uniapi/internal/provider/openai"
	"github.com/sooneocean/uniapi/internal/rag"
	"github.com/sooneocean/uniapi/internal/repo"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
	"github.com/sooneocean/uniapi/internal/web"
	"github.com/sooneocean/uniapi/internal/webhook"
)

var version = "dev"

// credentialCache is a thread-safe cache for account credentials.
type credentialCache struct {
	mu          sync.RWMutex
	cred        string
	authType    string
	lastRefresh time.Time
	accountID   string
	accountRepo *repo.AccountRepo
}

func newCredentialCache(accID string, initialCred, initialAuthType string, accountRepo *repo.AccountRepo) *credentialCache {
	return &credentialCache{
		cred:        initialCred,
		authType:    initialAuthType,
		lastRefresh: time.Now(),
		accountID:   accID,
		accountRepo: accountRepo,
	}
}

func (cc *credentialCache) Get() (string, string) {
	cc.mu.RLock()
	if time.Since(cc.lastRefresh) <= 5*time.Minute {
		cred, authType := cc.cred, cc.authType
		cc.mu.RUnlock()
		return cred, authType
	}
	cc.mu.RUnlock()

	// Need refresh — acquire write lock.
	cc.mu.Lock()
	defer cc.mu.Unlock()
	// Double-check after acquiring write lock.
	if time.Since(cc.lastRefresh) <= 5*time.Minute {
		return cc.cred, cc.authType
	}

	fresh, err := cc.accountRepo.GetByID(cc.accountID)
	if err != nil {
		return cc.cred, cc.authType // return stale on error
	}
	cc.cred = fresh.Credential
	cc.authType = fresh.AuthType
	cc.lastRefresh = time.Now()
	return cc.cred, cc.authType
}

func main() {
	port := flag.Int("port", 0, "server port")
	dataDir := flag.String("data-dir", "", "data directory")
	secret := flag.String("secret", "", "encryption secret")
	cfgPath := flag.String("config", "", "config file path")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		fmt.Println("UniAPI " + version)
		return
	}

	// Load config
	if *cfgPath == "" {
		home, _ := os.UserHomeDir()
		defaultCfg := filepath.Join(home, ".uniapi", "config.yaml")
		if _, err := os.Stat(defaultCfg); err == nil {
			*cfgPath = defaultCfg
		}
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil && *cfgPath != "" {
		slog.Error("config", "error", err)
		os.Exit(1)
	}
	if cfg == nil {
		cfg = &config.Config{}
		cfg.Server.Port = 9000
		cfg.Server.Host = "0.0.0.0"
		cfg.Routing.Strategy = "round_robin"
		cfg.Routing.MaxRetries = 3
		cfg.Routing.FailoverAttempts = 2
	}

	// Init structured logger
	logger.Init(cfg.LogLevel)

	// CLI overrides
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *secret != "" {
		cfg.Security.Secret = *secret
	}

	// Data dir
	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(home, ".uniapi")
	}
	os.MkdirAll(cfg.DataDir, 0700)

	// Secret
	if cfg.Security.Secret == "" {
		secretPath := filepath.Join(cfg.DataDir, "secret")
		cfg.Security.Secret, err = crypto.LoadOrCreateSecret(secretPath)
		if err != nil {
			slog.Error("secret", "error", err)
			os.Exit(1)
		}
	}

	// Database
	dbPath := filepath.Join(cfg.DataDir, "data.db")
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Repos
	userRepo := repo.NewUserRepo(database)
	encKey, err := crypto.DeriveKeyWithInfo(cfg.Security.Secret, "uniapi-encryption")
	if err != nil {
		slog.Error("derive enc key", "error", err)
		os.Exit(1)
	}
	accountRepo := repo.NewAccountRepo(database, encKey)
	convoRepo := repo.NewConversationRepo(database)
	recorder := usage.NewRecorder(database.DB)
	defer recorder.Stop()

	// OAuth manager
	oauthMgr := oauth.NewManager(database, accountRepo, encKey, cfg.OAuth.BaseURL, cfg.OAuth)

	// Background tasks
	bgTasks := background.New(database.DB, cfg.Storage.RetentionDays, oauthMgr)
	bgTasks.Start()
	defer bgTasks.Stop()

	// Cache
	memCache := cache.New()
	defer memCache.Stop()

	// Router
	rtr := router.New(memCache, router.Config{
		Strategy: cfg.Routing.Strategy, MaxRetries: cfg.Routing.MaxRetries, FailoverAttempts: cfg.Routing.FailoverAttempts,
	})

	// Register config-managed providers
	for _, pc := range cfg.Providers {
		for _, acc := range pc.Accounts {
			var p provider.Provider
			maxConc := acc.MaxConcurrent
			if maxConc == 0 {
				maxConc = 5
			}
			provCfg := provider.ProviderConfig{Name: pc.Name, Type: pc.Type, BaseURL: pc.BaseURL}
			apiKey := acc.APIKey
			credFunc := func() (string, string) { return apiKey, "api_key" }
			switch pc.Type {
			case "anthropic":
				p = pAnthropic.NewAnthropic(provCfg, acc.Models, credFunc)
			case "openai":
				p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc)
			case "gemini":
				p = pGemini.NewGemini(provCfg, acc.Models, credFunc)
			case "openai_compatible":
				p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc)
			default:
				slog.Warn("unknown provider type", "type", pc.Type)
				continue
			}
			accountID := fmt.Sprintf("%s-%s", pc.Name, acc.Label)
			rtr.AddAccount(accountID, p, maxConc)
			slog.Info("registered provider", "name", pc.Name, "label", acc.Label, "models", len(acc.Models))
		}
	}

	// Load DB-managed accounts (API key + OAuth/session) into router
	dbAccounts, err := accountRepo.ListAll()
	if err != nil {
		slog.Error("failed to load DB accounts", "error", err)
	} else {
		for _, acc := range dbAccounts {
			if !acc.Enabled || acc.NeedsReauth || acc.ConfigManaged {
				continue
			}
			cache := newCredentialCache(acc.ID, acc.Credential, acc.AuthType, accountRepo)
			credFunc := cache.Get
			provCfg := provider.ProviderConfig{Name: acc.Provider, Type: acc.Provider}
			var p provider.Provider
			switch acc.Provider {
			case "openai":
				p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc)
			case "anthropic":
				p = pAnthropic.NewAnthropic(provCfg, acc.Models, credFunc)
			case "gemini":
				p = pGemini.NewGemini(provCfg, acc.Models, credFunc)
			default:
				p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc) // openai_compatible
			}
			rtr.AddAccountWithOwner(acc.ID, p, acc.MaxConcurrent, acc.OwnerUserID)
			slog.Info("loaded DB account", "id", acc.ID, "provider", acc.Provider)
		}
	}

	// RAG manager
	ragMgr := rag.NewManager(database.DB)

	// Plugin manager
	pluginMgr := plugin.NewManager(database.DB)

	// Webhook manager
	webhookCfgs := make([]webhook.WebhookConfig, len(cfg.Webhooks))
	for i, wh := range cfg.Webhooks {
		webhookCfgs[i] = webhook.WebhookConfig{URL: wh.URL, Events: wh.Events}
	}
	webhookMgr := webhook.NewManager(webhookCfgs)

	// Auth
	jwtKey, err := crypto.DeriveKeyWithInfo(cfg.Security.Secret, "uniapi-jwt-signing")
	if err != nil {
		slog.Error("derive jwt key", "error", err)
		os.Exit(1)
	}
	jwtMgr := auth.NewJWTManager(jwtKey, 7*24*time.Hour)

	// registerAccount dynamically adds newly bound accounts to the live router
	registerAccount := func(acc *repo.Account) {
		cache := newCredentialCache(acc.ID, acc.Credential, acc.AuthType, accountRepo)
		credFunc := cache.Get
		provCfg := provider.ProviderConfig{Name: acc.Provider, Type: acc.Provider}
		var p provider.Provider
		switch acc.Provider {
		case "openai":
			p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc)
		case "anthropic":
			p = pAnthropic.NewAnthropic(provCfg, acc.Models, credFunc)
		case "gemini":
			p = pGemini.NewGemini(provCfg, acc.Models, credFunc)
		default:
			p = pOpenai.NewOpenAI(provCfg, acc.Models, credFunc) // openai_compatible
		}
		rtr.AddAccountWithOwner(acc.ID, p, acc.MaxConcurrent, acc.OwnerUserID)
	}

	// Audit logger
	auditLogger := audit.NewLogger(database.DB)

	// Gin
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(handler.RequestIDMiddleware())
	engine.Use(handler.RequestLogMiddleware())
	engine.Use(handler.CORSMiddleware(cfg.Server.CORSOrigins))
	engine.Use(handler.CSRFMiddleware())
	engine.Use(handler.MetricsMiddleware())

	// Response cache
	cacheTTL := cfg.ResponseCache.TTL
	if cacheTTL == 0 {
		cacheTTL = 300
	}
	respCache := handler.NewResponseCache(memCache, time.Duration(cacheTTL)*time.Second, cfg.ResponseCache.Enabled)

	// Auth routes
	authHandler := handler.NewAuthHandler(userRepo, jwtMgr, database, auditLogger)
	authHandler.SetWebhookManager(webhookMgr)
	loginLimiter := handler.RateLimitMiddleware(memCache, 10, 1*time.Minute) // 10 attempts per minute
	api := engine.Group("/api")
	api.GET("/status", authHandler.Status)
	api.POST("/setup", loginLimiter, authHandler.Setup)
	api.POST("/login", loginLimiter, authHandler.Login)
	api.POST("/logout", authHandler.Logout)

	// Protected auth routes
	apiAuth := api.Group("")
	apiAuth.Use(handler.JWTAuthMiddleware(jwtMgr))
	apiAuth.GET("/me", authHandler.Me)

	// Settings handler (providers, users, API keys)
	settingsHandler := handler.NewSettingsHandler(accountRepo, userRepo, convoRepo, recorder, database, auditLogger, registerAccount, rtr)

	// Conversation handler
	convoHandler := handler.NewConversationHandler(convoRepo, rtr, auditLogger)

	// System prompt handler
	systemPromptRepo := repo.NewSystemPromptRepo(database)
	promptHandler := handler.NewSystemPromptHandler(systemPromptRepo)

	// Usage handler
	usageHandler := handler.NewUsageHandler(database, recorder, auditLogger)

	// Admin handler
	adminHandler := handler.NewAdminHandler(database, convoRepo, systemPromptRepo, auditLogger)

	// Provider management (admin only)
	apiAuth.GET("/providers", settingsHandler.ListProviders)
	apiAuth.POST("/providers", settingsHandler.AddProvider)
	apiAuth.DELETE("/providers/:id", settingsHandler.DeleteProvider)
	apiAuth.GET("/provider-templates", settingsHandler.ListTemplates)

	// User management (admin only)
	apiAuth.GET("/users", settingsHandler.ListUsers)
	apiAuth.POST("/users", settingsHandler.CreateUser)
	apiAuth.DELETE("/users/:id", settingsHandler.DeleteUser)
	apiAuth.PUT("/users/:id/quotas", settingsHandler.UpdateUserQuotas)

	// API key management
	apiAuth.GET("/api-keys", settingsHandler.ListAPIKeys)
	apiAuth.POST("/api-keys", settingsHandler.CreateAPIKey)
	apiAuth.DELETE("/api-keys/:id", settingsHandler.DeleteAPIKey)

	// Conversation management
	apiAuth.GET("/conversations", convoHandler.ListConversations)
	apiAuth.POST("/conversations", convoHandler.CreateConversation)
	apiAuth.GET("/conversations/:id", convoHandler.GetConversation)
	apiAuth.PUT("/conversations/:id", convoHandler.UpdateConversation)
	apiAuth.DELETE("/conversations/:id", convoHandler.DeleteConversation)
	apiAuth.POST("/conversations/:id/messages", convoHandler.AddMessage)
	apiAuth.DELETE("/conversations/:id/messages/:msgId", convoHandler.DeleteMessageAndAfter)
	apiAuth.GET("/conversations/:id/export", convoHandler.ExportConversation)
	apiAuth.POST("/conversations/:id/auto-title", convoHandler.AutoTitle)
	apiAuth.PUT("/conversations/:id/folder", convoHandler.UpdateConversationFolder)
	apiAuth.PUT("/conversations/:id/pin", convoHandler.ToggleConversationPin)
	apiAuth.POST("/conversations/:id/share", convoHandler.ShareConversation)
	apiAuth.DELETE("/conversations/:id/share", convoHandler.UnshareConversation)
	engine.GET("/api/shared/:token", convoHandler.GetSharedConversation)

	// System prompts
	apiAuth.GET("/system-prompts", promptHandler.ListSystemPrompts)
	apiAuth.POST("/system-prompts", promptHandler.CreateSystemPrompt)
	apiAuth.PUT("/system-prompts/:id", promptHandler.UpdateSystemPrompt)
	apiAuth.DELETE("/system-prompts/:id", promptHandler.DeleteSystemPrompt)

	// Usage
	apiAuth.GET("/usage", usageHandler.GetUsage)
	apiAuth.GET("/usage/all", usageHandler.GetAllUsage)
	apiAuth.GET("/usage/analytics", usageHandler.UsageAnalytics)

	// OAuth routes
	oauthHandler := handler.NewOAuthHandler(oauthMgr, rtr, registerAccount, auditLogger)
	oauthHandler.SetWebhookManager(webhookMgr)
	oauthGroup := engine.Group("/api/oauth")
	oauthGroup.GET("/callback/:provider", oauthHandler.Callback) // NO auth — uses state token

	oauthAuth := oauthGroup.Group("")
	oauthAuth.Use(handler.JWTAuthMiddleware(jwtMgr))
	oauthAuth.GET("/providers", oauthHandler.ListProviders)
	oauthAuth.GET("/accounts", oauthHandler.ListAccounts)
	oauthAuth.DELETE("/accounts/:id", oauthHandler.UnbindAccount)
	oauthAuth.POST("/accounts/:id/reauth", oauthHandler.Reauth)

	// Bind routes under /bind/:provider to avoid Gin wildcard conflicts
	bindGroup := oauthAuth.Group("/bind/:provider")
	bindGroup.GET("/authorize", oauthHandler.Authorize)
	bindGroup.POST("/session-token", oauthHandler.BindSessionToken)

	// Model alias handler
	modelAliasHandler := handler.NewModelAliasHandlerWithCache(database.DB, memCache)
	apiAuth.GET("/model-aliases", modelAliasHandler.ListModelAliases)
	apiAuth.POST("/model-aliases", modelAliasHandler.CreateModelAlias)
	apiAuth.DELETE("/model-aliases/:alias", modelAliasHandler.DeleteModelAlias)

	// Knowledge base routes
	knowledgeHandler := handler.NewKnowledgeHandler(ragMgr)
	apiAuth.POST("/knowledge", knowledgeHandler.Upload)
	apiAuth.GET("/knowledge", knowledgeHandler.List)
	apiAuth.DELETE("/knowledge/:id", knowledgeHandler.Delete)

	// Plugin routes
	pluginHandler := handler.NewPluginHandler(pluginMgr)
	apiAuth.GET("/plugins", pluginHandler.List)
	apiAuth.POST("/plugins", pluginHandler.Register)
	apiAuth.DELETE("/plugins/:id", pluginHandler.Delete)
	apiAuth.POST("/plugins/:id/test", pluginHandler.Test)

	// Prompt templates
	templatesHandler := handler.NewTemplatesHandler(database)
	apiAuth.GET("/templates", templatesHandler.List)
	apiAuth.POST("/templates", templatesHandler.Create)
	apiAuth.PUT("/templates/:id", templatesHandler.Update)
	apiAuth.DELETE("/templates/:id", templatesHandler.Delete)
	apiAuth.POST("/templates/:id/use", templatesHandler.Use)

	// Data export / import
	apiAuth.GET("/export", adminHandler.ExportUserData)
	apiAuth.POST("/import", adminHandler.ImportUserData)

	// Chat rooms routes
	roomsHandler := handler.NewRoomsHandler(database.DB, rtr)
	apiAuth.POST("/rooms", roomsHandler.Create)
	apiAuth.GET("/rooms", roomsHandler.List)
	apiAuth.POST("/rooms/:id/join", roomsHandler.Join)
	apiAuth.GET("/rooms/:id/messages", roomsHandler.GetMessages)
	apiAuth.POST("/rooms/:id/messages", roomsHandler.SendMessage)
	apiAuth.DELETE("/rooms/:id", roomsHandler.Delete)
	apiAuth.GET("/rooms/:id/members", roomsHandler.GetMembers)
	apiAuth.GET("/rooms/:id/stream", roomsHandler.StreamRoom)

	// Workflows routes
	workflowsHandler := handler.NewWorkflowsHandler(database.DB, rtr)
	apiAuth.GET("/workflows", workflowsHandler.List)
	apiAuth.POST("/workflows", workflowsHandler.Create)
	apiAuth.PUT("/workflows/:id", workflowsHandler.Update)
	apiAuth.DELETE("/workflows/:id", workflowsHandler.Delete)
	apiAuth.POST("/workflows/:id/run", workflowsHandler.Run)

	// Themes routes
	themesHandler := handler.NewThemesHandler(database.DB)
	apiAuth.GET("/themes", themesHandler.List)
	apiAuth.POST("/themes", themesHandler.Create)
	apiAuth.DELETE("/themes/:id", themesHandler.Delete)
	apiAuth.PUT("/themes/:id/apply", themesHandler.Apply)

	// API routes
	apiHandler := handler.NewAPIHandlerWithCache(rtr, recorder, webhookMgr, respCache, database.DB, memCache)
	apiHandler.SetRAGManager(ragMgr)
	apiHandler.SetPluginManager(pluginMgr)
	v1 := engine.Group("/v1")
	v1.Use(handler.APIKeyAuthMiddleware(database.DB, jwtMgr, memCache))
	apiLimiter := handler.RateLimitMiddleware(memCache, 60, 1*time.Minute) // 60 req/min per IP
	v1.Use(apiLimiter)
	v1.POST("/chat/completions", apiHandler.ChatCompletions)
	v1.GET("/models", apiHandler.ListModels)
	v1.POST("/compare", apiHandler.CompareModels)

	// Health
	engine.GET("/health", func(c *gin.Context) {
		dbOK := database.DB.Ping() == nil
		modelCount := len(rtr.AllModels())

		status := "ok"
		code := 200
		if !dbOK {
			status = "unhealthy"
			code = 503
		}

		c.JSON(code, gin.H{
			"status":    status,
			"db":        map[bool]string{true: "connected", false: "disconnected"}[dbOK],
			"models":    modelCount,
			"providers": len(cfg.Providers),
			"version":   version,
		})
	})

	// Prometheus metrics (admin only)
	apiAuth.GET("/metrics", func(c *gin.Context) {
		role, _ := c.Get("role")
		if r, ok := role.(string); !ok || r != "admin" {
			c.JSON(403, gin.H{"error": "admin only"})
			return
		}
		promhttp.Handler().ServeHTTP(c.Writer, c.Request)
	})

	// Audit log (admin only)
	apiAuth.GET("/audit-log", adminHandler.GetAuditLog)

	// Admin dashboard
	apiAuth.GET("/dashboard", usageHandler.Dashboard)

	// Database backup (admin only)
	apiAuth.GET("/backup", adminHandler.BackupDB)

	web.RegisterFrontend(engine)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // long for streaming
		IdleTimeout:  60 * time.Second,
	}
	slog.Info("UniAPI starting", "addr", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")

	// Graceful shutdown with 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
