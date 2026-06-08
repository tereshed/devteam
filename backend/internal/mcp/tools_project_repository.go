package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RepoListParams — параметры project_repository_list.
type RepoListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
}

// RepoAddParams — параметры project_repository_add (как POST /projects/:id/repositories).
type RepoAddParams struct {
	ProjectID        string  `json:"project_id" jsonschema:"UUID проекта"`
	Slug             string  `json:"slug" jsonschema:"Короткий стабильный идентификатор репо (напр. ui, core, infra)"`
	DisplayName      string  `json:"display_name" jsonschema:"Человекочитаемое имя репозитория"`
	RoleDescription  *string `json:"role_description,omitempty" jsonschema:"Роль репо для decomposer'а (напр. 'Flutter UI', 'высоконагруженный Go-бэкенд')"`
	GitProvider      *string `json:"git_provider,omitempty" jsonschema:"Git-провайдер (local, github, gitlab, bitbucket)"`
	GitURL           string  `json:"git_url" jsonschema:"URL git-репозитория"`
	GitDefaultBranch *string `json:"git_default_branch,omitempty" jsonschema:"Ветка по умолчанию (default: main)"`
	IsPrimary        *bool   `json:"is_primary,omitempty" jsonschema:"Сделать репо primary"`
}

// RepoRemoveParams — параметры project_repository_remove.
type RepoRemoveParams struct {
	ProjectID    string `json:"project_id" jsonschema:"UUID проекта"`
	RepositoryID string `json:"repository_id" jsonschema:"UUID репозитория"`
}

// RegisterProjectRepositoryTools регистрирует MCP-инструменты для репозиториев проекта (мульти-репо).
func RegisterProjectRepositoryTools(server *mcp.Server, projectSvc service.ProjectService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_repository_list",
		Description: "Список git-репозиториев проекта (мульти-репо). Как GET /projects/:id/repositories.",
	}, makeRepoListHandler(projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_repository_add",
		Description: "Добавить git-репозиторий в проект. Первый репозиторий становится primary. Как POST /projects/:id/repositories.",
	}, makeRepoAddHandler(projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_repository_remove",
		Description: "Удалить git-репозиторий из проекта. Primary нельзя удалить, пока есть другие репозитории. Как DELETE /projects/:id/repositories/:repoId.",
	}, makeRepoRemoveHandler(projectSvc))
}

func makeRepoListHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *RepoListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *RepoListParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		pid, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}
		repos, err := projectSvc.ListRepositories(ctx, uid, role, pid)
		if err != nil {
			return repositoryServiceMCPError(err)
		}
		data := dto.ToProjectRepositoryListResponse(repos)
		return OK(fmt.Sprintf("found %d repositories", data.Total), data)
	}
}

func makeRepoAddHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *RepoAddParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *RepoAddParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.Slug == "" || params.GitURL == "" {
			return ValidationErr("slug and git_url are required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		pid, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}
		addReq := dto.AddRepositoryRequest{
			Slug:        params.Slug,
			DisplayName: params.DisplayName,
			GitURL:      params.GitURL,
		}
		if params.DisplayName == "" {
			addReq.DisplayName = params.Slug
		}
		if params.RoleDescription != nil {
			addReq.RoleDescription = *params.RoleDescription
		}
		if params.GitProvider != nil {
			addReq.GitProvider = *params.GitProvider
		}
		if params.GitDefaultBranch != nil {
			addReq.GitDefaultBranch = *params.GitDefaultBranch
		}
		if params.IsPrimary != nil {
			addReq.IsPrimary = *params.IsPrimary
		}
		repo, err := projectSvc.AddRepository(ctx, uid, role, pid, addReq)
		if err != nil {
			return repositoryServiceMCPError(err)
		}
		data := dto.ToProjectRepositoryResponse(repo)
		return OK(fmt.Sprintf("repository %q (slug=%s) added", data.DisplayName, data.Slug), data)
	}
}

func makeRepoRemoveHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *RepoRemoveParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *RepoRemoveParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" || params.RepositoryID == "" {
			return ValidationErr("project_id and repository_id are required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		pid, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}
		rid, err := uuid.Parse(params.RepositoryID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid repository_id: %q", params.RepositoryID))
		}
		if err := projectSvc.RemoveRepository(ctx, uid, role, pid, rid); err != nil {
			return repositoryServiceMCPError(err)
		}
		return OK(fmt.Sprintf("repository %s removed", rid), map[string]string{"repository_id": rid.String()})
	}
}

func repositoryServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound), errors.Is(err, service.ErrRepoNotFound):
		return Err("repository not found", err)
	case errors.Is(err, service.ErrProjectForbidden), errors.Is(err, service.ErrGitCredentialForbidden):
		return Err("access denied", err)
	case errors.Is(err, service.ErrRepoSlugExists):
		return Err("repository slug already exists", err)
	case errors.Is(err, service.ErrGitValidationFailed):
		return Err("git repository validation failed", err)
	case errors.Is(err, service.ErrRepoSlugRequired),
		errors.Is(err, service.ErrRepoURLRequired),
		errors.Is(err, service.ErrProjectInvalidProvider),
		errors.Is(err, service.ErrCannotRemovePrimaryRepo):
		return ValidationErr(err.Error())
	default:
		return Err("repository operation failed", err)
	}
}
