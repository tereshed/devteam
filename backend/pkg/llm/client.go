package llm

import (
	"context"
	"errors"
)

// ErrEmbeddingsNotSupported возвращается провайдерами, которые не поддерживают эмбеддинги.
var ErrEmbeddingsNotSupported = errors.New("embeddings are not supported by this provider")

// EmbedRequest — запрос на генерацию эмбеддингов.
type EmbedRequest struct {
	Model string   `json:"model,omitempty"`
	Input []string `json:"input"`
}

// EmbedResponse — ответ с эмбеддингами.
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Usage      Usage       `json:"usage"`
}

// Client — расширенный клиент LLM-провайдера (Sprint 15.7).
//
// Используется LLMAgentExecutor (6.2) и LLMProviderService (15.10).
// Реализации создаются фабрикой NewLLMClient (15.9) из models.LLMProvider + дешифрованных кредов.
//
// HealthCheck — лёгкая проверка доступности (как правило, GET /models или эквивалент);
// fail-fast в orchestrator_service (15.19) полагается на этот метод.
type Client interface {
	// Chat — генерация ответа (для существующих реализаций — алиас Generate).
	Chat(ctx context.Context, req Request) (*Response, error)
	// Embed — генерация эмбеддингов. Возвращает ErrEmbeddingsNotSupported,
	// если провайдер не поддерживает.
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
	// HealthCheck — проверка доступности провайдера.
	HealthCheck(ctx context.Context) error
	// ResolveBaseURL — итоговый base URL клиента (после применения дефолтов из ProviderType).
	ResolveBaseURL() string
}

// ProviderAdapter оборачивает Provider, добавляя реализации Chat/Embed/HealthCheck/ResolveBaseURL.
// Используется для существующих провайдеров (anthropic/openai/deepseek/gemini/qwen).
type ProviderAdapter struct {
	Provider Provider
	// BaseURL — итоговый base URL клиента (для ResolveBaseURL).
	BaseURL string
	// HealthCheckFn — кастомный healthcheck. Если nil, HealthCheck возвращает nil.
	HealthCheckFn func(ctx context.Context) error
	// EmbedFn — кастомная реализация Embed. Если nil — ErrEmbeddingsNotSupported.
	EmbedFn func(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

// Chat делегирует Generate базовому Provider.
func (a *ProviderAdapter) Chat(ctx context.Context, req Request) (*Response, error) {
	return a.Provider.Generate(ctx, req)
}

// Embed возвращает результат EmbedFn или ErrEmbeddingsNotSupported.
func (a *ProviderAdapter) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if a.EmbedFn == nil {
		return nil, ErrEmbeddingsNotSupported
	}
	return a.EmbedFn(ctx, req)
}

// HealthCheck вызывает HealthCheckFn, либо считает провайдера здоровым.
func (a *ProviderAdapter) HealthCheck(ctx context.Context) error {
	if a.HealthCheckFn == nil {
		return nil
	}
	return a.HealthCheckFn(ctx)
}

// ResolveBaseURL возвращает зафиксированный base URL.
func (a *ProviderAdapter) ResolveBaseURL() string {
	return a.BaseURL
}

// Compile-time check: ProviderAdapter реализует Client.
var _ Client = (*ProviderAdapter)(nil)
