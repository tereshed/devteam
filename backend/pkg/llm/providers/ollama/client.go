// Package ollama — локальный Ollama-сервер, OpenAI-совместимый эндпоинт /v1/chat/completions.
package ollama

import (
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/oaicompat"
)

const (
	DefaultBaseURL = "http://localhost:11434/v1"
	DefaultModel   = "llama3"
)

// NewClient создаёт клиент для локального/удалённого Ollama. Ollama не требует API-ключа.
func NewClient(c llm.Config) (*oaicompat.Client, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	apiKey := c.APIKey
	if apiKey == "" {
		apiKey = "ollama" // Ollama игнорирует ключ, но требует наличия заголовка Authorization.
	}
	return oaicompat.NewClient(oaicompat.Config{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		DefaultModel: DefaultModel,
		HTTPClient:   c.HTTPClient, // Sprint 15.N8.
	})
}
