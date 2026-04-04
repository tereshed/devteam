package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// CreateProjectRequest запрос на создание проекта
type CreateProjectRequest struct {
	Name             string         `json:"name" binding:"required,min=1,max=255"`
	Description      string         `json:"description"`
	GitProvider      string         `json:"git_provider"`
	GitURL           string         `json:"git_url" binding:"omitempty,url"`
	GitDefaultBranch string         `json:"git_default_branch"`
	GitCredentialID  *uuid.UUID     `json:"git_credential_id"`
	VectorCollection string         `json:"vector_collection"`
	TechStack        datatypes.JSON `json:"tech_stack" swaggertype:"string"`
	Status           string         `json:"status"`
	Settings         datatypes.JSON `json:"settings" swaggertype:"string"`
}

// ListProjectsRequest фильтры и пагинация списка проектов
type ListProjectsRequest struct {
	Status      *string `form:"status"`
	GitProvider *string `form:"git_provider"`
	Search      *string `form:"search"`
	Limit       int     `form:"limit"`
	Offset      int     `form:"offset"`
	OrderBy     string  `form:"order_by"`
	OrderDir    string  `form:"order_dir"`
}

// UpdateProjectRequest частичное обновление проекта (nil / пусто — поле не меняется).
//
// Отвязка git credential: JSON null и отсутствие поля дают nil в Go — используйте RemoveGitCredential=true.
// Сброс jsonb tech_stack/settings до пустого объекта: ClearTechStack / ClearSettings (или TechStack/Settings как указатель).
type UpdateProjectRequest struct {
	Name                 *string         `json:"name"`
	Description          *string         `json:"description"`
	GitProvider          *string         `json:"git_provider"`
	GitURL               *string         `json:"git_url"`
	GitDefaultBranch     *string         `json:"git_default_branch"`
	GitCredentialID      *uuid.UUID      `json:"git_credential_id"`
	RemoveGitCredential  bool            `json:"remove_git_credential"`
	VectorCollection     *string         `json:"vector_collection"`
	TechStack            *datatypes.JSON `json:"tech_stack" swaggertype:"string"`
	Settings             *datatypes.JSON `json:"settings" swaggertype:"string"`
	ClearTechStack       bool            `json:"clear_tech_stack"`
	ClearSettings        bool            `json:"clear_settings"`
	Status               *string         `json:"status"`
}

// GitCredentialSummary краткие данные credential без секретов.
type GitCredentialSummary struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	AuthType string `json:"auth_type"`
	Label    string `json:"label"`
}

// TeamSummaryResponse краткая информация о команде проекта.
type TeamSummaryResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	AgentCount int    `json:"agent_count"`
}

// ProjectResponse ответ с данными проекта (GET /projects/:id).
type ProjectResponse struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	GitProvider      string                `json:"git_provider"`
	GitURL           string                `json:"git_url"`
	GitDefaultBranch string                `json:"git_default_branch"`
	GitCredential    *GitCredentialSummary `json:"git_credential,omitempty"` // nil — ключ в JSON не выводится
	VectorCollection string                `json:"vector_collection"`
	TechStack        datatypes.JSON        `json:"tech_stack" swaggertype:"string"`
	Status           string                `json:"status"`
	Settings         datatypes.JSON        `json:"settings" swaggertype:"string"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// ProjectListResponse пагинированный список проектов.
type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// ToGitCredentialSummary преобразует модель в DTO без encrypted_value.
func ToGitCredentialSummary(gc *models.GitCredential) *GitCredentialSummary {
	if gc == nil {
		return nil
	}
	return &GitCredentialSummary{
		ID:       gc.ID.String(),
		Provider: string(gc.Provider),
		AuthType: string(gc.AuthType),
		Label:    gc.Label,
	}
}

// ToProjectResponse маппинг models.Project → ProjectResponse.
func ToProjectResponse(p *models.Project) ProjectResponse {
	if p == nil {
		return ProjectResponse{}
	}
	return ProjectResponse{
		ID:               p.ID.String(),
		Name:             p.Name,
		Description:      p.Description,
		GitProvider:      string(p.GitProvider),
		GitURL:           p.GitURL,
		GitDefaultBranch: p.GitDefaultBranch,
		GitCredential:    ToGitCredentialSummary(p.GitCredential),
		VectorCollection: p.VectorCollection,
		TechStack:        p.TechStack,
		Status:           string(p.Status),
		Settings:         p.Settings,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

// ToProjectListResponse обёртка списка проектов с метаданными пагинации.
func ToProjectListResponse(projects []models.Project, total int64, limit, offset int) ProjectListResponse {
	out := make([]ProjectResponse, 0, len(projects))
	for i := range projects {
		out = append(out, ToProjectResponse(&projects[i]))
	}
	return ProjectListResponse{
		Projects: out,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}
}
