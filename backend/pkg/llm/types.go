package llm

import (
	"encoding/json"
	"net/http"
)

// Role represents the role of the message sender
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderOpenAI           ProviderType = "openai"
	ProviderAnthropic        ProviderType = "anthropic"
	ProviderAnthropicOAuth   ProviderType = "anthropic_oauth"
	ProviderGemini           ProviderType = "gemini"
	ProviderDeepseek         ProviderType = "deepseek"
	ProviderQwen             ProviderType = "qwen"
	ProviderOpenRouter       ProviderType = "openrouter"
	ProviderMoonshot         ProviderType = "moonshot"
	ProviderOllama           ProviderType = "ollama"
	ProviderZhipu            ProviderType = "zhipu"
	ProviderAntigravity      ProviderType = "antigravity"
	ProviderAntigravityOAuth ProviderType = "antigravity_oauth"
)

// Config represents the configuration for a provider
type Config struct {
	APIKey  string
	BaseURL string
	// HTTPClient — опциональный custom http.Client (Sprint 15.N8).
	// Если задан — провайдер ОБЯЗАН использовать его вместо &http.Client{} (defense-in-depth:
	// этот клиент содержит SSRF-guard через DialContext.Control + CheckRedirect).
	HTTPClient *http.Client `json:"-"`
}

// Message represents a single message in the conversation
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`         // For tool messages
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // For assistant messages
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool messages
}

// ToolCall represents a request to call a tool
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents the function call details
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// Tool represents a tool definition (MCP compatible)
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema" swaggertype:"object"` // JSON Schema for arguments
}

// Request represents a generation request
type Request struct {
	Provider ProviderType `json:"provider,omitempty"`
	Model    string       `json:"model,omitempty"`
	// OwnerUserID — владелец проекта задачи (uuid-строка). Если задан, llmService
	// сперва ищет ключ провайдера в user_llm_credentials этого пользователя и
	// только при отсутствии записи падает на env-ключ процесса. json:"-": поле
	// не уходит провайдеру и не попадает в llm_logs.PromptSnapshot.
	OwnerUserID            string          `json:"-"`
	Messages               []Message       `json:"messages"`
	SystemPrompt           string          `json:"system_prompt,omitempty"`
	Tools                  []Tool          `json:"tools,omitempty"`
	StructuredOutputSchema json.RawMessage `json:"structured_output_schema,omitempty" swaggertype:"object"` // JSON Schema for structured output
	// Temperature — если не nil, передаётся провайдеру, в том числе при значении 0 (важно для YAML-конфигов агентов).
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   *int           `json:"max_tokens,omitempty"`
	Metadata    map[string]any `json:"-"` // Internal metadata for logging (not sent to provider)
	// ServerTools — провайдер-специфичные server-side тулы, исполняемые самим
	// провайдером (например {"type": "openrouter:web_search"} — OpenRouter сам
	// выполняет поиск и интегрирует результаты в ответ). Сериализуются в массив
	// tools как есть, в дополнение к function-тулам из Tools; tool_call для них
	// НЕ приходит — результат уже в тексте ответа (+ annotations/url_citation).
	// Поддерживается только oaicompat-провайдерами, понимающими нестандартный
	// type; для остальных оставляй пустым.
	ServerTools []map[string]any `json:"server_tools,omitempty"`
}

// Response represents the generation response
type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
