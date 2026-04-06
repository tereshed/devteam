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

// TaskListParams — параметры task_list (как GET /projects/:id/tasks).
type TaskListParams struct {
	ProjectID       string   `json:"project_id" jsonschema:"required,description=UUID проекта"`
	Status          *string  `json:"status,omitempty" jsonschema:"description=Фильтр по статусу (pending, planning, in_progress, review, changes_requested, testing, paused, completed, failed, cancelled)"`
	Statuses        []string `json:"statuses,omitempty" jsonschema:"description=Фильтр по нескольким статусам"`
	Priority        *string  `json:"priority,omitempty" jsonschema:"description=Фильтр по приоритету (critical, high, medium, low)"`
	AssignedAgentID *string  `json:"assigned_agent_id,omitempty" jsonschema:"description=UUID агента"`
	RootOnly        bool     `json:"root_only,omitempty" jsonschema:"description=Только корневые задачи (без подзадач)"`
	Search          *string  `json:"search,omitempty" jsonschema:"description=Поиск по title/description"`
	Limit           *int     `json:"limit,omitempty" jsonschema:"description=Лимит (1–200; по умолчанию 50)"`
	Offset          *int     `json:"offset,omitempty" jsonschema:"description=Смещение"`
	OrderBy         string   `json:"order_by,omitempty" jsonschema:"description=Поле сортировки (created_at, updated_at, priority, status)"`
	OrderDir        string   `json:"order_dir,omitempty" jsonschema:"description=Направление (asc, desc)"`
}

// TaskGetParams — параметры task_get.
type TaskGetParams struct {
	TaskID string `json:"task_id" jsonschema:"required,description=UUID задачи"`
}

// TaskCreateParams — параметры task_create.
type TaskCreateParams struct {
	ProjectID       string  `json:"project_id" jsonschema:"required,description=UUID проекта"`
	Title           string  `json:"title" jsonschema:"required,description=Название задачи (1–500 символов)"`
	Description     *string `json:"description,omitempty" jsonschema:"description=Подробное описание задачи"`
	Priority        *string `json:"priority,omitempty" jsonschema:"description=Приоритет (critical, high, medium, low). По умолчанию medium."`
	AssignedAgentID *string `json:"assigned_agent_id,omitempty" jsonschema:"description=UUID агента для назначения (должен быть в команде проекта)"`
	ParentTaskID    *string `json:"parent_task_id,omitempty" jsonschema:"description=UUID родительской задачи (для создания подзадачи)"`
}

// TaskUpdateParams — параметры task_update.
type TaskUpdateParams struct {
	TaskID             string  `json:"task_id" jsonschema:"required,description=UUID задачи"`
	Title              *string `json:"title,omitempty" jsonschema:"description=Новое название"`
	Description        *string `json:"description,omitempty" jsonschema:"description=Новое описание"`
	Priority           *string `json:"priority,omitempty" jsonschema:"description=Новый приоритет (critical, high, medium, low)"`
	Status             *string `json:"status,omitempty" jsonschema:"description=Новый статус (допустимые переходы проверяются state machine)"`
	AssignedAgentID    *string `json:"assigned_agent_id,omitempty" jsonschema:"description=UUID агента для назначения"`
	ClearAssignedAgent bool    `json:"clear_assigned_agent,omitempty" jsonschema:"description=Снять назначенного агента"`
	BranchName         *string `json:"branch_name,omitempty" jsonschema:"description=Git-ветка задачи"`
}

// RegisterTaskTools регистрирует MCP-инструменты для задач.
func RegisterTaskTools(server *mcp.Server, taskSvc service.TaskService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "task_list",
		Description: "Список задач проекта с фильтрацией и пагинацией. Как GET /projects/:id/tasks.",
	}, makeTaskListHandler(taskSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "task_get",
		Description: "Получить задачу по UUID с агентом и подзадачами. Как GET /tasks/:id.",
	}, makeTaskGetHandler(taskSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "task_create",
		Description: "Создать задачу в проекте. Статус будет pending. Как POST /projects/:id/tasks.",
	}, makeTaskCreateHandler(taskSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "task_update",
		Description: "Обновить задачу (title, description, priority, status, агент, ветка). Как PUT /tasks/:id.",
	}, makeTaskUpdateHandler(taskSvc))
}

func normalizeTaskMCPPagination(limitPtr, offsetPtr *int) (int, int) {
	limit := 50
	if limitPtr != nil && *limitPtr > 0 {
		limit = *limitPtr
	}
	if limit > 200 {
		limit = 200
	}
	offset := 0
	if offsetPtr != nil && *offsetPtr > 0 {
		offset = *offsetPtr
	}
	return limit, offset
}

