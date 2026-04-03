package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/pkg/llm"
)

// --- Валидация ---

func TestLLMGenerate_NilParams(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_EmptyPrompt(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{Prompt: "   "})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_PromptTooLong(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	cfg.MaxPromptRunes = 10
	handler := makeLLMGenerateHandler(svc, cfg)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{Prompt: "hello world this is too long"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_UnknownProvider(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:   "hello",
		Provider: "unknown_provider",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_TemperatureOutOfRange(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	temp := 3.0
	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:      "hello",
		Temperature: &temp,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_MaxTokensOutOfRange(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	tokens := 99999
	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:    "hello",
		MaxTokens: &tokens,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_SystemPromptTooLong(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	cfg.MaxPromptRunes = 10
	handler := makeLLMGenerateHandler(svc, cfg)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:       "hello",
		SystemPrompt: "this system prompt is way too long for the limit",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestLLMGenerate_ProviderCaseInsensitive(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	svc.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
		return req.Provider == llm.ProviderOpenAI
	})).Return(&llm.Response{Content: "ok", Usage: llm.Usage{TotalTokens: 10}}, nil)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:   "hello",
		Provider: "OpenAI", // uppercase
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

// --- Успешная генерация ---

func TestLLMGenerate_Success(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	svc.On("Generate", mock.Anything, mock.Anything).Return(&llm.Response{
		Content: "Generated text",
		Usage:   llm.Usage{TotalTokens: 42},
	}, nil)

	result, structured, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt: "hello world",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	resp := structured.(*Response)
	assert.Equal(t, StatusOK, resp.Status)
	assert.Contains(t, resp.Details, "42 tokens")

	data := resp.Data.(*LLMGenerateData)
	assert.Equal(t, "Generated text", data.Content)
	assert.Equal(t, "(default)", data.Provider)
	assert.Equal(t, "(default)", data.Model)
	svc.AssertExpectations(t)
}

func TestLLMGenerate_WithOptionalParams(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	temp := 0.7
	tokens := 100

	svc.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
		return req.Temperature == 0.7 && req.MaxTokens == 100 && req.SystemPrompt == "Be helpful"
	})).Return(&llm.Response{
		Content: "ok",
		Usage:   llm.Usage{TotalTokens: 5},
	}, nil)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
		Prompt:       "test",
		Temperature:  &temp,
		MaxTokens:    &tokens,
		SystemPrompt: "Be helpful",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestLLMGenerate_ServiceError(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	svc.On("Generate", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{Prompt: "hello"})

	require.NoError(t, err) // HTTP всегда 2xx
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestLLMGenerate_TrimWhitespace(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	handler := makeLLMGenerateHandler(svc, cfg)

	svc.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
		return req.Messages[0].Content == "hello" // trimmed
	})).Return(&llm.Response{Content: "ok", Usage: llm.Usage{}}, nil)

	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{Prompt: "  hello  "})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestLLMGenerate_PromptTooLong_Unicode(t *testing.T) {
	svc := new(mockLLMService)
	cfg := defaultMCPConfig()
	cfg.MaxPromptRunes = 5
	handler := makeLLMGenerateHandler(svc, cfg)

	// "Привет" = 6 рун, но больше 6 байт
	result, _, err := handler(context.Background(), nil, &LLMGenerateParams{Prompt: "Привет"})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	// "Приве" = 5 рун — должно пройти
	svc.On("Generate", mock.Anything, mock.Anything).Return(&llm.Response{Content: "ok", Usage: llm.Usage{}}, nil)
	result, _, err = handler(context.Background(), nil, &LLMGenerateParams{Prompt: "Приве"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestLLMGenerate_AllAllowedProviders(t *testing.T) {
	providers := []string{"openai", "anthropic", "gemini", "deepseek", "qwen", ""}
	for _, p := range providers {
		t.Run("provider_"+p, func(t *testing.T) {
			svc := new(mockLLMService)
			cfg := defaultMCPConfig()
			handler := makeLLMGenerateHandler(svc, cfg)

			svc.On("Generate", mock.Anything, mock.Anything).Return(
				&llm.Response{Content: "ok", Usage: llm.Usage{}}, nil)

			result, _, err := handler(context.Background(), nil, &LLMGenerateParams{
				Prompt:   "test",
				Provider: strings.ToUpper(p), // проверяем case-insensitivity
			})
			require.NoError(t, err)
			assert.False(t, result.IsError, "provider %q should be allowed", p)
		})
	}
}
