package dto

import (
	"encoding/json"
	"time"

	"github.com/devteam/backend/internal/models"
)

// TeamResponse — команда проекта с агентами (HTTP / MCP).
type TeamResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ProjectID string          `json:"project_id"`
	Type      string          `json:"type"`
	Agents    []AgentResponse `json:"agents"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ToolBindingResponse — привязка инструмента к агенту (read, HTTP / MCP team).
type ToolBindingResponse struct {
	ToolDefinitionID string `json:"tool_definition_id"`
	Name             string `json:"name"`
	Category         string `json:"category"`
}

// AgentResponse — краткое представление агента в составе команды.
type AgentResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Role         string                 `json:"role"`
	RoleDescription *string             `json:"role_description,omitempty"`
	Model        *string                `json:"model"`
	PromptID     *string                `json:"prompt_id,omitempty"`
	PromptName   *string                `json:"prompt_name"`
	SystemPrompt *string                `json:"system_prompt,omitempty"`
	CodeBackend  *string                `json:"code_backend"`
	ProviderKind *string                `json:"provider_kind,omitempty"`
	IsActive     bool                   `json:"is_active"`
	ToolBindings []ToolBindingResponse `json:"tool_bindings"`
}

// UpdateTeamRequest — частичное обновление команды (пока только имя).
type UpdateTeamRequest struct {
	Name *string `json:"name"`
}

// CreateTeamRequest — запрос на создание команды.
type CreateTeamRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

// CreateTeamAgentRequest — запрос на создание агента в команде.
type CreateTeamAgentRequest struct {
	Name            string   `json:"name" binding:"required"`
	Role            string   `json:"role" binding:"required"`
	ExecutionKind   string   `json:"execution_kind" binding:"required"`
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	Model           *string  `json:"model,omitempty"`
	ProviderKind    *string  `json:"provider_kind,omitempty"`
	CodeBackend     *string  `json:"code_backend,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
}


// ToTeamResponse маппит модель команды в DTO.
func ToTeamResponse(team *models.Team) TeamResponse {
	if team == nil {
		return TeamResponse{}
	}
	agents := make([]AgentResponse, 0, len(team.Agents))
	for i := range team.Agents {
		agents = append(agents, ToAgentResponse(&team.Agents[i]))
	}
	return TeamResponse{
		ID:        team.ID.String(),
		Name:      team.Name,
		ProjectID: team.ProjectID.String(),
		Type:      string(team.Type),
		Agents:    agents,
		CreatedAt: team.CreatedAt,
		UpdatedAt: team.UpdatedAt,
	}
}

// ToAgentResponse маппит агента в DTO (Prompt должен быть preloaded для prompt_name).
func ToAgentResponse(agent *models.Agent) AgentResponse {
	if agent == nil {
		return AgentResponse{}
	}
	var promptName *string
	if agent.Prompt != nil {
		n := agent.Prompt.Name
		promptName = &n
	}
	var promptID *string
	if agent.PromptID != nil {
		s := agent.PromptID.String()
		promptID = &s
	}
	var codeBackend *string
	if agent.CodeBackend != nil {
		s := string(*agent.CodeBackend)
		codeBackend = &s
	}
	var providerKind *string
	if agent.ProviderKind != nil {
		pk := string(*agent.ProviderKind)
		providerKind = &pk
	}
	tb := make([]ToolBindingResponse, 0, len(agent.ToolBindings))
	for i := range agent.ToolBindings {
		b := &agent.ToolBindings[i]
		name := ""
		category := ""
		if b.ToolDefinition != nil {
			name = b.ToolDefinition.Name
			category = b.ToolDefinition.Category
		}
		tb = append(tb, ToolBindingResponse{
			ToolDefinitionID: b.ToolDefinitionID.String(),
			Name:             name,
			Category:         category,
		})
	}
	var modelVal *string = agent.Model
	if modelVal == nil && agent.ExecutionKind == models.AgentExecutionKindSandbox && len(agent.CodeBackendSettings) > 0 {
		var settings struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(agent.CodeBackendSettings, &settings); err == nil && settings.Model != "" {
			modelVal = &settings.Model
		}
	}

	return AgentResponse{
		ID:              agent.ID.String(),
		Name:            agent.Name,
		Role:            string(agent.Role),
		RoleDescription: agent.RoleDescription,
		Model:           modelVal,
		PromptID:     promptID,
		PromptName:   promptName,
		SystemPrompt: agent.SystemPrompt,
		CodeBackend:  codeBackend,
		ProviderKind: providerKind,
		IsActive:     agent.IsActive,
		ToolBindings: tb,
	}
}

// TeamTypeResponse — тип команды (HTTP / MCP).
type TeamTypeResponse struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateTeamTypeRequest — запрос на создание типа команды.
type CreateTeamTypeRequest struct {
	Code string `json:"code" binding:"required"`
	Name string `json:"name" binding:"required"`
}

func ToTeamTypeResponse(tt *models.TeamTypeModel) TeamTypeResponse {
	if tt == nil {
		return TeamTypeResponse{}
	}
	return TeamTypeResponse{
		ID:        tt.ID.String(),
		Code:      tt.Code,
		Name:      tt.Name,
		IsSystem:  tt.IsSystem,
		CreatedAt: tt.CreatedAt,
		UpdatedAt: tt.UpdatedAt,
	}
}
