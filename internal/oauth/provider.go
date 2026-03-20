package oauth

// BindingProvider describes an AI provider that supports account binding.
type BindingProvider struct {
	Name                 string
	DisplayName          string
	ProviderType         string   // maps to adapter type
	AuthURL              string
	TokenURL             string
	Scopes               []string
	SupportsOAuth        bool
	SupportsSessionToken bool
	DefaultModels        []string
}

var defaultProviders = map[string]*BindingProvider{
	"openai": {
		Name: "openai", DisplayName: "OpenAI", ProviderType: "openai",
		SupportsOAuth: false, SupportsSessionToken: true,
		DefaultModels: []string{"gpt-4o", "gpt-4o-mini"},
	},
	"anthropic": {
		Name: "anthropic", DisplayName: "Claude", ProviderType: "anthropic",
		SupportsOAuth: false, SupportsSessionToken: true,
		DefaultModels: []string{"claude-sonnet-4-20250514", "claude-haiku-4-20250414"},
	},
	"aliyun": {
		Name: "aliyun", DisplayName: "Qwen", ProviderType: "openai_compatible",
		AuthURL:  "https://signin.aliyun.com/oauth2/v1/auth",
		TokenURL: "https://oauth.aliyun.com/v1/token",
		Scopes:   []string{"openid"},
		SupportsOAuth: true, SupportsSessionToken: true,
		DefaultModels: []string{"qwen-plus", "qwen-turbo"},
	},
}
