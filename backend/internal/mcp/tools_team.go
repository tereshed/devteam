package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "team_agent_patch",
		Description: "Частично обновить агента команды (PATCH /projects/:id/team/agents/:agentId). Поля: clear_model / model, clear_prompt_id / prompt_id, clear_code_backend / code_backend, is_active, tool_definition_ids (опционально: полная замена привязок; [] — снять все).",
	}, makeTeamAgentPatchHandler(projectSvc, teamSvc))
}

// TeamAgentPatchParams — параметры team_agent_patch (типизированные поля вместо сырого JSON).
type TeamAgentPatchParams struct {
	ProjectID string `json:"project_id" jsonschema:"description=UUID проекта,required"`
	AgentID   string `json:"agent_id" jsonschema:"description=UUID агента,required"`

	ClearModel       bool    `json:"clear_model" jsonschema:"description=Сбросить model в NULL (не совмещать с model)"`
	Model            *string `json:"model" jsonschema:"description=Новое значение model"`
	ClearPromptID    bool    `json:"clear_prompt_id" jsonschema:"description=Сбросить prompt_id в NULL"`
	PromptID         *string `json:"prompt_id" jsonschema:"description=UUID промпта"`
	ClearCodeBackend bool    `json:"clear_code_backend" jsonschema:"description=Сбросить code_backend в NULL"`
	CodeBackend      *string `json:"code_backend" jsonschema:"description=Значение code_backend"`
	IsActive         *bool   `json:"is_active" jsonschema:"description=Активен ли агент"`

	// ToolDefinitionIDs — nil: не менять tool_bindings; указатель на пустой срез: снять все;
	// непустой срез UUID — полная замена набора (объекты {tool_definition_id} в JSON PATCH).
	ToolDefinitionIDs *[]string `json:"tool_definition_ids,omitempty" jsonschema:"description=Опционально: UUID инструментов для полной замены привязок; [] снимает все; отсутствие ключа — не менять"`
}

func teamAgentPatchWireJSON(p *TeamAgentPatchParams) ([]byte, error) {
	if p.ClearModel && p.Model != nil {
		return nil, fmt.Errorf("clear_model and model are mutually exclusive")
	}
	if p.ClearPromptID && p.PromptID != nil && *p.PromptID != "" {
		return nil, fmt.Errorf("clear_prompt_id and prompt_id are mutually exclusive")
	}
	if p.ClearCodeBackend && p.CodeBackend != nil && *p.CodeBackend != "" {
		return nil, fmt.Errorf("clear_code_backend and code_backend are mutually exclusive")
	}
	if p.PromptID != nil && *p.PromptID == "" {
		return nil, fmt.Errorf("prompt_id is empty; use clear_prompt_id to reset")
	}
	m := make(map[string]any)
	if p.ClearModel {
		m["model"] = nil
	} else if p.Model != nil {
		if strings.TrimSpace(*p.Model) == "" {
			return nil, fmt.Errorf("model is empty; use clear_model to reset")
		}
		m["model"] = *p.Model
	}
	if p.ClearPromptID {
		m["prompt_id"] = nil
	} else if p.PromptID != nil {
		m["prompt_id"] = *p.PromptID
	}
	if p.ClearCodeBackend {
		m["code_backend"] = nil
	} else if p.CodeBackend != nil {
		if strings.TrimSpace(*p.CodeBackend) == "" {
			return nil, fmt.Errorf("code_backend is empty; use clear_code_backend to reset")
		}
		m["code_backend"] = *p.CodeBackend
	}
	if p.IsActive != nil {
		m["is_active"] = *p.IsActive
	}
	if p.ToolDefinitionIDs != nil {
		arr := make([]map[string]string, 0, len(*p.ToolDefinitionIDs))
		for _, raw := range *p.ToolDefinitionIDs {
			s := strings.TrimSpace(raw)
			if s == "" {
				return nil, fmt.Errorf("tool_definition_id is empty")
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return nil, fmt.Errorf("invalid tool_definition_id %q: %w", s, err)
			}
			arr = append(arr, map[string]string{"tool_definition_id": id.String()})
		}
		m["tool_bindings"] = arr
	}
	return json.Marshal(m)
}

func makeTeamAgentPatchHandler(projectSvc service.ProjectService, teamSvc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *TeamAgentPatchParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TeamAgentPatchParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" || params.AgentID == "" {
			return ValidationErr("project_id and agent_id are required")
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
		aid, err := uuid.Parse(params.AgentID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid agent_id: %q", params.AgentID))
		}

		if _, err := projectSvc.GetByID(ctx, uid, role, pid); err != nil {
			return projectServiceMCPError(err)
		}

		raw, err := teamAgentPatchWireJSON(params)
		if err != nil {
			return ValidationErr(err.Error())
		}

		var patch dto.PatchAgentRequest
		if err := json.Unmarshal(raw, &patch); err != nil {
			return ValidationErr(fmt.Sprintf("invalid patch fields: %v", err))
		}

		team, err := teamSvc.PatchAgent(ctx, pid, aid, patch)
		if err != nil {
			return teamServiceMCPError(err)
		}

		data := dto.ToTeamResponse(team)
		return OK("agent patched", data)
	}
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
	case errors.Is(err, service.ErrTeamAgentNotFound):
		return Err("agent not found", err)
	case errors.Is(err, service.ErrTeamAgentInvalidModel),
		errors.Is(err, service.ErrTeamAgentInvalidCodeBackend),
		errors.Is(err, service.ErrTeamAgentInvalidToolBindings):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrTeamAgentConflict):
		return Err("agent update conflict", err)
	default:
		return Err("team operation failed", err)
	}
}
