package provider

// ProviderTemplate defines a preconfigured provider with known endpoint and models
type ProviderTemplate struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Type          string   `json:"type"`          // "openai", "anthropic", "gemini", "openai_compatible"
	BaseURL       string   `json:"base_url"`
	DefaultModels []string `json:"default_models"`
	Description   string   `json:"description"`
	APIKeyURL     string   `json:"api_key_url"` // where to get an API key
}

var Templates = []ProviderTemplate{
	{
		Name: "openai", DisplayName: "OpenAI", Type: "openai",
		BaseURL:       "https://api.openai.com",
		DefaultModels: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o3-mini"},
		Description:   "GPT-4o, o3, and more",
		APIKeyURL:     "https://platform.openai.com/api-keys",
	},
	{
		Name: "anthropic", DisplayName: "Anthropic (Claude)", Type: "anthropic",
		BaseURL:       "https://api.anthropic.com",
		DefaultModels: []string{"claude-sonnet-4-20250514", "claude-haiku-4-20250414", "claude-opus-4-20250514"},
		Description:   "Claude Sonnet, Haiku, Opus",
		APIKeyURL:     "https://console.anthropic.com/settings/keys",
	},
	{
		Name: "gemini", DisplayName: "Google Gemini", Type: "gemini",
		BaseURL:       "https://generativelanguage.googleapis.com",
		DefaultModels: []string{"gemini-2.5-pro", "gemini-2.5-flash"},
		Description:   "Gemini 2.5 Pro and Flash",
		APIKeyURL:     "https://aistudio.google.com/apikey",
	},
	{
		Name: "deepseek", DisplayName: "DeepSeek", Type: "openai_compatible",
		BaseURL:       "https://api.deepseek.com",
		DefaultModels: []string{"deepseek-chat", "deepseek-reasoner"},
		Description:   "DeepSeek V3 and R1",
		APIKeyURL:     "https://platform.deepseek.com/api_keys",
	},
	{
		Name: "mistral", DisplayName: "Mistral AI", Type: "openai_compatible",
		BaseURL:       "https://api.mistral.ai",
		DefaultModels: []string{"mistral-large-latest", "mistral-small-latest", "codestral-latest"},
		Description:   "Mistral Large, Small, Codestral",
		APIKeyURL:     "https://console.mistral.ai/api-keys",
	},
	{
		Name: "groq", DisplayName: "Groq", Type: "openai_compatible",
		BaseURL:       "https://api.groq.com/openai",
		DefaultModels: []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768"},
		Description:   "Ultra-fast inference (Llama, Mixtral)",
		APIKeyURL:     "https://console.groq.com/keys",
	},
	{
		Name: "ollama", DisplayName: "Ollama (Local)", Type: "openai_compatible",
		BaseURL:       "http://localhost:11434/v1",
		DefaultModels: []string{"llama3", "codellama", "mistral"},
		Description:   "Local models via Ollama",
		APIKeyURL:     "",
	},
	{
		Name: "together", DisplayName: "Together AI", Type: "openai_compatible",
		BaseURL:       "https://api.together.xyz",
		DefaultModels: []string{"meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", "mistralai/Mixtral-8x22B-Instruct-v0.1"},
		Description:   "Open-source models at scale",
		APIKeyURL:     "https://api.together.xyz/settings/api-keys",
	},
}

func GetTemplate(name string) *ProviderTemplate {
	for _, t := range Templates {
		if t.Name == name {
			return &t
		}
	}
	return nil
}
