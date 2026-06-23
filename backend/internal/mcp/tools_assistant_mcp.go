package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/datatypes"
)

// AssistantMCPListParams — параметры assistant_mcp_list.
type AssistantMCPListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
}

// AssistantMCPCreateParams — параметры assistant_mcp_create.
type AssistantMCPCreateParams struct {
	ProjectID           string            `json:"project_id" jsonschema:"UUID проекта"`
	Name                string            `json:"name" jsonschema:"Имя сервера (уникально в пределах проекта)"`
	Transport           string            `json:"transport" jsonschema:"Транспорт: http | sse (remote-only)"`
	URL                 string            `json:"url" jsonschema:"URL удалённого MCP-сервера"`
	Headers             map[string]string `json:"headers,omitempty" jsonschema:"HTTP-заголовки; значения могут содержать секрет-ссылки на переменные проекта"`
	RequireConfirmation *bool             `json:"require_confirmation,omitempty" jsonschema:"Спрашивать подтверждение перед вызовом инструмента; по умолчанию true"`
	IsEnabled           *bool             `json:"is_enabled,omitempty" jsonschema:"Включён ли сервер; по умолчанию true"`
}

// AssistantMCPDeleteParams — параметры assistant_mcp_delete.
type AssistantMCPDeleteParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	ServerID  string `json:"server_id" jsonschema:"UUID MCP-сервера"`
}

// RegisterAssistantMCPTools регистрирует MCP-инструменты управления MCP-серверами
// ассистента проекта. Доступ к проекту проверяется через projectSvc (как в REST-слое),
// т.к. storage-сервис access-control не делает.
func RegisterAssistantMCPTools(server *mcp.Server, svc service.AssistantMCPServerService, projectSvc service.ProjectService) {
	if svc == nil || projectSvc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "assistant_mcp_list",
		Description: "Список внешних MCP-серверов ассистента проекта (remote http/sse). Как GET /projects/:id/assistant/mcp-servers.",
	}, makeAssistantMCPListHandler(svc, projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assistant_mcp_create",
		Description: "Добавить внешний MCP-сервер ассистента проекта (remote http/sse). Как POST /projects/:id/assistant/mcp-servers.",
	}, makeAssistantMCPCreateHandler(svc, projectSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assistant_mcp_delete",
		Description: "Удалить MCP-сервер ассистента проекта. Как DELETE /projects/:id/assistant/mcp-servers/:serverId.",
	}, makeAssistantMCPDeleteHandler(svc, projectSvc))
}

func assistantMCPToolError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrProjectForbidden),
		errors.Is(err, repository.ErrAssistantMCPServerNotFound),
		errors.Is(err, service.ErrAssistantMCPInvalidName),
		errors.Is(err, service.ErrAssistantMCPInvalidTransport),
		errors.Is(err, service.ErrAssistantMCPInvalidURL),
		errors.Is(err, service.ErrAssistantMCPInvalidHeaders):
		return ValidationErr(err.Error())
	default:
		return Err("assistant mcp operation failed", err)
	}
}

// authProjectForMCP достаёт uid/role из контекста и проверяет доступ к проекту.
func authProjectForMCP(ctx context.Context, projectSvc service.ProjectService, projectIDRaw string) (uuid.UUID, *mcp.CallToolResult, any, error, bool) {
	if projectIDRaw == "" {
		r, o, e := ValidationErr("project_id is required")
		return uuid.Nil, r, o, e, false
	}
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		r, o, e := ValidationErr("authentication required")
		return uuid.Nil, r, o, e, false
	}
	role, ok := UserRoleFromContext(ctx)
	if !ok {
		r, o, e := ValidationErr("authentication required")
		return uuid.Nil, r, o, e, false
	}
	projectID, err := uuid.Parse(projectIDRaw)
	if err != nil {
		r, o, e := ValidationErr(fmt.Sprintf("invalid project_id: %q", projectIDRaw))
		return uuid.Nil, r, o, e, false
	}
	if _, err := projectSvc.GetByID(ctx, uid, role, projectID); err != nil {
		r, o, e := assistantMCPToolError(err)
		return uuid.Nil, r, o, e, false
	}
	return projectID, nil, nil, nil, true
}

func makeAssistantMCPListHandler(svc service.AssistantMCPServerService, projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *AssistantMCPListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AssistantMCPListParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("project_id is required")
		}
		projectID, r, o, e, ok := authProjectForMCP(ctx, projectSvc, params.ProjectID)
		if !ok {
			return r, o, e
		}
		items, err := svc.List(ctx, projectID)
		if err != nil {
			return assistantMCPToolError(err)
		}
		data := dto.ToAssistantMCPServerListResponse(items)
		return OK(fmt.Sprintf("found %d mcp servers", len(data.Servers)), data)
	}
}

func makeAssistantMCPCreateHandler(svc service.AssistantMCPServerService, projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *AssistantMCPCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AssistantMCPCreateParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("project_id, name, transport, url are required")
		}
		projectID, r, o, e, ok := authProjectForMCP(ctx, projectSvc, params.ProjectID)
		if !ok {
			return r, o, e
		}
		headers := datatypes.JSON([]byte("{}"))
		if len(params.Headers) > 0 {
			if b, mErr := json.Marshal(params.Headers); mErr == nil {
				headers = datatypes.JSON(b)
			}
		}
		// Дефолты как в REST: require_confirmation и is_enabled → true при отсутствии.
		requireConfirm := params.RequireConfirmation == nil || *params.RequireConfirmation
		isEnabled := params.IsEnabled == nil || *params.IsEnabled

		cfg := &models.AssistantMCPServer{
			ProjectID:           projectID,
			Name:                params.Name,
			Transport:           models.MCPTransport(params.Transport),
			URL:                 params.URL,
			Headers:             headers,
			RequireConfirmation: requireConfirm,
			IsEnabled:           isEnabled,
		}
		if err := svc.Create(ctx, cfg); err != nil {
			return assistantMCPToolError(err)
		}
		return OK("mcp server created", dto.ToAssistantMCPServerResponse(cfg))
	}
}

func makeAssistantMCPDeleteHandler(svc service.AssistantMCPServerService, projectSvc service.ProjectService) func(ctx context.Context, req *mcp.CallToolRequest, params *AssistantMCPDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AssistantMCPDeleteParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ServerID == "" {
			return ValidationErr("project_id and server_id are required")
		}
		projectID, r, o, e, ok := authProjectForMCP(ctx, projectSvc, params.ProjectID)
		if !ok {
			return r, o, e
		}
		serverID, err := uuid.Parse(params.ServerID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid server_id: %q", params.ServerID))
		}
		// Кросс-проектный доступ по server_id запрещён: сервер должен принадлежать проекту.
		existing, err := svc.Get(ctx, serverID)
		if err != nil {
			return assistantMCPToolError(err)
		}
		if existing == nil || existing.ProjectID != projectID {
			return ValidationErr("mcp server not found in this project")
		}
		if err := svc.Delete(ctx, serverID); err != nil {
			return assistantMCPToolError(err)
		}
		return OK("mcp server deleted", map[string]any{"id": serverID.String()})
	}
}
