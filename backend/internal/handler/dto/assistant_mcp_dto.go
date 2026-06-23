package dto

import (
	"encoding/json"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// CreateAssistantMCPServerRequest — тело создания MCP-сервера ассистента.
// Transport — только http|sse (remote-only). Headers могут содержать ${secret:NAME}.
type CreateAssistantMCPServerRequest struct {
	Name                string            `json:"name" binding:"required"`
	Transport           string            `json:"transport" binding:"required"`
	URL                 string            `json:"url" binding:"required"`
	Headers             map[string]string `json:"headers"`
	RequireConfirmation *bool             `json:"require_confirmation"`
	IsEnabled           *bool             `json:"is_enabled"`
}

// UpdateAssistantMCPServerRequest — тело обновления (полная замена полей).
type UpdateAssistantMCPServerRequest struct {
	Name                string            `json:"name" binding:"required"`
	Transport           string            `json:"transport" binding:"required"`
	URL                 string            `json:"url" binding:"required"`
	Headers             map[string]string `json:"headers"`
	RequireConfirmation *bool             `json:"require_confirmation"`
	IsEnabled           *bool             `json:"is_enabled"`
}

// AssistantMCPServerResponse — представление MCP-сервера. Headers возвращаются
// как есть (включая ${secret:NAME}-ссылки) — для редактирования в UI; реальные
// секреты в таблице не хранятся (живут в project_secrets).
type AssistantMCPServerResponse struct {
	ID                  uuid.UUID         `json:"id"`
	ProjectID           uuid.UUID         `json:"project_id"`
	Name                string            `json:"name"`
	Transport           string            `json:"transport"`
	URL                 string            `json:"url"`
	Headers             map[string]string `json:"headers"`
	RequireConfirmation bool              `json:"require_confirmation"`
	IsEnabled           bool              `json:"is_enabled"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// AssistantMCPServerListResponse — список серверов проекта.
type AssistantMCPServerListResponse struct {
	Servers []AssistantMCPServerResponse `json:"servers"`
}

// ToAssistantMCPServerResponse маппит модель в ответ.
func ToAssistantMCPServerResponse(m *models.AssistantMCPServer) AssistantMCPServerResponse {
	headers := map[string]string{}
	if len(m.Headers) > 0 {
		_ = json.Unmarshal(m.Headers, &headers)
	}
	return AssistantMCPServerResponse{
		ID:                  m.ID,
		ProjectID:           m.ProjectID,
		Name:                m.Name,
		Transport:           string(m.Transport),
		URL:                 m.URL,
		Headers:             headers,
		RequireConfirmation: m.RequireConfirmation,
		IsEnabled:           m.IsEnabled,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

// ToAssistantMCPServerListResponse маппит список моделей в ответ.
func ToAssistantMCPServerListResponse(items []models.AssistantMCPServer) AssistantMCPServerListResponse {
	out := make([]AssistantMCPServerResponse, 0, len(items))
	for i := range items {
		out = append(out, ToAssistantMCPServerResponse(&items[i]))
	}
	return AssistantMCPServerListResponse{Servers: out}
}
