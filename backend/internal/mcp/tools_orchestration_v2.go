package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools_orchestration_v2.go — Sprint 17 / Sprint 5 — MCP-инструменты для
// observability и операций над v2-оркестрацией.
//
// Sprint 5 review fix #1: handlers зависят от service.OrchestrationQueryService
// и service.TaskLifecycleService — НЕ от repositories напрямую.

// ─────────────────────────────────────────────────────────────────────────────
// Params
// ─────────────────────────────────────────────────────────────────────────────

type ArtifactListParams struct {
	TaskID    string `json:"task_id" jsonschema:"required,description=UUID задачи"`
	OnlyReady *bool  `json:"only_ready,omitempty" jsonschema:"description=true (default) — только status=ready"`
}

type ArtifactGetParams struct {
	ArtifactID string `json:"artifact_id" jsonschema:"required,description=UUID артефакта"`
}

type RouterDecisionListParams struct {
	TaskID string `json:"task_id" jsonschema:"required,description=UUID задачи"`
}

type WorktreeListParams struct {
	TaskID string `json:"task_id" jsonschema:"required,description=UUID задачи"`
}

type TaskCancelV2Params struct {
	TaskID string `json:"task_id" jsonschema:"required,description=UUID задачи (только active можно отменить)"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// RegisterOrchestrationV2Tools — handlers зависят от services (не от repos напрямую).
// taskLifecycle опционально; nil — task_cancel_v2 не регистрируется.
func RegisterOrchestrationV2Tools(
	server *mcp.Server,
	querySvc *service.OrchestrationQueryService,
	taskLifecycle *service.TaskLifecycleService,
) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "artifact_list",
		Description: "Артефакты задачи (metadata, без content). Для timeline в UI.",
	}, makeArtifactListHandler(querySvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "artifact_get",
		Description: "Полный артефакт (включая content) по UUID. Для дебага.",
	}, makeArtifactGetHandler(querySvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "router_decision_list",
		Description: "Лог Router-решений по задаче. encrypted_raw_response НЕ отдаётся.",
	}, makeRouterDecisionListHandler(querySvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "worktree_list",
		Description: "Git worktree'и задачи: state, branch_name, allocated_at/released_at.",
	}, makeWorktreeListHandler(querySvc))

	if taskLifecycle != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "task_cancel_v2",
			Description: "Кооперативная отмена активной задачи. In-flight агенты прервутся; Step финализирует cancelled.",
		}, makeTaskCancelV2Handler(taskLifecycle))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers (тонкие)
// ─────────────────────────────────────────────────────────────────────────────

func makeArtifactListHandler(svc *service.OrchestrationQueryService) func(context.Context, *mcp.CallToolRequest, ArtifactListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p ArtifactListParams) (*mcp.CallToolResult, any, error) {
		taskID, err := uuid.Parse(p.TaskID)
		if err != nil {
			return ValidationErr("invalid task_id (must be UUID)")
		}
		onlyReady := true
		if p.OnlyReady != nil {
			onlyReady = *p.OnlyReady
		}
		arts, err := svc.ListArtifacts(ctx, taskID, onlyReady)
		if err != nil {
			return Err("failed to list artifacts", err)
		}
		return OK(fmt.Sprintf("Found %d artifacts (onlyReady=%v)", len(arts), onlyReady), map[string]any{
			"task_id": taskID,
			"items":   arts,
		})
	}
}

func makeArtifactGetHandler(svc *service.OrchestrationQueryService) func(context.Context, *mcp.CallToolRequest, ArtifactGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p ArtifactGetParams) (*mcp.CallToolResult, any, error) {
		id, err := uuid.Parse(p.ArtifactID)
		if err != nil {
			return ValidationErr("invalid artifact_id (must be UUID)")
		}
		art, err := svc.GetArtifact(ctx, id)
		if err != nil {
			if errors.Is(err, service.ErrArtifactNotInTask) {
				return Err("artifact not found", err)
			}
			return Err("failed to get artifact", err)
		}
		return OK(fmt.Sprintf("Artifact %s (kind=%s)", art.ID, art.Kind), art)
	}
}

func makeRouterDecisionListHandler(svc *service.OrchestrationQueryService) func(context.Context, *mcp.CallToolRequest, RouterDecisionListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RouterDecisionListParams) (*mcp.CallToolResult, any, error) {
		taskID, err := uuid.Parse(p.TaskID)
		if err != nil {
			return ValidationErr("invalid task_id (must be UUID)")
		}
		ds, err := svc.ListRouterDecisions(ctx, taskID)
		if err != nil {
			return Err("failed to list router decisions", err)
		}
		return OK(fmt.Sprintf("Found %d router decisions", len(ds)), map[string]any{
			"task_id": taskID,
			"items":   ds,
		})
	}
}

func makeWorktreeListHandler(svc *service.OrchestrationQueryService) func(context.Context, *mcp.CallToolRequest, WorktreeListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p WorktreeListParams) (*mcp.CallToolResult, any, error) {
		taskID, err := uuid.Parse(p.TaskID)
		if err != nil {
			return ValidationErr("invalid task_id (must be UUID)")
		}
		wts, err := svc.ListWorktrees(ctx, taskID)
		if err != nil {
			return Err("failed to list worktrees", err)
		}
		return OK(fmt.Sprintf("Found %d worktrees", len(wts)), map[string]any{
			"task_id": taskID,
			"items":   wts,
		})
	}
}

func makeTaskCancelV2Handler(svc *service.TaskLifecycleService) func(context.Context, *mcp.CallToolRequest, TaskCancelV2Params) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p TaskCancelV2Params) (*mcp.CallToolResult, any, error) {
		taskID, err := uuid.Parse(p.TaskID)
		if err != nil {
			return ValidationErr("invalid task_id (must be UUID)")
		}
		if err := svc.RequestCancel(ctx, taskID); err != nil {
			if errors.Is(err, service.ErrTaskNotCancellable) {
				return Err("task is not active (already done/failed/cancelled/needs_human)", err)
			}
			return Err("failed to request task cancel", err)
		}
		return OK("Cancel requested. In-flight agents will abort; next Step will finalize state=cancelled.", map[string]any{
			"task_id":      taskID,
			"requested_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
