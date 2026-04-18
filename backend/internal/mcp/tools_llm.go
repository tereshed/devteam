package mcp

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/llm"
)

// --- Провайдеры ---

// allowedProviders строится из констант пакета llm
var allowedProviders = func() map[llm.ProviderType]bool {
	m := map[llm.ProviderType]bool{
		"": true, // пустой — провайдер по умолчанию
	}
	for _, p := range []llm.ProviderType{
		llm.ProviderOpenAI, llm.ProviderAnthropic,
		llm.ProviderGemini, llm.ProviderDeepseek, llm.ProviderQwen,
	} {
		m[p] = true
	}
	return m
}()

var allowedProvidersList = func() string {
	names := []string{
		string(llm.ProviderOpenAI), string(llm.ProviderAnthropic),
		string(llm.ProviderGemini), string(llm.ProviderDeepseek), string(llm.ProviderQwen),
	}
	return strings.Join(names, ", ")
}()

// --- Params ---

// LLMGenerateParams — входные параметры инструмента llm_generate.
// Поля с *float64 / *int позволяют отличить "не передано" (nil) от "передано 0".
type LLMGenerateParams struct {
	Prompt       string   `json:"prompt" jsonschema:"description=Текстовый запрос к LLM,required"`
	Provider     string   `json:"provider,omitempty" jsonschema:"description=LLM провайдер (openai/anthropic/gemini/deepseek/qwen). Если не указан — провайдер по умолчанию"`
	Model        string   `json:"model,omitempty" jsonschema:"description=Модель провайдера (например gpt-4o). Если не указана — модель по умолчанию"`
	SystemPrompt string   `json:"system_prompt,omitempty" jsonschema:"description=Системный промпт для задания контекста/роли LLM"`
	Temperature  *float64 `json:"temperature,omitempty" jsonschema:"description=Температура генерации (0.0-2.0). Не указывайте для дефолта провайдера"`
	MaxTokens    *int     `json:"max_tokens,omitempty" jsonschema:"description=Максимальное количество токенов в ответе. Не указывайте для дефолта провайдера"`
}

// --- Data (payload внутри Response.Data) ---

// LLMGenerateData — данные успешного ответа llm_generate
type LLMGenerateData struct {
	Content   string         `json:"content"`
	Provider  string         `json:"provider"`            // фактически запрошенный провайдер ("(default)" если не указан)
	Model     string         `json:"model"`               // фактически запрошенная модель ("(default)" если не указана)
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
	Usage     llm.Usage      `json:"usage"`
}

// --- Registration ---

// RegisterLLMTools регистрирует MCP-инструменты для работы с LLM.
// Лимиты берутся из MCPConfig (конфигурируются через env: MCP_MAX_PROMPT_RUNES, MCP_MAX_TOKENS_LIMIT).
func RegisterLLMTools(server *mcp.Server, llmService service.LLMService, cfg config.MCPConfig) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "llm_generate",
		Description: "Генерация текста через LLM. Поддерживает OpenAI, Anthropic, Gemini, Deepseek, Qwen.",
	}, makeLLMGenerateHandler(llmService, cfg))
}

// --- Handler ---

func makeLLMGenerateHandler(llmService service.LLMService, cfg config.MCPConfig) func(ctx context.Context, req *mcp.CallToolRequest, params *LLMGenerateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *LLMGenerateParams) (*mcp.CallToolResult, any, error) {

		// --- Валидация ---

		if params == nil {
			return ValidationErr("parameters are required (at minimum: prompt)")
		}

		// Нормализация prompt: убираем пробелы по краям
		params.Prompt = strings.TrimSpace(params.Prompt)

		if params.Prompt == "" {
			return ValidationErr("prompt is required (non-empty after trimming whitespace)")
		}
		promptRunes := utf8.RuneCountInString(params.Prompt)
		if promptRunes > cfg.MaxPromptRunes {
			return ValidationErr(fmt.Sprintf(
				"prompt too long: %d runes (max %d)", promptRunes, cfg.MaxPromptRunes))
		}

		// Нормализация и валидация provider
		provider := llm.ProviderType(strings.ToLower(strings.TrimSpace(params.Provider)))
		if !allowedProviders[provider] {
			return ValidationErr(fmt.Sprintf(
				"unknown provider %q; allowed: %s (or empty for default)", params.Provider, allowedProvidersList))
		}

		// Валидация system_prompt длины (аналогичный лимит)
		if utf8.RuneCountInString(params.SystemPrompt) > cfg.MaxPromptRunes {
			return ValidationErr(fmt.Sprintf(
				"system_prompt too long: %d runes (max %d)", utf8.RuneCountInString(params.SystemPrompt), cfg.MaxPromptRunes))
		}

		if params.Temperature != nil {
			if *params.Temperature < 0 || *params.Temperature > 2.0 {
				return ValidationErr(fmt.Sprintf(
					"temperature must be between 0.0 and 2.0, got: %.2f", *params.Temperature))
			}
		}

		if params.MaxTokens != nil {
			if *params.MaxTokens < 1 || *params.MaxTokens > cfg.MaxTokensLimit {
				return ValidationErr(fmt.Sprintf(
					"max_tokens must be between 1 and %d, got: %d", cfg.MaxTokensLimit, *params.MaxTokens))
			}
		}

		// --- Собираем llm.Request ---
		// Temperature / MaxTokens — указатели: nil = не передавать провайдеру; иначе передаём, в т.ч. 0.

		llmReq := llm.Request{
			Provider: provider,
			Model:    params.Model,
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: params.Prompt},
			},
			SystemPrompt: params.SystemPrompt,
		}
		if params.Temperature != nil {
			llmReq.Temperature = params.Temperature
		}
		if params.MaxTokens != nil {
			llmReq.MaxTokens = params.MaxTokens
		}

		// --- Вызов сервиса ---

		resp, err := llmService.Generate(ctx, llmReq)
		if err != nil {
			return Err("generation failed; check server logs for details", err)
		}

		// --- Формируем ответ ---

		// Для details и data показываем "(default)" когда пользователь не указал значение,
		// чтобы клиент понимал, что использованы дефолтные настройки сервера.
		displayProvider := string(provider)
		if displayProvider == "" {
			displayProvider = "(default)"
		}
		displayModel := params.Model
		if displayModel == "" {
			displayModel = "(default)"
		}

		return OK(
			fmt.Sprintf("generated %d tokens via %s/%s",
				resp.Usage.TotalTokens, displayProvider, displayModel),
			&LLMGenerateData{
				Content:   resp.Content,
				Provider:  displayProvider,
				Model:     displayModel,
				ToolCalls: resp.ToolCalls,
				Usage:     resp.Usage,
			},
		)
	}
}
