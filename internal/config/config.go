package config

import (
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port        int      `mapstructure:"port"`
	Host        string   `mapstructure:"host"`
	CORSOrigins []string `mapstructure:"cors_origins"` // allowed origins, empty = default policy
}

type SecurityConfig struct {
	Secret string `mapstructure:"secret"`
}

type RoutingConfig struct {
	Strategy         string `mapstructure:"strategy"`
	MaxRetries       int    `mapstructure:"max_retries"`
	FailoverAttempts int    `mapstructure:"failover_attempts"`
}

type StorageConfig struct {
	RetentionDays int `mapstructure:"retention_days"`
}

type AccountConfig struct {
	Label         string   `mapstructure:"label"`
	APIKey        string   `mapstructure:"api_key"`
	Models        []string `mapstructure:"models"`
	MaxConcurrent int      `mapstructure:"max_concurrent"`
}

type ProviderConfig struct {
	Name     string          `mapstructure:"name"`
	Type     string          `mapstructure:"type"`
	BaseURL  string          `mapstructure:"base_url"`
	Accounts []AccountConfig `mapstructure:"accounts"`
}

type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

type OAuthConfigs struct {
	BaseURL string               `mapstructure:"base_url"`
	OpenAI  *OAuthProviderConfig `mapstructure:"openai"`
	Qwen    *OAuthProviderConfig `mapstructure:"qwen"`
	Claude  *OAuthProviderConfig `mapstructure:"claude"`
}

type WebhookConfig struct {
	URL    string   `mapstructure:"url"`
	Events []string `mapstructure:"events"` // "provider_error", "quota_warning", "user_login", "account_bound"
}

type CacheConfig struct {
	Enabled bool `mapstructure:"enabled"`
	TTL     int  `mapstructure:"ttl_seconds"` // default 300 (5 min)
}

type Config struct {
	Server        ServerConfig     `mapstructure:"server"`
	Security      SecurityConfig   `mapstructure:"security"`
	Routing       RoutingConfig    `mapstructure:"routing"`
	Storage       StorageConfig    `mapstructure:"storage"`
	Providers     []ProviderConfig `mapstructure:"providers"`
	OAuth         OAuthConfigs     `mapstructure:"oauth"`
	LogLevel      string           `mapstructure:"log_level"`
	DataDir       string           `mapstructure:"data_dir"`
	Webhooks      []WebhookConfig  `mapstructure:"webhooks"`
	ResponseCache CacheConfig      `mapstructure:"response_cache"`
}

func Load(cfgPath string) (*Config, error) {
	v := viper.New()
	v.SetDefault("server.port", 9000)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("routing.strategy", "round_robin")
	v.SetDefault("routing.max_retries", 3)
	v.SetDefault("routing.failover_attempts", 2)
	v.SetDefault("storage.retention_days", 0)
	v.SetDefault("log_level", "info")

	v.SetEnvPrefix("UNIAPI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("server.port", "UNIAPI_PORT")
	_ = v.BindEnv("security.secret", "UNIAPI_SECRET")
	_ = v.BindEnv("data_dir", "UNIAPI_DATA_DIR")
	_ = v.BindEnv("log_level", "UNIAPI_LOG_LEVEL")

	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
