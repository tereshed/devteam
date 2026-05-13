// Package freeclaudeproxy — клиент к sidecar-сервису free-claude-proxy (Sprint 15.16),
// который выставляет Anthropic-совместимый API поверх OpenRouter/DeepSeek/Moonshot/Ollama/Zhipu.
//
// Sprint 15.M11: реализация — тонкая обёртка над anthropic.Client (DRY с anthropic-пакетом).
// Прокси отличается от vanilla Anthropic API двумя вещами:
//   - Authorization: Bearer <token> вместо x-api-key;
//   - путь /v1/messages вместо /messages.
// Всё остальное (wire-формат, mapRequest/mapResponse) идентично.
package freeclaudeproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/anthropic"
)

const (
	// DefaultBaseURL — публикуемый прокси-сервисом адрес (см. docker-compose, Sprint 15.16).
	DefaultBaseURL = "http://free-claude-proxy:8787"
	// DefaultHealthPath — Sprint 15.M10: путь health-check можно переопределить через WithHealthPath.
	DefaultHealthPath = "/healthz"
)

// Client — обёртка над anthropic.Client с Bearer-аутентификацией и /v1/messages эндпоинтом.
type Client struct {
	*anthropic.Client
	baseURL    string
	healthPath string
	http       *http.Client
}

// Option — функциональная опция (Sprint 15.M10/M11).
type Option func(*Client)

// WithHealthPath переопределяет путь health-check (по умолчанию /healthz).
func WithHealthPath(p string) Option {
	return func(c *Client) {
		if p != "" {
			c.healthPath = p
		}
	}
}

// WithHTTPClient — общий http.Client для anthropic-уровня и health-check.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// NewClient создаёт клиент. APIKey в llm.Config интерпретируется как service token (Bearer).
func NewClient(cfg llm.Config, opts ...Option) (*Client, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// Sprint 15.N8: используем cfg.HTTPClient (SSRF-safe), если задан.
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	c := &Client{
		baseURL:    baseURL,
		healthPath: DefaultHealthPath,
		http:       hc,
	}
	for _, o := range opts {
		o(c)
	}
	innerCfg := llm.Config{APIKey: cfg.APIKey, BaseURL: baseURL}
	inner, err := anthropic.NewClient(innerCfg,
		anthropic.WithMessagePath("/v1/messages"),
		anthropic.WithAuthHeader(func(req *http.Request, apiKey string) {
			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
		}),
		anthropic.WithHTTPClient(c.http),
	)
	if err != nil {
		return nil, err
	}
	c.Client = inner
	return c, nil
}

// BaseURL — финальный base URL прокси (для ProviderAdapter.ResolveBaseURL).
func (c *Client) BaseURL() string { return c.baseURL }

// HealthCheck — GET <baseURL><healthPath>. 2xx → ok.
// Sprint 15.M10: путь конфигурируется (WithHealthPath или FREE_CLAUDE_PROXY_HEALTH_PATH).
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.healthPath, nil)
	if err != nil {
		return fmt.Errorf("freeclaudeproxy: build health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("freeclaudeproxy: do health request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("freeclaudeproxy: health api error (status %d): %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}
