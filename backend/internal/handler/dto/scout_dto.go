package dto

import (
	"encoding/json"
	"time"

	"github.com/devteam/backend/internal/models"
)

// UpdateScoutConfigRequest — частичное обновление конфига разведчика (PUT).
// Конфиг создаётся лениво при первом апдейте; до этого GET отдаёт дефолт.
// Агентные поля зеркалят настройку агента (provider/model/temperature +
// code_backend_settings + sandbox_permissions); отличие — всегда sandbox.
type UpdateScoutConfigRequest struct {
	IsEnabled *bool `json:"is_enabled"`
	// Prompt — редактируемый промпт; "" → сброс на встроенный дефолт.
	Prompt *string `json:"prompt"`
	// CodeBackend — claude-code/hermes/antigravity.
	CodeBackend *string `json:"code_backend"`
	// ProviderKind — kind LLM-провайдера; "" → сброс (NULL). Ограничен бэкендом
	// (hermes → anthropic/openrouter/hermes; antigravity → antigravity*).
	ProviderKind *string `json:"provider_kind"`
	// Temperature — параметр LLM (0..2).
	Temperature *float64 `json:"temperature"`
	// CodeBackendSettings — JSON-объект: model/mcp_servers/skills/hermes-блок.
	CodeBackendSettings json.RawMessage `json:"code_backend_settings,omitempty" swaggertype:"object"`
	// SandboxPermissions — JSON-объект: allow/deny/ask/defaultMode.
	SandboxPermissions json.RawMessage `json:"sandbox_permissions,omitempty" swaggertype:"object"`
	// SubscriptionID — UUID подключённой подписки; "" → сброс на дефолтную подписку владельца.
	SubscriptionID *string `json:"subscription_id"`
	TimeoutSeconds *int    `json:"timeout_seconds"`
}

// ScoutConfigResponse — конфиг разведчика проекта.
type ScoutConfigResponse struct {
	ProjectID           string          `json:"project_id"`
	IsEnabled           bool            `json:"is_enabled"`
	Prompt              string          `json:"prompt"`
	CodeBackend         string          `json:"code_backend"`
	ProviderKind        *string         `json:"provider_kind,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings" swaggertype:"object"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions" swaggertype:"object"`
	SubscriptionID      *string         `json:"subscription_id,omitempty"`
	TimeoutSeconds      int             `json:"timeout_seconds"`
}

// ToScoutConfigResponse маппит модель в DTO.
func ToScoutConfigResponse(cfg *models.ScoutConfig) ScoutConfigResponse {
	if cfg == nil {
		return ScoutConfigResponse{}
	}
	resp := ScoutConfigResponse{
		ProjectID:           cfg.ProjectID.String(),
		IsEnabled:           cfg.IsEnabled,
		Prompt:              cfg.Prompt,
		CodeBackend:         string(cfg.CodeBackend),
		Temperature:         cfg.Temperature,
		CodeBackendSettings: jsonOrEmptyObject(cfg.CodeBackendSettings),
		SandboxPermissions:  jsonOrEmptyObject(cfg.SandboxPermissions),
		TimeoutSeconds:      cfg.TimeoutSeconds,
	}
	if cfg.ProviderKind != nil {
		pk := string(*cfg.ProviderKind)
		resp.ProviderKind = &pk
	}
	if cfg.SubscriptionID != nil {
		s := cfg.SubscriptionID.String()
		resp.SubscriptionID = &s
	}
	return resp
}

// jsonOrEmptyObject отдаёт {} для пустого jsonb (синтетический дефолт не персистится).
func jsonOrEmptyObject(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(raw)
}

// DispatchScoutRequest — запуск прогона разведчика по постановке проблемы.
type DispatchScoutRequest struct {
	Problem string `json:"problem" binding:"required"`
}

// ScoutRunResponse — карточка прогона разведчика.
type ScoutRunResponse struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	Status            string     `json:"status"`
	CodeBackend       string     `json:"code_backend"`
	Problem           string     `json:"problem"`
	Dossier           string     `json:"dossier"`
	Error             string     `json:"error,omitempty"`
	SandboxInstanceID string     `json:"sandbox_instance_id,omitempty"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}

// ScoutRunListResponse — список прогонов проекта.
type ScoutRunListResponse struct {
	Runs  []ScoutRunResponse `json:"runs"`
	Total int                `json:"total"`
}

// ToScoutRunResponse маппит модель в DTO.
func ToScoutRunResponse(run *models.ScoutRun) ScoutRunResponse {
	if run == nil {
		return ScoutRunResponse{}
	}
	return ScoutRunResponse{
		ID:                run.ID.String(),
		ProjectID:         run.ProjectID.String(),
		Status:            string(run.Status),
		CodeBackend:       string(run.CodeBackend),
		Problem:           run.Problem,
		Dossier:           run.Dossier,
		Error:             run.Error,
		SandboxInstanceID: run.SandboxInstanceID,
		StartedAt:         run.StartedAt,
		FinishedAt:        run.FinishedAt,
	}
}

// ToScoutRunListResponse оборачивает список прогонов.
func ToScoutRunListResponse(items []models.ScoutRun) ScoutRunListResponse {
	out := make([]ScoutRunResponse, 0, len(items))
	for i := range items {
		out = append(out, ToScoutRunResponse(&items[i]))
	}
	return ScoutRunListResponse{Runs: out, Total: len(out)}
}
