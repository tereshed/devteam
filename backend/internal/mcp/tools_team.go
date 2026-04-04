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

// TeamGetParams — параметры team_get.
type TeamGetParams struct {
	ProjectID string `json:"project_id" jsonschema:"description=UUID проекта,required"`
}

// TeamUpdateParams — параметры team_update.
type TeamUpdateParams struct {
	ProjectID string  `json:"project_id" jsonschema:"description=UUID проекта,required"`
	Name      *string `json:"name,omitempty" jsonschema:"description=Новое имя команды"`
}

// RegisterTeamTools регистрирует MCP-инструменты для команды проекта.
func RegisterTeamTools(server *mcp.Server, projectSvc service.ProjectService, teamSvc service.TeamService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "team_get",
		Description: "Получить команду проекта с агентами (как GET /projects/:id/team).",
	}, makeTeamGetHandler(projectSvc, teamSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "team_update",
		Description: "Обновить команду проекта (имя), как PUT /projects/:id/team.",
	}, makeTeamUpdateHandler(projectSvc, teamSvc))
}

func makeTeamGetHandler(projectSvc service.ProjectService, teamSvc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *TeamGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TeamGetParams) (*mcp.CallToolResult, any, error) {
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

		if _, err := projectSvc.GetByID(ctx, uid, role, pid); err != nil {
			return projectServiceMCPError(err)
		}

		team, err := teamSvc.GetByProjectID(ctx, pid)
		if err != nil {
			return teamServiceMCPError(err)
		}

		data := dto.ToTeamResponse(team)
		return OK(fmt.Sprintf("team %q for project %s", data.Name, data.ProjectID), data)
	}
}

func makeTeamUpdateHandler(projectSvc service.ProjectService, teamSvc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *TeamUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TeamUpdateParams) (*mcp.CallToolResult, any, error) {
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

		if _, err := projectSvc.GetByID(ctx, uid, role, pid); err != nil {
			return projectServiceMCPError(err)
		}

		upd := dto.UpdateTeamRequest{Name: params.Name}
		team, err := teamSvc.Update(ctx, pid, upd)
		if err != nil {
			return teamServiceMCPError(err)
		}

		data := dto.ToTeamResponse(team)
		return OK(fmt.Sprintf("team updated: %q", data.Name), data)
	}
}

func teamServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrTeamNotFound):
		return Err("team not found", err)
	case errors.Is(err, service.ErrTeamInvalidName):
		return ValidationErr(err.Error())
	default:
		return Err("team operation failed", err)
	}
}
