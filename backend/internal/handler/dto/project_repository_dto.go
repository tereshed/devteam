package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// AddRepositoryRequest — запрос на добавление репозитория в проект (мульти-репо).
type AddRepositoryRequest struct {
	Slug             string     `json:"slug" binding:"required,min=1,max=64"`
	DisplayName      string     `json:"display_name" binding:"required,min=1,max=255"`
	RoleDescription  string     `json:"role_description"`
	GitProvider      string     `json:"git_provider"`
	GitURL           string     `json:"git_url" binding:"required,url"`
	GitDefaultBranch string     `json:"git_default_branch"`
	GitCredentialID  *uuid.UUID `json:"git_credential_id"`
	// GitIntegrationCredentialID — выбранный OAuth-аккаунт провайдера для этого репо (мульти-аккаунт).
	GitIntegrationCredentialID *uuid.UUID `json:"git_integration_credential_id"`
	IsPrimary                  bool       `json:"is_primary"`
	SortOrder                  int        `json:"sort_order"`
}

// UpdateRepositoryRequest — частичное обновление репозитория (nil — поле не меняется).
type UpdateRepositoryRequest struct {
	DisplayName                    *string    `json:"display_name"`
	RoleDescription                *string    `json:"role_description"`
	GitProvider                    *string    `json:"git_provider"`
	GitURL                         *string    `json:"git_url"`
	GitDefaultBranch               *string    `json:"git_default_branch"`
	GitCredentialID                *uuid.UUID `json:"git_credential_id"`
	GitIntegrationCredentialID     *uuid.UUID `json:"git_integration_credential_id"`
	RemoveGitIntegrationCredential bool       `json:"remove_git_integration_credential"`
	IsPrimary                      *bool      `json:"is_primary"`
	SortOrder                      *int       `json:"sort_order"`
}

// ProjectRepositoryResponse — данные репозитория проекта (без секретов).
type ProjectRepositoryResponse struct {
	ID                         string                `json:"id"`
	ProjectID                  string                `json:"project_id"`
	Slug                       string                `json:"slug"`
	DisplayName                string                `json:"display_name"`
	RoleDescription            string                `json:"role_description"`
	GitProvider                string                `json:"git_provider"`
	GitURL                     string                `json:"git_url"`
	GitDefaultBranch           string                `json:"git_default_branch"`
	GitCredential              *GitCredentialSummary `json:"git_credential,omitempty"`
	GitIntegrationCredentialID *string               `json:"git_integration_credential_id,omitempty"`
	VectorCollection           string                `json:"vector_collection"`
	LastIndexedCommit          string                `json:"last_indexed_commit"`
	Status                     string                `json:"status"`
	IsPrimary                  bool                  `json:"is_primary"`
	SortOrder                  int                   `json:"sort_order"`
	CreatedAt                  time.Time             `json:"created_at"`
	UpdatedAt                  time.Time             `json:"updated_at"`
}

// ProjectRepositoryListResponse — список репозиториев проекта.
type ProjectRepositoryListResponse struct {
	Repositories []ProjectRepositoryResponse `json:"repositories"`
	Total        int                         `json:"total"`
}

func integrationIDToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// ToProjectRepositoryResponse маппинг models.ProjectRepository → DTO.
func ToProjectRepositoryResponse(r *models.ProjectRepository) ProjectRepositoryResponse {
	if r == nil {
		return ProjectRepositoryResponse{}
	}
	return ProjectRepositoryResponse{
		ID:                         r.ID.String(),
		ProjectID:                  r.ProjectID.String(),
		Slug:                       r.Slug,
		DisplayName:                r.DisplayName,
		RoleDescription:            r.RoleDescription,
		GitProvider:                string(r.GitProvider),
		GitURL:                     r.GitURL,
		GitDefaultBranch:           r.GitDefaultBranch,
		GitCredential:              ToGitCredentialSummary(r.GitCredential),
		GitIntegrationCredentialID: integrationIDToString(r.GitIntegrationCredentialID),
		VectorCollection:           r.VectorCollection,
		LastIndexedCommit:          r.LastIndexedCommit,
		Status:                     string(r.Status),
		IsPrimary:                  r.IsPrimary,
		SortOrder:                  r.SortOrder,
		CreatedAt:                  r.CreatedAt,
		UpdatedAt:                  r.UpdatedAt,
	}
}

// ToProjectRepositoryListResponse список репозиториев проекта.
func ToProjectRepositoryListResponse(repos []models.ProjectRepository) ProjectRepositoryListResponse {
	out := make([]ProjectRepositoryResponse, 0, len(repos))
	for i := range repos {
		out = append(out, ToProjectRepositoryResponse(&repos[i]))
	}
	return ProjectRepositoryListResponse{Repositories: out, Total: len(out)}
}
