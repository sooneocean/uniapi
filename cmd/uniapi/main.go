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
	"github.com/sooneocean/uniapi/internal/provider"
	pAnthropic "github.com/sooneocean/uniapi/internal/provider/anthropic"
	pGemini "github.com/sooneocean/uniapi/internal/provider/gemini"
	pOpenai "github.com/sooneocean/uniapi/internal/provider/openai"
	"github.com/sooneocean/uniapi/internal/repo"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
	"github.com/sooneocean/uniapi/internal/web"
)

func main() {
	port := flag.Int("port", 0, "server port")
	dataDir := flag.String("data-dir", "", "data directory")
	secret := flag.String("secret", "", "encryption secret")
	cfgPath := flag.String("config", "", "config file path")
	flag.Parse()

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
			accID := acc.ID
			credFunc := func() (string, string) {
				fresh, err := accountRepo.GetByID(accID)
				if err != nil {
					return "", "api_key"
				}
				return fresh.Credential, fresh.AuthType
			}
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

	// Auth
	jwtKey, err := crypto.DeriveKeyWithInfo(cfg.Security.Secret, "uniapi-jwt-signing")
	if err != nil {
		slog.Error("derive jwt key", "error", err)
		os.Exit(1)
	}
	jwtMgr := auth.NewJWTManager(jwtKey, 7*24*time.Hour)

	// registerAccount dynamically adds newly bound accounts to the live router
	registerAccount := func(acc *repo.Account) {
		accID := acc.ID
		credFunc := func() (string, string) {
			fresh, err := accountRepo.GetByID(accID)
			if err != nil {
				return "", "api_key"
			}
			return fresh.Credential, fresh.AuthType
		}
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

	// Auth routes
	authHandler := handler.NewAuthHandler(userRepo, jwtMgr, database, auditLogger)
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

	// Settings handler
	settingsHandler := handler.NewSettingsHandler(accountRepo, userRepo, convoRepo, recorder, database, auditLogger, registerAccount, rtr)

	// Provider management (admin only)
	apiAuth.GET("/providers", settingsHandler.ListProviders)
	apiAuth.POST("/providers", settingsHandler.AddProvider)
	apiAuth.DELETE("/providers/:id", settingsHandler.DeleteProvider)
	apiAuth.GET("/provider-templates", settingsHandler.ListTemplates)

	// User management (admin only)
	apiAuth.GET("/users", settingsHandler.ListUsers)
	apiAuth.POST("/users", settingsHandler.CreateUser)
	apiAuth.DELETE("/users/:id", settingsHandler.DeleteUser)

	// API key management
	apiAuth.GET("/api-keys", settingsHandler.ListAPIKeys)
	apiAuth.POST("/api-keys", settingsHandler.CreateAPIKey)
	apiAuth.DELETE("/api-keys/:id", settingsHandler.DeleteAPIKey)

	// Conversation management
	apiAuth.GET("/conversations", settingsHandler.ListConversations)
	apiAuth.POST("/conversations", settingsHandler.CreateConversation)
	apiAuth.GET("/conversations/:id", settingsHandler.GetConversation)
	apiAuth.PUT("/conversations/:id", settingsHandler.UpdateConversation)
	apiAuth.DELETE("/conversations/:id", settingsHandler.DeleteConversation)
	apiAuth.POST("/conversations/:id/messages", settingsHandler.AddMessage)
	apiAuth.DELETE("/conversations/:id/messages/:msgId", settingsHandler.DeleteMessageAndAfter)
	apiAuth.GET("/conversations/:id/export", settingsHandler.ExportConversation)
	apiAuth.POST("/conversations/:id/auto-title", settingsHandler.AutoTitle)

	// System prompts
	apiAuth.GET("/system-prompts", settingsHandler.ListSystemPrompts)
	apiAuth.POST("/system-prompts", settingsHandler.CreateSystemPrompt)
	apiAuth.PUT("/system-prompts/:id", settingsHandler.UpdateSystemPrompt)
	apiAuth.DELETE("/system-prompts/:id", settingsHandler.DeleteSystemPrompt)

	// Usage
	apiAuth.GET("/usage", settingsHandler.GetUsage)
	apiAuth.GET("/usage/all", settingsHandler.GetAllUsage)

	// OAuth routes
	oauthHandler := handler.NewOAuthHandler(oauthMgr, rtr, registerAccount, auditLogger)
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

	// API routes
	apiHandler := handler.NewAPIHandler(rtr, recorder)
	v1 := engine.Group("/v1")
	v1.Use(handler.APIKeyAuthMiddleware(database.DB, jwtMgr))
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
	apiAuth.GET("/audit-log", settingsHandler.GetAuditLog)

	// Admin dashboard
	apiAuth.GET("/dashboard", settingsHandler.Dashboard)

	// Database backup (admin only)
	apiAuth.GET("/backup", settingsHandler.BackupDB)

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
