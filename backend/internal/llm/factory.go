// Package llm соединяет DB-модель models.LLMProvider с протокольными клиентами из pkg/llm.
//
// Sprint 15.9: фабрика NewLLMClient(provider, secrets) — выбирает реализацию по kind, дешифрует креды.
package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/anthropic"
	"github.com/devteam/backend/pkg/llm/providers/deepseek"
	"github.com/devteam/backend/pkg/llm/providers/freeclaudeproxy"
	"github.com/devteam/backend/pkg/llm/providers/gemini"
	"github.com/devteam/backend/pkg/llm/providers/moonshot"
	"github.com/devteam/backend/pkg/llm/providers/oaicompat"
	"github.com/devteam/backend/pkg/llm/providers/ollama"
	"github.com/devteam/backend/pkg/llm/providers/openai"
	"github.com/devteam/backend/pkg/llm/providers/openrouter"
	"github.com/devteam/backend/pkg/llm/providers/qwen"
	"github.com/devteam/backend/pkg/llm/providers/zhipu"
)

// SecretsResolver — источник секретов для провайдера: дешифрует blob (AES-256-GCM)
// в API-ключ или OAuth-токен.
//
// Реализуется внутри LLMProviderService (15.10) поверх backend/internal/service.Encryptor.
// AAD рекомендуется делать равным provider.ID.String(), чтобы blob нельзя было
// «перенести» с одной записи на другую.
type SecretsResolver interface {
	ResolveCredentials(ctx context.Context, provider *models.LLMProvider) (string, error)
}

// SecretsResolverFunc — функциональный адаптер для SecretsResolver.
type SecretsResolverFunc func(ctx context.Context, provider *models.LLMProvider) (string, error)

func (f SecretsResolverFunc) ResolveCredentials(ctx context.Context, provider *models.LLMProvider) (string, error) {
	return f(ctx, provider)
}

// ErrProviderDisabled — попытка получить клиента для выключенного провайдера.
var ErrProviderDisabled = errors.New("llm provider is disabled")

// ErrUnsupportedKind — kind провайдера неизвестен фабрике.
var ErrUnsupportedKind = errors.New("unsupported llm provider kind")

// NewLLMClient — фабрика клиента LLM-провайдера (Sprint 15.9).
//
// Алгоритм:
//  1. Проверяет provider.Enabled.
//  2. Через SecretsResolver получает plaintext-кредентиал (если провайдер требует).
//  3. По provider.Kind выбирает конкретную реализацию из pkg/llm/providers и
//     оборачивает её в llm.ProviderAdapter, чтобы получить llm.Client.
func NewLLMClient(ctx context.Context, provider *models.LLMProvider, secrets SecretsResolver) (llm.Client, error) {
	if provider == nil {
		return nil, fmt.Errorf("llm factory: provider is nil")
	}
	if !provider.Enabled {
		return nil, ErrProviderDisabled
	}

	credential, err := resolveCredential(ctx, provider, secrets)
	if err != nil {
		return nil, err
	}

	cfg := llm.Config{APIKey: credential, BaseURL: provider.BaseURL}

	switch provider.Kind {
	case models.LLMProviderKindAnthropic, models.LLMProviderKindAnthropicOAuth:
		c, err := anthropic.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{Provider: c, BaseURL: provider.BaseURL}, nil

	case models.LLMProviderKindOpenAI:
		c, err := openai.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{Provider: c, BaseURL: provider.BaseURL}, nil

	case models.LLMProviderKindGemini:
		c, err := gemini.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{Provider: c, BaseURL: provider.BaseURL}, nil

	case models.LLMProviderKindDeepSeek:
		c, err := deepseek.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{Provider: c, BaseURL: provider.BaseURL}, nil

	case models.LLMProviderKindQwen:
		c, err := qwen.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{Provider: c, BaseURL: provider.BaseURL}, nil

	case models.LLMProviderKindOpenRouter:
		c, err := openrouter.NewFromLLMConfig(cfg)
		if err != nil {
			return nil, err
		}
		return wrapOAI(c), nil

	case models.LLMProviderKindMoonshot:
		c, err := moonshot.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return wrapOAI(c), nil

	case models.LLMProviderKindOllama:
		c, err := ollama.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return wrapOAI(c), nil

	case models.LLMProviderKindZhipu:
		c, err := zhipu.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return wrapOAI(c), nil

	case models.LLMProviderKindFreeClaudeProxy:
		c, err := freeclaudeproxy.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       c.BaseURL(),
			HealthCheckFn: c.HealthCheck,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedKind, provider.Kind)
	}
}

// wrapOAI оборачивает oaicompat.Client в ProviderAdapter с HealthCheck.
func wrapOAI(c *oaicompat.Client) *llm.ProviderAdapter {
	return &llm.ProviderAdapter{
		Provider:      c,
		BaseURL:       c.BaseURL(),
		HealthCheckFn: c.HealthCheck,
	}
}

// resolveCredential возвращает plaintext-кредентиал, если у провайдера он требуется.
// Для auth_type == none (например, локальная Ollama) шаг пропускается.
// В остальных случаях вызывается resolver: у persistent-провайдеров он дешифрует
// CredentialsEncrypted, у in-memory (TestConnection) — отдаёт plaintext напрямую.
func resolveCredential(ctx context.Context, provider *models.LLMProvider, secrets SecretsResolver) (string, error) {
	if provider.AuthType == models.LLMProviderAuthNone {
		return "", nil
	}
	if secrets == nil {
		if len(provider.CredentialsEncrypted) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("llm factory: secrets resolver required for provider %s", provider.Name)
	}
	return secrets.ResolveCredentials(ctx, provider)
}
