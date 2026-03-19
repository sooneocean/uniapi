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
