package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// orchestration_v2_handler.go — Sprint 17 / Orchestration v2 — read-only HTTP API
// для UI (Flutter): DAG (артефакты), Router timeline, Worktrees debug screen.
//
// Все handlers — тонкие read-only обёртки над репозиториями artifact/router_decision/
// worktree. Параллель MCP-инструментам из internal/mcp/tools_orchestration_v2.go;
// service-слой переиспользуется через те же репозитории.
//
// БЕЗОПАСНОСТЬ:
//   - router_decisions: encrypted_raw_response НИКОГДА не сериализуется (по тегу json:"-"
//     в модели и Select-фильтру в репозитории).
//   - worktrees: путь не хранится в БД и не возвращается (поле отсутствует в модели).
//
// Маршруты (все под authMW, см. server.go):
//   GET /api/v1/tasks/:id/artifacts                — список (metadata only, без content)
//   GET /api/v1/tasks/:id/artifacts/:artifactId    — полный артефакт (с content)
//   GET /api/v1/tasks/:id/router-decisions         — router timeline
//   GET /api/v1/worktrees?task_id=&state=          — список worktree (debug)

// OrchestrationV2Handler — HTTP-обёртка над v2-репозиториями.
//
// taskSvc используется только в ListWorktrees для проверки task-ownership
// (Sprint 17 / 6.2): глобальный список (без task_id) — admin-only, а вариант
// с task_id допускается обычному пользователю при владении задачей. Логика
// split'а живёт в самом handler'е, поэтому маршрут навешан под общий authMW.
//
// worktreeMgr — опциональный (nil если WORKTREES_ROOT/REPO_ROOT не заданы).
// Используется в ReleaseWorktree (Sprint 17 / 6.3); если nil — endpoint отвечает
// 503 Service Unavailable, чтобы admin понимал почему кнопка не работает.
type OrchestrationV2Handler struct {
	artifactRepo repository.ArtifactRepository
	decisionRepo repository.RouterDecisionRepository
	worktreeRepo repository.WorktreeRepository
	taskSvc      service.TaskService
	worktreeMgr  *service.WorktreeManager
}

