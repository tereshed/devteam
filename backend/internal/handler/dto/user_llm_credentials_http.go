package dto

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Контракт HTTP для /me/llm-credentials (SSOT: docs/tasks/13.5-backend-user-llm-credentials.md).

// LlmProviderMaskView — одна карточка провайдера в ответе GET/PATCH 200.
// MaskedPreview: null если ключа нет; иначе ровно 8 символов (**** + последние 4 руны).
type LlmProviderMaskView struct {
	MaskedPreview *string `json:"masked_preview" example:"****3Mk9" extensions:"x-nullable"`
}

// LlmCredentialsResponse — тело GET и успешного PATCH 200 (ровно шесть провайдеров).
type LlmCredentialsResponse struct {
	OpenAI     LlmProviderMaskView `json:"openai"`
	Anthropic  LlmProviderMaskView `json:"anthropic"`
	Gemini     LlmProviderMaskView `json:"gemini"`
	DeepSeek   LlmProviderMaskView `json:"deepseek"`
	Qwen       LlmProviderMaskView `json:"qwen"`
	OpenRouter LlmProviderMaskView `json:"openrouter"`
}

// PatchLlmCredentialsRequest — частичное обновление ключей (write-only поля для Swagger).
type PatchLlmCredentialsRequest struct {
	OpenAIAPIKey       *string `json:"openai_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearOpenAIKey     *bool   `json:"clear_openai_key,omitempty"`
	AnthropicAPIKey    *string `json:"anthropic_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearAnthropicKey  *bool   `json:"clear_anthropic_key,omitempty"`
	GeminiAPIKey       *string `json:"gemini_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearGeminiKey     *bool   `json:"clear_gemini_key,omitempty"`
	DeepSeekAPIKey     *string `json:"deepseek_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearDeepSeekKey   *bool   `json:"clear_deepseek_key,omitempty"`
	QwenAPIKey         *string `json:"qwen_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearQwenKey       *bool   `json:"clear_qwen_key,omitempty"`
	OpenRouterAPIKey   *string `json:"openrouter_api_key,omitempty" format:"password" swaggertype:"string"`
	ClearOpenRouterKey *bool   `json:"clear_openrouter_key,omitempty"`
}

// ErrTrailingJSONInPatchBody — после основного объекта в теле остался ещё JSON.
var ErrTrailingJSONInPatchBody = errors.New("trailing json after patch object")

// DecodePatchLlmCredentialsJSON разбирает тело PATCH с DisallowUnknownFields (один JSON-объект).
func DecodePatchLlmCredentialsJSON(body []byte) (*PatchLlmCredentialsRequest, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var req PatchLlmCredentialsRequest
	if err := dec.Decode(&req); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, ErrTrailingJSONInPatchBody
	}
	return &req, nil
}
