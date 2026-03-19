package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/user/uniapi/internal/auth"
    "github.com/user/uniapi/internal/cache"
    "github.com/user/uniapi/internal/config"
    "github.com/user/uniapi/internal/crypto"
    "github.com/user/uniapi/internal/db"
    "github.com/user/uniapi/internal/handler"
    "github.com/user/uniapi/internal/provider"
    pAnthropic "github.com/user/uniapi/internal/provider/anthropic"
    pGemini "github.com/user/uniapi/internal/provider/gemini"
    pOpenai "github.com/user/uniapi/internal/provider/openai"
    "github.com/user/uniapi/internal/router"
    "github.com/user/uniapi/internal/web"
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
        if _, err := os.Stat(defaultCfg); err == nil { *cfgPath = defaultCfg }
    }
    cfg, err := config.Load(*cfgPath)
    if err != nil && *cfgPath != "" { log.Fatalf("config: %v", err) }
    if cfg == nil {
        cfg = &config.Config{}
        cfg.Server.Port = 9000; cfg.Server.Host = "0.0.0.0"
        cfg.Routing.Strategy = "round_robin"; cfg.Routing.MaxRetries = 3; cfg.Routing.FailoverAttempts = 2
    }

    // CLI overrides
    if *port > 0 { cfg.Server.Port = *port }
    if *dataDir != "" { cfg.DataDir = *dataDir }
    if *secret != "" { cfg.Security.Secret = *secret }

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
        if err != nil { log.Fatalf("secret: %v", err) }
    }

    // Database
    dbPath := filepath.Join(cfg.DataDir, "data.db")
    database, err := db.Open(dbPath)
    if err != nil { log.Fatalf("database: %v", err) }
    defer database.Close()

    // Cache
    memCache := cache.New()
    defer memCache.Stop()

    // Router
    rtr := router.New(memCache, router.Config{
        Strategy: cfg.Routing.Strategy, MaxRetries: cfg.Routing.MaxRetries, FailoverAttempts: cfg.Routing.FailoverAttempts,
    })

    // Register providers
    for _, pc := range cfg.Providers {
        for _, acc := range pc.Accounts {
            var p provider.Provider
            maxConc := acc.MaxConcurrent
            if maxConc == 0 { maxConc = 5 }
            provCfg := provider.ProviderConfig{Name: pc.Name, Type: pc.Type, BaseURL: pc.BaseURL}
            switch pc.Type {
            case "anthropic":
                p = pAnthropic.NewAnthropic(provCfg, acc.Models, acc.APIKey)
            case "openai":
                p = pOpenai.NewOpenAI(provCfg, acc.Models, acc.APIKey)
            case "gemini":
                p = pGemini.NewGemini(provCfg, acc.Models, acc.APIKey)
            case "openai_compatible":
                p = pOpenai.NewOpenAI(provCfg, acc.Models, acc.APIKey)
            default:
                log.Printf("Unknown provider type: %s", pc.Type); continue
            }
            accountID := fmt.Sprintf("%s-%s", pc.Name, acc.Label)
            rtr.AddAccount(accountID, p, maxConc)
            log.Printf("Registered: %s (%s) with %d models", pc.Name, acc.Label, len(acc.Models))
        }
    }

    // Auth
    jwtKey := crypto.DeriveKey(cfg.Security.Secret)
    jwtMgr := auth.NewJWTManager(jwtKey, 7*24*time.Hour)
    _ = jwtMgr // will be used for auth routes

    // Gin
    gin.SetMode(gin.ReleaseMode)
    engine := gin.New()
    engine.Use(gin.Recovery())
    engine.Use(handler.CORSMiddleware())

    // API routes
    apiHandler := handler.NewAPIHandler(rtr)
    v1 := engine.Group("/v1")
    v1.Use(handler.APIKeyAuthMiddleware(database.DB, jwtMgr))
    v1.POST("/chat/completions", apiHandler.ChatCompletions)
    v1.GET("/models", apiHandler.ListModels)

    // Health
    engine.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

    web.RegisterFrontend(engine)

    addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
    log.Printf("UniAPI starting on %s", addr)
    if err := engine.Run(addr); err != nil { log.Fatalf("server: %v", err) }
}
