package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EnhancerConfigGetParams — параметры enhancer_config_get / enhancer_run_now / enhancer_run_list.
type EnhancerConfigGetParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
}

// EnhancerConfigUpdateParams — параметры enhancer_config_update.
type EnhancerConfigUpdateParams struct {
	ProjectID          string  `json:"project_id" jsonschema:"UUID проекта"`
	IsActive           *bool   `json:"is_active,omitempty" jsonschema:"Включить/выключить энхансер проекта"`
	CronExpression     *string `json:"cron_expression,omitempty" jsonschema:"5-польное cron-выражение автозапуска; пустая строка убирает расписание"`
	AnalysisWindowDays *int    `json:"analysis_window_days,omitempty" jsonschema:"Окно анализа истории задач в днях (1-90)"`
	MaxChangesPerRun   *int    `json:"max_changes_per_run,omitempty" jsonschema:"Лимит предложений за прогон (1-20)"`
}

// EnhancerChangeListParams — параметры enhancer_change_list.
type EnhancerChangeListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RunID     string `json:"run_id" jsonschema:"UUID прогона энхансера"`
}

// EnhancerChangeActionParams — параметры enhancer_change_apply/reject/rollback.
type EnhancerChangeActionParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	ChangeID  string `json:"change_id" jsonschema:"UUID предложения изменения"`
}

// RegisterEnhancerTools регистрирует MCP-инструменты энхансера проекта.
func RegisterEnhancerTools(server *mcp.Server, svc service.EnhancerService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_config_get",
		Description: "Конфиг энхансера проекта (мета-агент улучшения работы агентов). Как GET /projects/:id/enhancer.",
	}, makeEnhancerConfigGetHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_config_update",
		Description: "Обновить конфиг энхансера проекта (тумблер, cron, окно анализа, лимит предложений). Как PUT /projects/:id/enhancer.",
	}, makeEnhancerConfigUpdateHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_run_now",
		Description: "Запустить прогон энхансера немедленно. Как POST /projects/:id/enhancer/run.",
	}, makeEnhancerRunNowHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_run_list",
		Description: "Последние прогоны энхансера проекта с отчётами. Как GET /projects/:id/enhancer/runs.",
	}, makeEnhancerRunListHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_change_list",
		Description: "Предложения изменений одного прогона энхансера. Как GET /projects/:id/enhancer/runs/:runId/changes.",
	}, makeEnhancerChangeListHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_change_apply",
		Description: "Применить proposed-предложение энхансера (оверрайд промпта агента / правка описания или настроек проекта). Как POST /projects/:id/enhancer/changes/:changeId/apply.",
	}, makeEnhancerChangeActionHandler(svc.ApplyChange))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_change_reject",
		Description: "Отклонить proposed-предложение энхансера. Как POST /projects/:id/enhancer/changes/:changeId/reject.",
	}, makeEnhancerChangeActionHandler(svc.RejectChange))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enhancer_change_rollback",
		Description: "Откатить применённое предложение энхансера. Как POST /projects/:id/enhancer/changes/:changeId/rollback.",
	}, makeEnhancerChangeActionHandler(svc.RollbackChange))
}

func enhancerMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrEnhancerRunNotFound),
		errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrProjectForbidden),
		errors.Is(err, service.ErrEnhancerInvalidCron),
		errors.Is(err, service.ErrEnhancerInvalidAutonomy),
		errors.Is(err, service.ErrEnhancerInvalidWindow),
		errors.Is(err, service.ErrEnhancerInvalidLimit),
		errors.Is(err, service.ErrEnhancerRunInProgress),
		errors.Is(err, service.ErrEnhancerChangeNotFound),
		errors.Is(err, service.ErrEnhancerChangeBadState),
		errors.Is(err, service.ErrEnhancerChangeConflict),
		errors.Is(err, service.ErrEnhancerChangeInvalidPayload):
		return ValidationErr(err.Error())
	default:
		return Err("enhancer operation failed", err)
	}
}

// makeEnhancerChangeActionHandler — общий хендлер apply/reject/rollback.
func makeEnhancerChangeActionHandler(
	action func(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, changeID uuid.UUID) (*models.EnhancerChange, error),
) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerChangeActionParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerChangeActionParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.ChangeID == "" {
			return ValidationErr("change_id is required")
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
		changeID, err := uuid.Parse(params.ChangeID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid change_id: %q", params.ChangeID))
		}
		ch, err := action(ctx, uid, role, projectID, changeID)
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerChangeResponse(ch)
		return OK(fmt.Sprintf("change %s -> %s", data.ID, data.Status), data)
	}
}

func makeEnhancerConfigGetHandler(svc service.EnhancerService) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
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
		cfg, err := svc.GetConfig(ctx, uid, role, projectID)
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerConfigResponse(cfg)
		return OK("enhancer config", data)
	}
}

func makeEnhancerConfigUpdateHandler(svc service.EnhancerService) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerConfigUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerConfigUpdateParams) (*mcp.CallToolResult, any, error) {
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
		cfg, err := svc.UpdateConfig(ctx, uid, role, projectID, dto.UpdateEnhancerConfigRequest{
			IsActive:           params.IsActive,
			CronExpression:     params.CronExpression,
			AnalysisWindowDays: params.AnalysisWindowDays,
			MaxChangesPerRun:   params.MaxChangesPerRun,
		})
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerConfigResponse(cfg)
		return OK("enhancer config updated", data)
	}
}

func makeEnhancerRunNowHandler(svc service.EnhancerService) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
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
		run, err := svc.RunNow(ctx, uid, role, projectID)
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerRunResponse(run)
		return OK(fmt.Sprintf("enhancer run %s started", data.ID), data)
	}
}

func makeEnhancerRunListHandler(svc service.EnhancerService) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerConfigGetParams) (*mcp.CallToolResult, any, error) {
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
		runs, err := svc.ListRuns(ctx, uid, role, projectID)
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerRunListResponse(runs)
		return OK(fmt.Sprintf("found %d enhancer runs", data.Total), data)
	}
}

func makeEnhancerChangeListHandler(svc service.EnhancerService) func(ctx context.Context, req *mcp.CallToolRequest, params *EnhancerChangeListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *EnhancerChangeListParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.RunID == "" {
			return ValidationErr("run_id is required")
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
		runID, err := uuid.Parse(params.RunID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid run_id: %q", params.RunID))
		}
		changes, err := svc.ListRunChanges(ctx, uid, role, projectID, runID)
		if err != nil {
			return enhancerMCPError(err)
		}
		data := dto.ToEnhancerChangeListResponse(changes)
		return OK(fmt.Sprintf("found %d proposed changes", data.Total), data)
	}
}