func makeTaskListHandler(taskSvc service.TaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *TaskListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TaskListParams) (*mcp.CallToolResult, any, error) {
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

		listReq := dto.ListTasksRequest{
			Status:   params.Status,
			Statuses: params.Statuses,
			Priority: params.Priority,
			RootOnly: params.RootOnly,
			Search:   params.Search,
			OrderBy:  params.OrderBy,
			OrderDir: params.OrderDir,
		}
		if params.AssignedAgentID != nil {
			agentID, err := uuid.Parse(*params.AssignedAgentID)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid assigned_agent_id: %q", *params.AssignedAgentID))
			}
			listReq.AssignedAgentID = &agentID
		}

		listReq.Limit, listReq.Offset = normalizeTaskMCPPagination(params.Limit, params.Offset)

		tasks, total, err := taskSvc.List(ctx, uid, role, projectID, listReq)
		if err != nil {
			return taskServiceMCPError(err)
		}

		data := dto.ToTaskListResponse(tasks, total, listReq.Limit, listReq.Offset)
		return OK(fmt.Sprintf("found %d tasks (total %d, limit %d, offset %d)", len(data.Tasks), total, listReq.Limit, listReq.Offset), data)
	}
}

func makeTaskGetHandler(taskSvc service.TaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *TaskGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TaskGetParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.TaskID == "" {
			return ValidationErr("task_id is required")
		}

		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}

		taskID, err := uuid.Parse(params.TaskID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid task_id: %q", params.TaskID))
		}

		task, err := taskSvc.GetByID(ctx, uid, role, taskID)
		if err != nil {
			return taskServiceMCPError(err)
		}

		data := dto.ToTaskResponse(task)
		return OK(fmt.Sprintf("task %q (%s) [%s]", data.Title, data.ID, data.Status), data)
	}
}

func makeTaskCreateHandler(taskSvc service.TaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *TaskCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TaskCreateParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.Title == "" {
			return ValidationErr("title is required")
		}
		if params.ProjectID == "" {
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

		createReq := dto.CreateTaskRequest{
			Title: params.Title,
		}
		if params.Description != nil {
			createReq.Description = *params.Description
		}
		if params.Priority != nil {
			createReq.Priority = *params.Priority
		}
		if params.AssignedAgentID != nil {
			agentID, err := uuid.Parse(*params.AssignedAgentID)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid assigned_agent_id: %q", *params.AssignedAgentID))
			}
			createReq.AssignedAgentID = &agentID
		}
		if params.ParentTaskID != nil {
			parentID, err := uuid.Parse(*params.ParentTaskID)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid parent_task_id: %q", *params.ParentTaskID))
			}
			createReq.ParentTaskID = &parentID
		}

		task, err := taskSvc.Create(ctx, uid, role, projectID, createReq)
		if err != nil {
			return taskServiceMCPError(err)
		}

		data := dto.ToTaskResponse(task)
		return OK(fmt.Sprintf("task %q created (id: %s)", data.Title, data.ID), data)
	}
}

func makeTaskUpdateHandler(taskSvc service.TaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *TaskUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *TaskUpdateParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.TaskID == "" {
			return ValidationErr("task_id is required")
		}

		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		role, ok := UserRoleFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}

		taskID, err := uuid.Parse(params.TaskID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid task_id: %q", params.TaskID))
		}

		updateReq := dto.UpdateTaskRequest{
			Title:              params.Title,
			Description:        params.Description,
			Priority:           params.Priority,
			Status:             params.Status,
			ClearAssignedAgent: params.ClearAssignedAgent,
			BranchName:         params.BranchName,
		}
		if params.AssignedAgentID != nil {
			agentID, err := uuid.Parse(*params.AssignedAgentID)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid assigned_agent_id: %q", *params.AssignedAgentID))
			}
			updateReq.AssignedAgentID = &agentID
		}

		task, err := taskSvc.Update(ctx, uid, role, taskID, updateReq)
		if err != nil {
			return taskServiceMCPError(err)
		}

		data := dto.ToTaskResponse(task)
		return OK(fmt.Sprintf("task %q updated (status: %s)", data.Title, data.Status), data)
	}
}

func taskServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrTaskNotFound):
		return Err("task not found", err)
	case errors.Is(err, service.ErrProjectNotFound):
		return Err("project not found", err)
	case errors.Is(err, service.ErrProjectForbidden):
		return Err("access to project denied", err)
	case errors.Is(err, service.ErrTaskInvalidTransition):
		return Err("invalid status transition", err)
	case errors.Is(err, service.ErrTaskTerminalStatus):
		return Err("task is in terminal status", err)
	case errors.Is(err, service.ErrTaskConcurrentUpdate):
		return Err("task was modified concurrently, please retry", err)
	case errors.Is(err, service.ErrAgentNotInTeam):
		return Err("agent does not belong to project team", err)
	case errors.Is(err, service.ErrTaskParentNotFound):
		return Err("parent task not found", err)
	case errors.Is(err, service.ErrTaskInvalidTitle),
		errors.Is(err, service.ErrTaskInvalidPriority),
		errors.Is(err, service.ErrTaskInvalidStatus),
		errors.Is(err, service.ErrTaskMessageInvalidType):
		return ValidationErr(err.Error())
	default:
		return Err("task operation failed", err)
	}
}
