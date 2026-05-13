// Package openrouter — OpenRouter (https://openrouter.ai) — OpenAI-совместимый агрегатор моделей.
package openrouter

import (
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/oaicompat"
)

// DefaultBaseURL — публичный endpoint OpenRouter.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// DefaultModel — модель по умолчанию.
const DefaultModel = "openrouter/auto"

// Config — параметры клиента OpenRouter (дополнения поверх llm.Config).
type Config struct {
	llm.Config
	// HTTPReferer — обязательный для OpenRouter заголовок (рекомендован).
	HTTPReferer string
	// XTitle — заголовок X-Title для атрибуции (необязательный).
	XTitle string
	// DefaultModel — модель по умолчанию (если задан запросом — приоритет у запроса).
	DefaultModel string
}

// NewClient создаёт OpenAI-совместимый клиент с дефолтами OpenRouter.
func NewClient(cfg Config) (*oaicompat.Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	model := cfg.DefaultModel
	if model == "" {
		model = DefaultModel
	}
	extra := map[string]string{}
	if cfg.HTTPReferer != "" {
		extra["HTTP-Referer"] = cfg.HTTPReferer
	}
	if cfg.XTitle != "" {
		extra["X-Title"] = cfg.XTitle
	}
	return oaicompat.NewClient(oaicompat.Config{
		APIKey:       cfg.APIKey,
		BaseURL:      baseURL,
		DefaultModel: model,
		ExtraHeaders: extra,
		// Sprint 15.N8: проброс SSRF-safe http.Client из llm.Config.
		HTTPClient: cfg.HTTPClient,
	})
}

// NewFromLLMConfig — упрощённая фабрика, использующая llm.Config напрямую.
// Применяется в pkg/llm/factory, где Config не несёт OpenRouter-специфичных полей.
func NewFromLLMConfig(c llm.Config) (*oaicompat.Client, error) {
	return NewClient(Config{Config: c})
}
