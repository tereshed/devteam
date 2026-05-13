// Package llm соединяет DB-модель models.LLMProvider с протокольными клиентами из pkg/llm.
//
// Sprint 15.9: фабрика NewLLMClient(provider, secrets) — выбирает реализацию по kind, дешифрует креды.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/anthropic"
	"github.com/devteam/backend/pkg/llm/providers/deepseek"
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

// HTTPClientFactory — Sprint 15.N8: внешний поставщик http.Client с SSRF-guard.
// Реализуется в internal/service (newSSRFSafeHTTPClient) и пробрасывается через NewLLMClient.
// Если nil — фабрика провайдеров получит llm.Config.HTTPClient=nil, и каждый клиент
// создаст &http.Client{} (legacy-поведение, без SSRF-защиты).
type HTTPClientFactory interface {
	// HTTPClient возвращает http.Client с SSRF-guard, конфигурированный под provider.Kind
	// (kind=ollama позволяет loopback; иначе — отказ).
	HTTPClient(provider *models.LLMProvider) *http.Client
}

// NewLLMClient — фабрика клиента LLM-провайдера (Sprint 15.9).
//
// Алгоритм:
//  1. Проверяет provider.Enabled.
//  2. Через SecretsResolver получает plaintext-кредентиал (если провайдер требует).
//  3. Через HTTPClientFactory получает SSRF-safe http.Client (Sprint 15.N8); пробрасывает в Config.
//  4. По provider.Kind выбирает конкретную реализацию из pkg/llm/providers и
//     оборачивает её в llm.ProviderAdapter, чтобы получить llm.Client.
func NewLLMClient(ctx context.Context, provider *models.LLMProvider, secrets SecretsResolver, httpClients HTTPClientFactory) (llm.Client, error) {
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
	if httpClients != nil {
		cfg.HTTPClient = httpClients.HTTPClient(provider)
	}

	// Sprint 15.minor: для провайдеров без собственного HealthCheck используем generic HEAD/GET.
	// Без него UI-кнопка «Проверить» возвращает nil без реального запроса.
	switch provider.Kind {
	case models.LLMProviderKindAnthropic, models.LLMProviderKindAnthropicOAuth:
		c, err := anthropic.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       provider.BaseURL,
			HealthCheckFn: genericHealthCheckFn(cfg.HTTPClient, provider.BaseURL, "/v1/models", credential, anthropicHeaders),
		}, nil

	case models.LLMProviderKindOpenAI:
		c, err := openai.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       provider.BaseURL,
			HealthCheckFn: genericHealthCheckFn(cfg.HTTPClient, provider.BaseURL, "/models", credential, bearerHeader),
		}, nil

	case models.LLMProviderKindGemini:
		c, err := gemini.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       provider.BaseURL,
			HealthCheckFn: genericHealthCheckFn(cfg.HTTPClient, provider.BaseURL, "/v1beta/models?key="+credential, "", noAuth),
		}, nil

	case models.LLMProviderKindDeepSeek:
		c, err := deepseek.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       provider.BaseURL,
			HealthCheckFn: genericHealthCheckFn(cfg.HTTPClient, provider.BaseURL, "/models", credential, bearerHeader),
		}, nil

	case models.LLMProviderKindQwen:
		c, err := qwen.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		return &llm.ProviderAdapter{
			Provider:      c,
			BaseURL:       provider.BaseURL,
			HealthCheckFn: genericHealthCheckFn(cfg.HTTPClient, provider.BaseURL, "/models", credential, bearerHeader),
		}, nil

	case models.LLMProviderKindOpenRouter:
		// Sprint 15.m8: читаем LLMProvider.Settings — поддерживаем http_referer/x_title для атрибуции OpenRouter.
		// Schema (опциональные ключи):
		//   { "http_referer": "https://myapp.com", "x_title": "DevTeam", "default_model": "anthropic/claude-3.5-sonnet" }
		settings := decodeProviderSettings(provider.Settings)
		c, err := openrouter.NewClient(openrouter.Config{
			Config:       cfg,
			HTTPReferer:  settings.HTTPReferer,
			XTitle:       settings.XTitle,
			DefaultModel: settings.DefaultModel,
		})
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

// providerSettings — Sprint 15.m8: распакованное содержимое llm_providers.settings JSONB.
type providerSettings struct {
	HTTPReferer  string `json:"http_referer,omitempty"`
	XTitle       string `json:"x_title,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
}

// decodeProviderSettings безопасно парсит provider.Settings; невалидный JSON → пустая структура.
func decodeProviderSettings(raw []byte) providerSettings {
	var s providerSettings
	if len(raw) == 0 {
		return s
	}
	_ = json.Unmarshal(raw, &s)
	return s
}

// Sprint 15.minor — generic HealthCheck для провайдеров, у которых нет собственного.
// Делает GET {baseURL}{path} (как правило, /models) и проверяет 2xx. credential применяется
// через headerFn (Bearer для OpenAI-compat, x-api-key для Anthropic, query-param для Gemini).
type headerFn func(req *http.Request, credential string)

func bearerHeader(req *http.Request, credential string) {
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
}
func anthropicHeaders(req *http.Request, credential string) {
	if credential != "" {
		req.Header.Set("x-api-key", credential)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
}
func noAuth(*http.Request, string) {}

func genericHealthCheckFn(httpClient *http.Client, baseURL, path, credential string, hdr headerFn) func(ctx context.Context) error {
	hc := httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context) error {
		if baseURL == "" {
			return errors.New("health check: base url is empty")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, http.NoBody)
		if err != nil {
			return fmt.Errorf("health check: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		hdr(req, credential)
		resp, err := hc.Do(req)
		if err != nil {
			return fmt.Errorf("health check: request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("health check: api status %d", resp.StatusCode)
		}
		return nil
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