// NewOrchestrationV2Handler — конструктор.
func NewOrchestrationV2Handler(
	artifactRepo repository.ArtifactRepository,
	decisionRepo repository.RouterDecisionRepository,
	worktreeRepo repository.WorktreeRepository,
	taskSvc service.TaskService,
	worktreeMgr *service.WorktreeManager,
) *OrchestrationV2Handler {
	return &OrchestrationV2Handler{
		artifactRepo: artifactRepo,
		decisionRepo: decisionRepo,
		worktreeRepo: worktreeRepo,
		taskSvc:      taskSvc,
		worktreeMgr:  worktreeMgr,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Response DTOs (handler-level — отделены от моделей для явного контракта UI)
// ─────────────────────────────────────────────────────────────────────────────

// artifactMetadataResponse — артефакт без content (для списочного режима).
type artifactMetadataResponse struct {
	ID            string     `json:"id"`
	TaskID        string     `json:"task_id"`
	ParentID      *string    `json:"parent_id,omitempty"`
	ProducerAgent string     `json:"producer_agent"`
	Kind          string     `json:"kind"`
	Summary       string     `json:"summary"`
	Status        string     `json:"status"`
	Iteration     int        `json:"iteration"`
	CreatedAt     time.Time  `json:"created_at"`
}

// artifactFullResponse — артефакт с content (для GET по ID).
type artifactFullResponse struct {
	artifactMetadataResponse
	Content datatypes.JSON `json:"content" swaggertype:"object"`
}

// routerDecisionResponse — одно решение Router'а; БЕЗ encrypted_raw_response.
type routerDecisionResponse struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	StepNo       int       `json:"step_no"`
	ChosenAgents []string  `json:"chosen_agents"`
	Outcome      *string   `json:"outcome,omitempty"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// worktreeResponse — git worktree; БЕЗ path (хранится только в коде, см. ComputePath).
type worktreeResponse struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	SubtaskID   *string    `json:"subtask_id,omitempty"`
	BaseBranch  string     `json:"base_branch"`
	BranchName  string     `json:"branch_name"`
	State       string     `json:"state"`
	AllocatedAt time.Time  `json:"allocated_at"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
}

// listWorktreesQuery — query params для GET /worktrees.
type listWorktreesQuery struct {
	TaskID *string `form:"task_id"`
	State  *string `form:"state"`
	Limit  *int    `form:"limit"`
	Offset *int    `form:"offset"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers — artifacts
// ─────────────────────────────────────────────────────────────────────────────

// ListArtifacts возвращает метаданные артефактов задачи (без content).
// @Summary List task artifacts (metadata)
// @Description Все артефакты задачи в порядке создания. Content не загружается; для полного — GET /tasks/{id}/artifacts/{artifactId}.
// @Tags orchestration-v2
// @Security BearerAuth
// @Produce json
// @Param id path string true "Task UUID"
// @Param only_ready query bool false "только status='ready' (default false — включая superseded)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.ErrorResponse
// @Router /tasks/{id}/artifacts [get]
func (h *OrchestrationV2Handler) ListArtifacts(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid task id")
		return
	}
	onlyReady := c.Query("only_ready") == "true"
	arts, err := h.artifactRepo.ListMetadataByTaskID(c.Request.Context(), taskID, onlyReady)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	out := make([]artifactMetadataResponse, 0, len(arts))
	for i := range arts {
		out = append(out, toArtifactMetadataResponse(&arts[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"total": len(out),
	})
}

// GetArtifact возвращает полный артефакт (с content).
// @Summary Get artifact by ID (with content)
// @Tags orchestration-v2
// @Security BearerAuth
// @Produce json
// @Param id path string true "Task UUID"
// @Param artifactId path string true "Artifact UUID"
// @Success 200 {object} artifactFullResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /tasks/{id}/artifacts/{artifactId} [get]
func (h *OrchestrationV2Handler) GetArtifact(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid task id")
		return
	}
	artifactID, err := uuid.Parse(c.Param("artifactId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid artifact id")
		return
	}
	art, err := h.artifactRepo.GetByID(c.Request.Context(), artifactID)
	if err != nil {
		if errors.Is(err, repository.ErrArtifactNotFound) {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "artifact not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	// Защита от cross-task доступа: артефакт должен принадлежать указанной задаче.
	if art.TaskID != taskID {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "artifact not found")
		return
	}
	c.JSON(http.StatusOK, toArtifactFullResponse(art))
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers — router decisions
// ─────────────────────────────────────────────────────────────────────────────

// ListRouterDecisions возвращает timeline решений Router'а для задачи.
// @Summary List router decisions (timeline)
// @Description Все решения Router'а в порядке step_no. encrypted_raw_response НЕ возвращается.
// @Tags orchestration-v2
// @Security BearerAuth
// @Produce json
// @Param id path string true "Task UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.ErrorResponse
// @Router /tasks/{id}/router-decisions [get]
func (h *OrchestrationV2Handler) ListRouterDecisions(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid task id")
		return
	}
	// withRawResponse=false — repository не загружает encrypted_raw_response.
	decisions, err := h.decisionRepo.ListByTaskID(c.Request.Context(), taskID, false)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	out := make([]routerDecisionResponse, 0, len(decisions))
	for i := range decisions {
		out = append(out, toRouterDecisionResponse(&decisions[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"total": len(out),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers — worktrees
// ─────────────────────────────────────────────────────────────────────────────

// ListWorktrees возвращает список worktree'ев с опциональными фильтрами.
//
// Access policy (Sprint 17 / 6.2):
//   - Без task_id → admin-only (глобальный список раскрывает имена веток, allocation timeline'ы;
//     это чувствительный debug-сигнал, не публичный).
//   - С task_id → доступ владельцу задачи (через TaskService.GetByID, который уже инкапсулирует
//     project-membership check) ИЛИ админу.
//
// Сортировка: allocated_at DESC, limit по умолчанию = repository.WorktreeListDefaultLimit (200).
//
// @Summary List worktrees (debug)
// @Description Список git worktree'ев. Без task_id — admin-only; с task_id — нужен доступ к задаче.
// @Tags orchestration-v2
// @Security BearerAuth
// @Produce json
// @Param task_id query string false "Filter by task UUID"
// @Param state query string false "Filter by state (allocated|in_use|released)"
// @Param limit query int false "Max items (default 200, capped at 200)"
// @Param offset query int false "Offset (default 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse "no task_id requires admin role; task_id requires task access"
// @Router /worktrees [get]
func (h *OrchestrationV2Handler) ListWorktrees(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}

	var q listWorktreesQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	filter := repository.WorktreeFilter{}

	// State-фильтр (валидируется до auth-проверки, чтобы 400 имел приоритет над 403 —
	// иначе клиент видит «forbidden» вместо подсказки «неверный state»).
	if q.State != nil && *q.State != "" {
		st := models.WorktreeState(*q.State)
		if !st.IsValid() {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid state (allowed: allocated|in_use|released)")
			return
		}
		filter.State = &st
	}

	// Hard cap на limit — защита от DoS через гигантский ?limit=10000000:
	// без capа репозиторий выгрузит в память миллион строк, OOM на бэкенде.
	// Cap совпадает с repository.WorktreeListDefaultLimit (200) и обещан в swagger-аннотации.
	if q.Limit != nil {
		limit := *q.Limit
		if limit > repository.WorktreeListDefaultLimit {
			limit = repository.WorktreeListDefaultLimit
		}
		filter.Limit = limit
	}
	if q.Offset != nil {
		filter.Offset = *q.Offset
	}

	if q.TaskID != nil && *q.TaskID != "" {
		taskID, err := uuid.Parse(*q.TaskID)
		if err != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid task_id")
			return
		}
		// Проверяем доступ к задаче (членство в проекте). Админ проходит автоматически
		// внутри project-service. Сюда же попадают ошибки 404 (задача не найдена).
		if _, err := h.taskSvc.GetByID(c.Request.Context(), userID, userRole, taskID); err != nil {
			writeTaskServiceError(c, err)
			return
		}
		filter.TaskID = &taskID
	} else {
		// Глобальный режим: только админ.
		if userRole != models.RoleAdmin {
			apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "global worktrees listing requires admin role")
			return
		}
	}

	worktrees, err := h.worktreeRepo.List(c.Request.Context(), filter)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	out := make([]worktreeResponse, 0, len(worktrees))
	for i := range worktrees {
		out = append(out, toWorktreeResponse(&worktrees[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"total": len(out),
	})
}

// ReleaseWorktree выполняет ручной "unstick" worktree (Sprint 17 / 6.3).
//
// Use-case: Worktree залип в state='in_use' потому что sandbox-процесс упал
// между Step'ами (без шанса вызвать Release из cancel-флоу). Без этой кнопки
// единственная альтернатива — `psql UPDATE worktrees SET state='released'`,
// что а) роняет файлы worktree сиротами и б) требует доступа к БД в проде.
//
// Access policy: admin-only (action разрушительный, защищаем от случайного
// клика обычного пользователя который случайно попал на /worktrees-debug экран).
//
// Маппинг ошибок service-слоя:
//   - repository.ErrWorktreeNotFound       → 404
//   - service.ErrWorktreeAlreadyReleased   → 409 (operator info)
//   - service.ErrWorktreeInvalidPath       → 500 (defence-in-depth сработал — серьёзный баг)
//
// @Summary Manually release a worktree (admin)
// @Description Принудительно выполняет git worktree remove --force и помечает запись released. Admin-only. Idempotent NOT — повторный вызов на released worktree возвращает 409.
// @Tags orchestration-v2
// @Security BearerAuth
// @Produce json
// @Param id path string true "Worktree UUID"
// @Success 200 {object} worktreeResponse
// @Failure 400 {object} apierror.ErrorResponse "invalid uuid"
// @Failure 403 {object} apierror.ErrorResponse "admin-only"
// @Failure 404 {object} apierror.ErrorResponse "worktree not found"
// @Failure 409 {object} apierror.ErrorResponse "worktree already released"
// @Failure 503 {object} apierror.ErrorResponse "worktree manager not configured"
// @Router /worktrees/{id}/release [post]
func (h *OrchestrationV2Handler) ReleaseWorktree(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	if userRole != models.RoleAdmin {
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "manual worktree release requires admin role")
		return
	}
	if h.worktreeMgr == nil {
		// WORKTREES_ROOT/REPO_ROOT не заданы (legacy clone path). Admin должен видеть
		// явную причину — отдельный код `feature_not_configured` позволяет фронту
		// отличить "включи в config" от "что-то реально упало".
		apierror.JSON(c, http.StatusServiceUnavailable, apierror.ErrFeatureNotConfigured,
			"worktree manager is not configured (WORKTREES_ROOT/REPO_ROOT unset)")
		return
	}

	worktreeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid worktree id")
		return
	}

	wt, err := h.worktreeMgr.ReleaseManual(c.Request.Context(), worktreeID, userID, string(userRole))
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrWorktreeNotFound):
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "worktree not found")
		case errors.Is(err, service.ErrWorktreeAlreadyReleased):
			apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, "worktree already released")
		default:
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		}
		return
	}

	// Возвращаем актуальное состояние, чтобы UI мог обновить tile без отдельного refetch.
	c.JSON(http.StatusOK, toWorktreeResponse(wt))
}

// ─────────────────────────────────────────────────────────────────────────────
// Mapping helpers
// ─────────────────────────────────────────────────────────────────────────────

func toArtifactMetadataResponse(a *models.Artifact) artifactMetadataResponse {
	var parentID *string
	if a.ParentID != nil {
		s := a.ParentID.String()
		parentID = &s
	}
	return artifactMetadataResponse{
		ID:            a.ID.String(),
		TaskID:        a.TaskID.String(),
		ParentID:      parentID,
		ProducerAgent: a.ProducerAgent,
		Kind:          string(a.Kind),
		Summary:       a.Summary,
		Status:        string(a.Status),
		Iteration:     a.Iteration,
		CreatedAt:     a.CreatedAt,
	}
}

func toArtifactFullResponse(a *models.Artifact) artifactFullResponse {
	return artifactFullResponse{
		artifactMetadataResponse: toArtifactMetadataResponse(a),
		Content:                  a.Content,
	}
}

func toRouterDecisionResponse(d *models.RouterDecision) routerDecisionResponse {
	chosen := make([]string, 0, len(d.ChosenAgents))
	chosen = append(chosen, d.ChosenAgents...)
	var outcome *string
	if d.Outcome != nil {
		s := string(*d.Outcome)
		outcome = &s
	}
	return routerDecisionResponse{
		ID:           d.ID.String(),
		TaskID:       d.TaskID.String(),
		StepNo:       d.StepNo,
		ChosenAgents: chosen,
		Outcome:      outcome,
		Reason:       d.Reason,
		CreatedAt:    d.CreatedAt,
	}
}

func toWorktreeResponse(w *models.Worktree) worktreeResponse {
	var subtaskID *string
	if w.SubtaskID != nil {
		s := w.SubtaskID.String()
		subtaskID = &s
	}
	return worktreeResponse{
		ID:          w.ID.String(),
		TaskID:      w.TaskID.String(),
		SubtaskID:   subtaskID,
		BaseBranch:  w.BaseBranch,
		BranchName:  w.BranchName,
		State:       string(w.State),
		AllocatedAt: w.AllocatedAt,
		ReleasedAt:  w.ReleasedAt,
	}
}
