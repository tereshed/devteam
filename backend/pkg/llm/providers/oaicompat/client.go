// Package oaicompat реализует общий OpenAI-совместимый клиент (chat/completions, /models).
// Используется новыми провайдерами Sprint 15.8 (OpenRouter, Moonshot, Ollama, Zhipu) — они отличаются
// только base URL и дефолтной моделью, REST-схема идентична OpenAI.
package oaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/devteam/backend/pkg/llm"
)

// Config — параметры OpenAI-совместимого клиента.
type Config struct {
	APIKey       string
	BaseURL      string // базовый URL (без хвостового слеша). Например: "https://api.deepseek.com/v1".
	DefaultModel string // дефолтная модель, если запрос не задаёт Model.
	// AuthHeader — имя заголовка ("Authorization") и схема ("Bearer"). Если пустой — "Authorization: Bearer <key>".
	AuthHeader string
	AuthScheme string
	// ExtraHeaders — статические заголовки, добавляемые ко всем запросам (например, HTTP-Referer для OpenRouter).
	ExtraHeaders map[string]string
	// HTTPClient — можно подменить (для тестов).
	HTTPClient *http.Client
}

// Client — OpenAI-совместимый клиент.
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient создаёт клиент. baseURL обязателен, остальные поля имеют дефолты.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("oaicompat: base URL is required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "Bearer"
	}
	return &Client{cfg: cfg, http: cfg.HTTPClient}, nil
}

// BaseURL возвращает финальный base URL.
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// Generate реализует llm.Provider.
func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body, err := json.Marshal(c.mapRequest(req))
	if err != nil {
		return nil, fmt.Errorf("oaicompat: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("oaicompat: build request: %w", err)
	}
	c.applyHeaders(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("oaicompat: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oaicompat: api error (status %d): %s", resp.StatusCode, string(payload))
	}

	var parsed chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("oaicompat: decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("oaicompat: empty choices in response")
	}
	return c.mapResponse(parsed), nil
}

// HealthCheck выполняет GET /models. Если эндпоинт вернул 2xx — считаем здоровым.
func (c *Client) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("oaicompat: build health request: %w", err)
	}
	c.applyHeaders(httpReq)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("oaicompat: do health request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("oaicompat: health api error (status %d): %s", resp.StatusCode, string(payload))
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set(c.cfg.AuthHeader, c.cfg.AuthScheme+" "+c.cfg.APIKey)
	}
	for k, v := range c.cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}
}

// === wire types (минимально необходимый OpenAI-совместимый набор) ===

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Tools          []tool          `json:"tools,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      *int            `json:"max_tokens,omitempty"`
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type tool struct {
	Type     string   `json:"type"`
	Function function `json:"function"`
}

type function struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type responseFormat struct {
	Type       string                `json:"type"`
	JSONSchema *jsonSchemaDefinition `json:"json_schema,omitempty"`
}

type jsonSchemaDefinition struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *Client) mapRequest(req llm.Request) chatCompletionRequest {
	messages := make([]message, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, message{Role: "system", Content: req.SystemPrompt})
	}
	for _, msg := range req.Messages {
		m := message{
			Role:       string(msg.Role),
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]toolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				m.ToolCalls[i] = toolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: toolFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		messages = append(messages, m)
	}

	var tools []tool
	if len(req.Tools) > 0 {
		tools = make([]tool, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = tool{
				Type: "function",
				Function: function{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	var rf *responseFormat
	if req.StructuredOutputSchema != nil {
		rf = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaDefinition{
				Name:   "structured_output",
				Schema: req.StructuredOutputSchema,
				Strict: true,
			},
		}
	}

	model := req.Model
	if model == "" {
		model = c.cfg.DefaultModel
	}

	return chatCompletionRequest{
		Model:          model,
		Messages:       messages,
		Tools:          tools,
		ResponseFormat: rf,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
	}
}

func (c *Client) mapResponse(resp chatCompletionResponse) *llm.Response {
	choice := resp.Choices[0]

	var toolCalls []llm.ToolCall
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls = make([]llm.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			toolCalls[i] = llm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: llm.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return &llm.Response{
		Content:   choice.Message.Content,
		ToolCalls: toolCalls,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
