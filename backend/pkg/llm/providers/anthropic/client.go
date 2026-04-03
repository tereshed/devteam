package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/devteam/backend/pkg/llm"
)

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

// Anthropic API Request Structures
type messagesRequest struct {
	Model       string      `json:"model"`
	Messages    []message   `json:"messages"`
	System      string      `json:"system,omitempty"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature,omitempty"`
	Tools       []tool      `json:"tools,omitempty"`
	ToolChoice  *toolChoice `json:"tool_choice,omitempty"`
}

type message struct {
	Role    string    `json:"role"`
	Content []content `json:"content"`
}

type content struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`          // For tool_use
	Name      string          `json:"name,omitempty"`        // For tool_use
	Input     json.RawMessage `json:"input,omitempty"`       // For tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // For tool_result
	Content   string          `json:"content,omitempty"`     // For tool_result
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// Anthropic API Response Structures
type messagesResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         string    `json:"role"`
	Content      []content `json:"content"`
	StopReason   string    `json:"stop_reason"`
	StopSequence string    `json:"stop_sequence"`
	Usage        usage     `json:"usage"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	anthropicReq := c.mapRequest(req)

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic api error (status %d): %s", resp.StatusCode, string(body))
	}

	var anthropicResp messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.mapResponse(anthropicResp), nil
}

func (c *Client) mapRequest(req llm.Request) messagesRequest {
	messages := make([]message, 0, len(req.Messages))

	for _, msg := range req.Messages {
		// Skip system messages as they go to top-level field
		if msg.Role == llm.RoleSystem {
			continue
		}

		var contents []content
		if msg.Role == llm.RoleTool {
			// Tool result
			contents = append(contents, content{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			})
		} else if len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
			if msg.Content != "" {
				contents = append(contents, content{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				contents = append(contents, content{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				})
			}
		} else {
			// Regular text message
			contents = append(contents, content{
				Type: "text",
				Text: msg.Content,
			})
		}

		messages = append(messages, message{
			Role:    string(msg.Role),
			Content: contents,
		})
	}

	var tools []tool
	if len(req.Tools) > 0 {
		tools = make([]tool, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = tool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			}
		}
	}

	var toolChoice *toolChoice
	// If structured output is requested, we can force a tool call
	// Note: Anthropic doesn't have "json_schema" mode exactly like OpenAI yet,
	// but usually this is done via tool use.
	// For now, we'll just pass tools. If strict structured output is needed,
	// we would add a specific tool for it and force it.
	// Ignoring StructuredOutputSchema for now or assuming it's handled via Tools.

	model := "claude-3-5-sonnet-20240620" // Default

	return messagesRequest{
		Model:       model,
		Messages:    messages,
		System:      req.SystemPrompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		ToolChoice:  toolChoice,
	}
}

func (c *Client) mapResponse(resp messagesResponse) *llm.Response {
	var textContent string
	var toolCalls []llm.ToolCall

	for _, content := range resp.Content {
		if content.Type == "text" {
			textContent += content.Text
		} else if content.Type == "tool_use" {
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   content.ID,
				Type: "function",
				Function: llm.Function{
					Name:      content.Name,
					Arguments: string(content.Input),
				},
			})
		}
	}

	return &llm.Response{
		Content:   textContent,
		ToolCalls: toolCalls,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
