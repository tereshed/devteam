package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

// AgentSettingsResponse — ответ GET /agents/:id/settings (Sprint 15.23).
// Sprint 15.e2e: поле `llm_provider_id` удалено — kind провайдера хранится
// в agent.provider_kind (см. team API), а креденшелы — в user_llm_credentials.
type AgentSettingsResponse struct {
	AgentID             uuid.UUID       `json:"agent_id"`
	CodeBackend         *string         `json:"code_backend,omitempty"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings" swaggertype:"object"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions" swaggertype:"object"`
}

// UpdateAgentSettingsRequest — тело PUT /agents/:id/settings.
//
// CodeBackendSettings / SandboxPermissions передаются как сырой JSON;
// сервис валидирует структуру (см. service.ValidateSandboxPermissions).
// Sprint 15.e2e: поля `llm_provider_id` / `clear_llm_provider` удалены вместе
// с колонкой в БД.
type UpdateAgentSettingsRequest struct {
	CodeBackend         *string         `json:"code_backend,omitempty"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings,omitempty" swaggertype:"object"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions,omitempty" swaggertype:"object"`
}
