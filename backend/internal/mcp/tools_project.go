package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
)

// ProjectListParams — параметры project_list (фильтры как у HTTP GET /projects).
type ProjectListParams struct {
	Status      *string `json:"status,omitempty" jsonschema:"description=Фильтр по статусу (active, paused, archived)"`
	GitProvider *string `json:"git_provider,omitempty" jsonschema:"description=Фильтр по git-провайдеру"`
	Search      *string `json:"search,omitempty" jsonschema:"description=Поиск по имени проекта"`
	Limit       *int    `json:"limit,omitempty" jsonschema:"description=Лимит (1–100; по умолчанию 20)"`
	Offset      *int    `json:"offset,omitempty" jsonschema:"description=Смещение"`
	OrderBy     string  `json:"order_by,omitempty" jsonschema:"description=Поле сортировки"`
	OrderDir    string  `json:"order_dir,omitempty" jsonschema:"description=Направление сортировки"`
}

// ProjectGetParams — параметры project_get.
type ProjectGetParams struct {
	ProjectID string `json:"project_id" jsonschema:"description=UUID проекта,required"`
}

// ProjectCreateParams — параметры project_create (как POST /projects без продвинутых полей).
type ProjectCreateParams struct {
	Name             string  `json:"name" jsonschema:"required,description=Название проекта"`
	Description      *string `json:"description,omitempty" jsonschema:"description=Описание проекта"`
	GitProvider      *string `json:"git_provider,omitempty" jsonschema:"description=Git-провайдер (local, github, gitlab, bitbucket)"`
	GitURL           *string `json:"git_url,omitempty" jsonschema:"description=URL git-репозитория"`
	GitDefaultBranch *string `json:"git_default_branch,omitempty" jsonschema:"description=Ветка по умолчанию (default: main)"`
}

// RegisterProjectTools регистрирует MCP-инструменты для проектов (список, карточка, создание).
func RegisterProjectTools(server *mcp.Server, projectSvc service.ProjectService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_list",
		Description: "Список проектов текущего пользователя (для admin — все). Пагинация и фильтры как в API GET /projects.",
	}, makeProjectListHandler(projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_get",
		Description: "Получить проект по UUID (с учётом прав доступа, как GET /projects/:id).",
	}, makeProjectGetHandler(projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_create",
		Description: "Создать новый проект. Автоматически создаёт команду с агентами по умолчанию. Как POST /projects.",
	}, makeProjectCreateHandler(projectSvc))
}

func normalizeProjectListPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func makeProjectListHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *ProjectListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ProjectListParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}

		listReq := dto.ListProjectsRequest{}
		if params != nil {
			listReq.Status = params.Status
			listReq.GitProvider = params.GitProvider
			listReq.Search = params.Search
			if params.Limit != nil {
				listReq.Limit = *params.Limit
			}
			if params.Offset != nil {
				listReq.Offset = *params.Offset
			}
			listReq.OrderBy = params.OrderBy
			listReq.OrderDir = params.OrderDir
		}

		listReq.Limit, listReq.Offset = normalizeProjectListPagination(listReq.Limit, listReq.Offset)

		projects, total, err := projectSvc.List(ctx, uid, role, listReq)
		if err != nil {
			return projectServiceMCPError(err)
		}

		limit, offset := listReq.Limit, listReq.Offset

		data := dto.ToProjectListResponse(projects, total, limit, offset)
		return OK(fmt.Sprintf("found %d projects (total %d, limit %d, offset %d)", len(data.Projects), total, limit, offset), data)
	}
}

func makeProjectGetHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *ProjectGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ProjectGetParams) (*mcp.CallToolResult, any, error) {
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

		project, err := projectSvc.GetByID(ctx, uid, role, pid)
		if err != nil {
			return projectServiceMCPError(err)
		}

		data := dto.ToProjectResponse(project)
		return OK(fmt.Sprintf("project %q (%s)", data.Name, data.ID), data)
	}
}

func makeProjectCreateHandler(projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *ProjectCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ProjectCreateParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.Name == "" {
			return ValidationErr("name is required")
		}

		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}

		createReq := dto.CreateProjectRequest{
			Name: params.Name,
		}
		if params.Description != nil {
			createReq.Description = *params.Description
		}
		if params.GitProvider != nil {
			createReq.GitProvider = *params.GitProvider
		}
		if params.GitURL != nil {
			createReq.GitURL = *params.GitURL
		}
		if params.GitDefaultBranch != nil {
			createReq.GitDefaultBranch = *params.GitDefaultBranch
		}

		project, err := projectSvc.Create(ctx, uid, createReq)
		if err != nil {
			return projectServiceMCPError(err)
		}

		data := dto.ToProjectResponse(project)
		return OK(fmt.Sprintf("project %q created (id: %s)", data.Name, data.ID), data)
	}
}

func projectServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		return Err("project not found", err)
	case errors.Is(err, service.ErrProjectForbidden):
		return Err("access to project denied", err)
	case errors.Is(err, service.ErrProjectNameExists):
		return Err("project name already exists", err)
	case errors.Is(err, service.ErrGitCredentialNotFound):
		return Err("git credential not found", err)
	case errors.Is(err, service.ErrGitCredentialForbidden):
		return Err("git credential access denied", err)
	case errors.Is(err, service.ErrGitValidationFailed):
		return Err("Git repository validation failed", err)
	case errors.Is(err, service.ErrGitCloneFailed):
		return Err("Git clone failed", err)
	case errors.Is(err, service.ErrDecryptionFailed):
		return Err("Failed to process git credentials", err)
	case errors.Is(err, service.ErrGitURLRequired),
		errors.Is(err, service.ErrGitCredentialRequired),
		errors.Is(err, service.ErrGitCredentialNotSupportedForLocal):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrProjectInvalidName),
		errors.Is(err, service.ErrProjectInvalidProvider),
		errors.Is(err, service.ErrProjectInvalidStatus),
		errors.Is(err, service.ErrUpdateProjectGitCredentialConflict),
		errors.Is(err, service.ErrUpdateProjectTechStackConflict),
		errors.Is(err, service.ErrUpdateProjectSettingsConflict):
		return ValidationErr(err.Error())
	default:
		return Err("project operation failed", err)
	}
}
