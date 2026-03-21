package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected default port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Routing.Strategy != "round_robin" {
		t.Errorf("expected default strategy round_robin, got %s", cfg.Routing.Strategy)
	}
	if cfg.Routing.MaxRetries != 3 {
		t.Errorf("expected default max_retries 3, got %d", cfg.Routing.MaxRetries)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  port: 8080
  host: "127.0.0.1"
routing:
  strategy: least_used
  max_retries: 5
  failover_attempts: 3
`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Routing.Strategy != "least_used" {
		t.Errorf("expected strategy least_used, got %s", cfg.Routing.Strategy)
	}
}

func TestOAuthConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
oauth:
  base_url: "https://example.com"
  qwen:
    client_id: "test-id"
    client_secret: "test-secret"
`), 0644)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuth.BaseURL != "https://example.com" {
		t.Errorf("wrong base_url")
	}
	if cfg.OAuth.Qwen == nil {
		t.Fatal("missing qwen")
	}
	if cfg.OAuth.Qwen.ClientID != "test-id" {
		t.Error("wrong client_id")
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 9000
	cfg.Routing.MaxRetries = 3
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should not error: %v", err)
	}

	cfg.Server.Port = -1
	if err := cfg.Validate(); err == nil {
		t.Error("negative port should fail")
	}

	cfg.Server.Port = 9000
	cfg.Storage.RetentionDays = -5
	if err := cfg.Validate(); err == nil {
		t.Error("negative retention should fail")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("UNIAPI_PORT", "7777")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", cfg.Server.Port)
	}
}
