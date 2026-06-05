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

// ScheduledTaskListParams — параметры scheduled_task_list.
type ScheduledTaskListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
}

// ScheduledTaskCreateParams — параметры scheduled_task_create.
type ScheduledTaskCreateParams struct {
	ProjectID      string  `json:"project_id" jsonschema:"UUID проекта"`
	Name           string  `json:"name" jsonschema:"Имя расписания; оно же становится Title создаваемых задач (1–500 символов)"`
	CronExpression string  `json:"cron_expression" jsonschema:"Стандартное 5-польное cron-выражение (например '0 9 * * 1-5')"`
	Description    *string `json:"description,omitempty" jsonschema:"Описание задачи, которое получит каждая создаваемая task'а"`
	Priority       *string `json:"priority,omitempty" jsonschema:"Приоритет создаваемых задач (critical, high, medium, low). По умолчанию medium."`
	TeamID         *string `json:"team_id,omitempty" jsonschema:"UUID команды проекта (опционально)"`
	IsActive       *bool   `json:"is_active,omitempty" jsonschema:"Активно ли расписание сразу (по умолчанию true)"`
}

// ScheduledTaskDeleteParams — параметры scheduled_task_delete.
type ScheduledTaskDeleteParams struct {
	ProjectID       string `json:"project_id" jsonschema:"UUID проекта"`
	ScheduledTaskID string `json:"scheduled_task_id" jsonschema:"UUID расписания"`
}

// RegisterScheduledTaskTools регистрирует MCP-инструменты для регулярных задач.
func RegisterScheduledTaskTools(server *mcp.Server, svc service.ScheduledTaskService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scheduled_task_list",
		Description: "Список регулярных (cron) задач проекта. Как GET /projects/:id/scheduled-tasks.",
	}, makeScheduledTaskListHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "scheduled_task_create",
		Description: "Создать расписание, по которому в проекте будут периодически создаваться задачи. Как POST /projects/:id/scheduled-tasks.",
	}, makeScheduledTaskCreateHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "scheduled_task_delete",
		Description: "Удалить расписание проекта. Уже созданные задачи не затрагиваются. Как DELETE /projects/:id/scheduled-tasks/:scheduleId.",
	}, makeScheduledTaskDeleteHandler(svc))
}

func scheduledTaskMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrScheduledTaskNotFound), errors.Is(err, service.ErrProjectNotFound):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrScheduledTaskInvalidName),
		errors.Is(err, service.ErrScheduledTaskInvalidCron),
		errors.Is(err, service.ErrTaskInvalidPriority),
		errors.Is(err, service.ErrTeamNotInProject):
		return ValidationErr(err.Error())
	default:
		return Err("scheduled task operation failed", err)
	}
}

func makeScheduledTaskListHandler(svc service.ScheduledTaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *ScheduledTaskListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ScheduledTaskListParams) (*mcp.CallToolResult, any, error) {
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
			return scheduledTaskMCPError(err)
		}
		data := dto.ToScheduledTaskListResponse(items)
		return OK(fmt.Sprintf("found %d scheduled tasks", data.Total), data)
	}
}

func makeScheduledTaskCreateHandler(svc service.ScheduledTaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *ScheduledTaskCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ScheduledTaskCreateParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.Name == "" {
			return ValidationErr("name is required")
		}
		if params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.CronExpression == "" {
			return ValidationErr("cron_expression is required")
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
		createReq := dto.CreateScheduledTaskRequest{
			Name:           params.Name,
			CronExpression: params.CronExpression,
			IsActive:       params.IsActive,
		}
		if params.Description != nil {
			createReq.Description = *params.Description
		}
		if params.Priority != nil {
			createReq.Priority = *params.Priority
		}
		if params.TeamID != nil {
			teamID, err := uuid.Parse(*params.TeamID)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid team_id: %q", *params.TeamID))
			}
			createReq.TeamID = &teamID
		}
		st, err := svc.Create(ctx, uid, role, projectID, createReq)
		if err != nil {
			return scheduledTaskMCPError(err)
		}
		data := dto.ToScheduledTaskResponse(st)
		return OK(fmt.Sprintf("scheduled task %q created (id: %s)", data.Name, data.ID), data)
	}
}

func makeScheduledTaskDeleteHandler(svc service.ScheduledTaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *ScheduledTaskDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ScheduledTaskDeleteParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		if params.ScheduledTaskID == "" {
			return ValidationErr("scheduled_task_id is required")
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
		scheduleID, err := uuid.Parse(params.ScheduledTaskID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid scheduled_task_id: %q", params.ScheduledTaskID))
		}
		if err := svc.Delete(ctx, uid, role, projectID, scheduleID); err != nil {
			return scheduledTaskMCPError(err)
		}
		return OK(fmt.Sprintf("scheduled task %s deleted", scheduleID), map[string]string{"id": scheduleID.String()})
	}
}
