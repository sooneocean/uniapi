package handler

import (
	"github.com/sooneocean/uniapi/internal/provider"
	pAnthropic "github.com/sooneocean/uniapi/internal/provider/anthropic"
	pGemini "github.com/sooneocean/uniapi/internal/provider/gemini"
	pOpenai "github.com/sooneocean/uniapi/internal/provider/openai"
	pSub2api "github.com/sooneocean/uniapi/internal/provider/sub2api"
)

// CreateProvider builds a provider instance from type name, config, models, and credFunc.
func CreateProvider(provType string, cfg provider.ProviderConfig, models []string, credFunc func() (string, string)) provider.Provider {
	// NEW: Check if session_token → use sub2api adapter
	_, authType := credFunc()
	if authType == "session_token" {
		switch provType {
		case "openai":
			return pSub2api.NewChatGPT(models, credFunc)
		case "anthropic":
			return pSub2api.NewClaudeWeb(models, credFunc)
		case "gemini":
			return pSub2api.NewGeminiWeb(models, credFunc)
		}
	}

	switch provType {
	case "anthropic":
		return pAnthropic.NewAnthropic(cfg, models, credFunc)
	case "gemini":
		return pGemini.NewGemini(cfg, models, credFunc)
	case "openai", "openai_compatible":
		return pOpenai.NewOpenAI(cfg, models, credFunc)
	default:
		return pOpenai.NewOpenAI(cfg, models, credFunc) // fallback
	}
}
