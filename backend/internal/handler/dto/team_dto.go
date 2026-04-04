package dto

import (
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

// AgentResponse — краткое представление агента в составе команды.
type AgentResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Model       *string `json:"model"`
	PromptName  *string `json:"prompt_name"`
	CodeBackend *string `json:"code_backend"`
	IsActive    bool    `json:"is_active"`
}

// UpdateTeamRequest — частичное обновление команды (пока только имя).
type UpdateTeamRequest struct {
	Name *string `json:"name"`
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
	var codeBackend *string
	if agent.CodeBackend != nil {
		s := string(*agent.CodeBackend)
		codeBackend = &s
	}
	return AgentResponse{
		ID:          agent.ID.String(),
		Name:        agent.Name,
		Role:        string(agent.Role),
		Model:       agent.Model,
		PromptName:  promptName,
		CodeBackend: codeBackend,
		IsActive:    agent.IsActive,
	}
}
