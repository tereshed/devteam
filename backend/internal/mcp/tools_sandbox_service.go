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

// SandboxServiceListParams — параметры sandbox_service_list.
type SandboxServiceListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
}

// SandboxServiceSetParams — параметры sandbox_service_set (upsert по alias).
type SandboxServiceSetParams struct {
	ProjectID           string `json:"project_id" jsonschema:"UUID проекта"`
	Alias               string `json:"alias" jsonschema:"Сетевой alias/hostname сервиса в bridge-сети прогона (например db)"`
	IsEnabled           bool   `json:"is_enabled" jsonschema:"Поднимать ли сервис для прогонов агентов с attach_sandbox_services"`
	Kind                string `json:"kind,omitempty" jsonschema:"Тип сервиса; пусто → postgres"`
	Image               string `json:"image,omitempty" jsonschema:"Docker-образ; пусто → postgres:16-alpine"`
	DBName              string `json:"db_name,omitempty" jsonschema:"Имя БД; пусто → app"`
	DBUser              string `json:"db_user,omitempty" jsonschema:"Суперюзер БД; пусто → postgres (пароль генерится на прогон, не хранится)"`
	Port                int    `json:"port,omitempty" jsonschema:"Порт сервиса; 0 → 5432"`
	SeedKind            string `json:"seed_kind,omitempty" jsonschema:"none | repo_file | inline; пусто → none"`
	SeedValue           string `json:"seed_value,omitempty" jsonschema:"Путь к .sql в репо (repo_file) или SQL (inline)"`
	ReadyTimeoutSeconds int    `json:"ready_timeout_seconds,omitempty" jsonschema:"Потолок ожидания готовности сервиса (10-600); 0 → 60"`
}

// RegisterSandboxServiceTools регистрирует MCP-инструменты деклараций сервис-сайдкаров.
func RegisterSandboxServiceTools(server *mcp.Server, svc service.SandboxServiceConfigService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_service_list",
		Description: "Список эфемерных сервис-сайдкаров проекта (postgres для интеграционных тестов с БД). Как GET /projects/:id/sandbox-services.",
	}, makeSandboxServiceListHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_service_set",
		Description: "Создать/обновить декларацию сервис-сайдкара проекта (upsert по alias). Как PUT /projects/:id/sandbox-services.",
	}, makeSandboxServiceSetHandler(svc))
}

func sandboxServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrProjectForbidden),
		errors.Is(err, service.ErrSandboxServiceNotFound),
		errors.Is(err, service.ErrSandboxServiceInvalidAlias),
		errors.Is(err, service.ErrSandboxServiceInvalidKind),
		errors.Is(err, service.ErrSandboxServiceInvalidSeedKind),
		errors.Is(err, service.ErrSandboxServiceInvalidImage),
		errors.Is(err, service.ErrSandboxServiceInvalidPort),
		errors.Is(err, service.ErrSandboxServiceInvalidTimeout),
		errors.Is(err, service.ErrSandboxServiceInvalidField),
		errors.Is(err, service.ErrSandboxServiceInvalidSeedValue):
		return ValidationErr(err.Error())
	default:
		return Err("sandbox service operation failed", err)
	}
}

func makeSandboxServiceListHandler(svc service.SandboxServiceConfigService) func(ctx context.Context, req *mcp.CallToolRequest, params *SandboxServiceListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *SandboxServiceListParams) (*mcp.CallToolResult, any, error) {
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
		projectID, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}
		items, err := svc.List(ctx, uid, role, projectID)
		if err != nil {
			return sandboxServiceMCPError(err)
		}
		data := dto.ToSandboxServiceListResponse(items)
		return OK(fmt.Sprintf("found %d sandbox services", data.Total), data)
	}
}

func makeSandboxServiceSetHandler(svc service.SandboxServiceConfigService) func(ctx context.Context, req *mcp.CallToolRequest, params *SandboxServiceSetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *SandboxServiceSetParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.Alias == "" {
			return ValidationErr("alias is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		projectID, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}
		cfg, err := svc.Upsert(ctx, uid, role, projectID, dto.UpsertSandboxServiceRequest{
			IsEnabled:           params.IsEnabled,
			Kind:                params.Kind,
			Alias:               params.Alias,
			Image:               params.Image,
			DBName:              params.DBName,
			DBUser:              params.DBUser,
			Port:                params.Port,
			SeedKind:            params.SeedKind,
			SeedValue:           params.SeedValue,
			ReadyTimeoutSeconds: params.ReadyTimeoutSeconds,
		})
		if err != nil {
			return sandboxServiceMCPError(err)
		}
		data := dto.ToSandboxServiceConfigResponse(cfg)
		return OK(fmt.Sprintf("sandbox service %q saved", data.Alias), data)
	}
}
