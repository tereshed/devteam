package qwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/devteam/backend/pkg/llm"
)

// Client is essentially an OpenAI client but configured for Qwen
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(config llm.Config) (*Client, error) {
	return &Client{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		client:  &http.Client{},
	}, nil
}

// Reusing OpenAI structures as Qwen is compatible
type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Tools          []tool          `json:"tools,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
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

func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	qwenReq := c.mapRequest(req)

	reqBody, err := json.Marshal(qwenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qwen api error (status %d): %s", resp.StatusCode, string(body))
	}

	var qwenResp chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&qwenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return c.mapResponse(qwenResp), nil
}

func (c *Client) mapRequest(req llm.Request) chatCompletionRequest {
	messages := make([]message, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		messages = append(messages, message{
			Role:    "system",
			Content: req.SystemPrompt,
		})
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

	var respFormat *responseFormat
	if req.StructuredOutputSchema != nil {
		respFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaDefinition{
				Name:   "structured_output",
				Schema: req.StructuredOutputSchema,
				Strict: true,
			},
		}
	}

	model := "qwen-turbo" // Default for Qwen

	return chatCompletionRequest{
		Model:          model,
		Messages:       messages,
		Tools:          tools,
		ResponseFormat: respFormat,
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
