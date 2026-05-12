package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

// AgentSettingsResponse — ответ GET /agents/:id/settings (Sprint 15.23).
type AgentSettingsResponse struct {
	AgentID             uuid.UUID       `json:"agent_id"`
	LLMProviderID       *uuid.UUID      `json:"llm_provider_id,omitempty"`
	CodeBackend         *string         `json:"code_backend,omitempty"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings" swaggertype:"object"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions" swaggertype:"object"`
}

// UpdateAgentSettingsRequest — тело PUT /agents/:id/settings.
//
// LLMProviderID: явный null очищает связь (omitempty не используем, чтобы отличать "не передано" от "сбросить").
// CodeBackendSettings / SandboxPermissions передаются как сырой JSON; сервис валидирует структуру (см. service.ValidateSandboxPermissions).
type UpdateAgentSettingsRequest struct {
	LLMProviderID       *uuid.UUID      `json:"llm_provider_id,omitempty"`
	ClearLLMProvider    bool            `json:"clear_llm_provider,omitempty"`
	CodeBackend         *string         `json:"code_backend,omitempty"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings,omitempty" swaggertype:"object"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions,omitempty" swaggertype:"object"`
}
