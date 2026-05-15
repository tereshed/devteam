package mcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
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

// WorktreeReleaseParams — параметры для деструктивного worktree_release.
//
// Confirm обязателен и должен быть `true` — это explicit-acknowledgement,
// чтобы случайный/галлюцинирующий вызов LLM без явного намерения оператора
// не снёс git worktree с uncommitted changes агента. Audit-log на стороне
// MCP-handler'а пишет user_id, user_role И api_key_id (последний достаём из
// MCP-context'а — установлен NewAuthMiddleware), чтобы при инцидент-разборе
// можно было однозначно установить, какой именно admin-ключ инициировал вызов
// (а не только владельца ключа — у одного админа может быть несколько ключей
// для разных автоматизаций).
type WorktreeReleaseParams struct {
	WorktreeID string `json:"worktree_id" jsonschema:"required,description=UUID worktree для принудительного освобождения"`
	Confirm    bool   `json:"confirm" jsonschema:"required,description=Должно быть true. Подтверждение деструктивной операции (git worktree remove --force, потеря uncommitted changes агента)."`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// RegisterOrchestrationV2Tools — handlers зависят от services (не от repos напрямую).
//
// taskLifecycle опционально; nil — task_cancel_v2 не регистрируется.
// worktreeMgr опционально; nil — worktree_release не регистрируется (legacy clone-path).
func RegisterOrchestrationV2Tools(
	server *mcp.Server,
	querySvc *service.OrchestrationQueryService,
	taskLifecycle *service.TaskLifecycleService,
	worktreeMgr *service.WorktreeManager,
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

	// Sprint 17 / 6.3 — manual unstick. Деструктивная admin-операция: ручка
	// требует (a) admin-роль через context, (b) явный confirm:true. Без любого
	// из двух — tool отказывает до вызова WorktreeManager. Audit-log в обоих
	// слоях: WorktreeManager.ReleaseManual пишет user_id+user_role, MCP-handler
	// дополнительно пишет api_key_id перед вызовом (см. makeWorktreeReleaseHandler).
	if worktreeMgr != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "worktree_release",
			Description: "ДЕСТРУКТИВНО: принудительно освобождает worktree (git worktree remove --force). Только admin, требует confirm:true. Используй ТОЛЬКО когда оператор явно попросил unstick застрявшего worktree.",
		}, makeWorktreeReleaseHandler(worktreeMgr))
	}

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

// makeWorktreeReleaseHandler — деструктивный manual-unstick через MCP.
//
// Гарды (порядок важен — каждый сужает поверхность атаки):
//  1. Confirm == true. Если LLM вызвал без явного намерения оператора —
//     ValidationErr; до WorktreeManager не доходим.
//  2. Caller — admin (UserRoleFromContext). Не админ → ValidationErr с префиксом
//     "forbidden:". Семантика отличается от HTTP 403 (MCP-протокол не различает
//     400/403 на уровне статуса — у IsError только бинарный флаг), поэтому
//     приоритет — машинно-парсимый префикс в Details, который клиенты могут
//     отличить через `strings.HasPrefix(details, "forbidden:")`.
//  3. UUID worktree валиден.
//  4. ReleaseManual → audit-log в WorktreeManager (user_id из MCP-context'а,
//     user_role="admin"). MCP-handler ДОПОЛНИТЕЛЬНО пишет audit-line с api_key_id
//     ПЕРЕД вызовом — иначе при разборе инцидента нельзя будет отличить, какой
//     именно admin-ключ инициировал вызов (ReleaseManual видит только user_id).
//     409/404 пробрасываются как user-facing details.
func makeWorktreeReleaseHandler(mgr *service.WorktreeManager) func(context.Context, *mcp.CallToolRequest, WorktreeReleaseParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p WorktreeReleaseParams) (*mcp.CallToolResult, any, error) {
		// 1) explicit confirm — guard #1.
		if !p.Confirm {
			return ValidationErr("confirm must be true: this is a destructive operation (git worktree remove --force). Pass confirm:true ONLY if operator explicitly requested unstick.")
		}

		// 2) admin-only — guard #2. Не падаем silent'ом если context не пробросил
		// роль (тест-режим / misconfigured middleware): такая же защита, как
		// в HTTP-handler. Префикс "forbidden:" — machine-parseable маркер для
		// клиентов, которым важно отличить auth-отказ от validation-отказа.
		role, ok := UserRoleFromContext(ctx)
		if !ok || role != models.RoleAdmin {
			return ValidationErr("forbidden: worktree_release requires admin role")
		}
		userID, _ := UserIDFromContext(ctx) // допустимо uuid.Nil — audit-log это переживёт.

		// 3) UUID parse — guard #3.
		worktreeID, err := uuid.Parse(p.WorktreeID)
		if err != nil {
			return ValidationErr("invalid worktree_id (must be UUID)")
		}

		// MCP-side audit (API-key dimension): пишем ДО ReleaseManual, чтобы при
		// разборе видеть попытку даже если сам Release упадёт. ApiKeyID может
		// отсутствовать (e.g. unit-тесты); тогда поле логируется как Nil-uuid —
		// это не утечка чего-либо чувствительного.
		apiKeyID, _ := ApiKeyIDFromContext(ctx)
		log.Printf("[mcp/worktree_release] audit: worktree_id=%s user_id=%s user_role=%s api_key_id=%s",
			worktreeID, userID, role, apiKeyID)

		// 4) ReleaseManual. Маппинг к user-facing сообщениям параллелен HTTP-handler'у.
		wt, err := mgr.ReleaseManual(ctx, worktreeID, userID, string(role))
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrWorktreeNotFound):
				return Err("worktree not found", err)
			case errors.Is(err, service.ErrWorktreeAlreadyReleased):
				return Err("worktree already released (no-op)", err)
			case errors.Is(err, service.ErrWorktreeInvalidPath):
				// Defence-in-depth tripwire — ОЧЕНЬ серьёзный сигнал. Логируем как Err
				// (есть internal log + user-facing нейтральное сообщение).
				return Err("internal: worktree path validation failed", err)
			default:
				return Err("failed to release worktree", err)
			}
		}
		return OK(fmt.Sprintf("Worktree %s released (state=released)", wt.ID), map[string]any{
			"worktree_id": wt.ID,
			"task_id":     wt.TaskID,
			"state":       string(wt.State),
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
