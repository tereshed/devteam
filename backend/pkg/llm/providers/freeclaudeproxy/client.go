// Package freeclaudeproxy — клиент к sidecar-сервису free-claude-proxy (Sprint 15.16),
// который выставляет Anthropic-совместимый API поверх OpenRouter/DeepSeek/Moonshot/Ollama/Zhipu.
//
// Прокси принимает заголовок Authorization: Bearer <ANTHROPIC_AUTH_TOKEN> вместо x-api-key.
// Поэтому здесь используется собственная реализация на основе REST-схемы Anthropic (см. anthropic.Client),
// но с другим заголовком авторизации.
package freeclaudeproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/devteam/backend/pkg/llm"
)

const (
	// DefaultBaseURL — публикуемый прокси-сервисом адрес (см. docker-compose, Sprint 15.16).
	DefaultBaseURL = "http://free-claude-proxy:8787"
	// DefaultModel — модель, под которой прокси экспонирует выбранного провайдера.
	DefaultModel = "claude-3-5-sonnet-20240620"
)

// Client — Anthropic-совместимый клиент к free-claude-proxy.
type Client struct {
	authToken string
	baseURL   string
	http      *http.Client
}

// NewClient создаёт клиент. APIKey в llm.Config интерпретируется как service token (Bearer).
func NewClient(c llm.Config) (*Client, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{authToken: c.APIKey, baseURL: baseURL, http: &http.Client{}}, nil
}

// BaseURL возвращает финальный base URL прокси.
func (c *Client) BaseURL() string { return c.baseURL }

// === wire types (минимальный Anthropic-совместимый набор) ===

type messagesRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	System      string    `json:"system,omitempty"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
}

type message struct {
	Role    string    `json:"role"`
	Content []content `json:"content"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type messagesResponse struct {
	Content []content `json:"content"`
	Usage   usage     `json:"usage"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Generate реализует llm.Provider.
func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body, err := json.Marshal(c.mapRequest(req))
	if err != nil {
		return nil, fmt.Errorf("freeclaudeproxy: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("freeclaudeproxy: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("freeclaudeproxy: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("freeclaudeproxy: api error (status %d): %s", resp.StatusCode, string(payload))
	}

	var parsed messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("freeclaudeproxy: decode response: %w", err)
	}

	var text string
	for _, p := range parsed.Content {
		if p.Type == "text" {
			text += p.Text
		}
	}
	return &llm.Response{
		Content: text,
		Usage: llm.Usage{
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		},
	}, nil
}

// HealthCheck проверяет доступность прокси через GET /healthz.
// Прокси обязан экспонировать этот эндпоинт (Sprint 15.19 — fail-fast).
func (c *Client) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("freeclaudeproxy: build health request: %w", err)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("freeclaudeproxy: do health request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("freeclaudeproxy: health api error (status %d): %s", resp.StatusCode, string(payload))
	}
	return nil
}

func (c *Client) mapRequest(req llm.Request) messagesRequest {
	messages := make([]message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem {
			continue
		}
		messages = append(messages, message{
			Role:    string(msg.Role),
			Content: []content{{Type: "text", Text: msg.Content}},
		})
	}
	model := req.Model
	if model == "" {
		model = DefaultModel
	}
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	return messagesRequest{
		Model:       model,
		Messages:    messages,
		System:      req.SystemPrompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}
}
