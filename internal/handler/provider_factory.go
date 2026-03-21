package handler

import (
	"github.com/sooneocean/uniapi/internal/provider"
	pAnthropic "github.com/sooneocean/uniapi/internal/provider/anthropic"
	pGemini "github.com/sooneocean/uniapi/internal/provider/gemini"
	pOpenai "github.com/sooneocean/uniapi/internal/provider/openai"
)

// CreateProvider builds a provider instance from type name, config, models, and credFunc.
func CreateProvider(provType string, cfg provider.ProviderConfig, models []string, credFunc func() (string, string)) provider.Provider {
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
