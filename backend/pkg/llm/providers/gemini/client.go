package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/wibe-flutter-gin-template/backend/pkg/llm"
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

// Gemini API Request Structures
type generateContentRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	Tools             []tool            `json:"tools,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type functionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type functionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitempty"`
}

type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // OpenAPI Schema
}

type generationConfig struct {
	Temperature      float64          `json:"temperature,omitempty"`
	MaxOutputTokens  int              `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string           `json:"responseMimeType,omitempty"`
	ResponseSchema   *json.RawMessage `json:"responseSchema,omitempty"`
}

// Gemini API Response Structures
type generateContentResponse struct {
	Candidates    []candidate   `json:"candidates"`
	UsageMetadata usageMetadata `json:"usageMetadata"`
}

type candidate struct {
	Content      content `json:"content"`
	FinishReason string  `json:"finishReason"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	geminiReq := c.mapRequest(req)

	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Default model
	model := "gemini-1.5-flash"
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	return c.mapResponse(geminiResp), nil
}

func (c *Client) mapRequest(req llm.Request) generateContentRequest {
	var contents []content
	var systemInstruction *content

	if req.SystemPrompt != "" {
		systemInstruction = &content{
			Parts: []part{{Text: req.SystemPrompt}},
		}
	}

	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == llm.RoleAssistant {
			role = "model"
		} else if msg.Role == llm.RoleTool {
			role = "function" // Gemini uses 'function' role for tool responses conceptually, but API expects 'functionResponse' part in 'user' or 'function' role context depending on API version.
			// Actually, for function response, role is usually 'function' in v1beta.
			role = "function"
		}

		var parts []part
		if msg.Role == llm.RoleTool {
			parts = append(parts, part{
				FunctionResponse: &functionResponse{
					Name:     msg.Name,
					Response: json.RawMessage(msg.Content), // Content should be JSON
				},
			})
		} else if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				parts = append(parts, part{
					FunctionCall: &functionCall{
						Name: tc.Function.Name,
						Args: json.RawMessage(tc.Function.Arguments),
					},
				})
			}
		} else {
			parts = append(parts, part{Text: msg.Content})
		}

		contents = append(contents, content{
			Role:  role,
			Parts: parts,
		})
	}

	var tools []tool
	if len(req.Tools) > 0 {
		var decls []functionDeclaration
		for _, t := range req.Tools {
			decls = append(decls, functionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		tools = append(tools, tool{FunctionDeclarations: decls})
	}

	var genConfig *generationConfig
	if req.StructuredOutputSchema != nil {
		genConfig = &generationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   &req.StructuredOutputSchema,
			Temperature:      req.Temperature,
			MaxOutputTokens:  req.MaxTokens,
		}
	} else {
		genConfig = &generationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	return generateContentRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		Tools:             tools,
		GenerationConfig:  genConfig,
	}
}

func (c *Client) mapResponse(resp generateContentResponse) *llm.Response {
	cand := resp.Candidates[0]

	var textContent string
	var toolCalls []llm.ToolCall

	for _, p := range cand.Content.Parts {
		if p.Text != "" {
			textContent += p.Text
		}
		if p.FunctionCall != nil {
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   "", // Gemini doesn't provide ID for function calls in the same way
				Type: "function",
				Function: llm.Function{
					Name:      p.FunctionCall.Name,
					Arguments: string(p.FunctionCall.Args),
				},
			})
		}
	}

	return &llm.Response{
		Content:   textContent,
		ToolCalls: toolCalls,
		Usage: llm.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}
}
