package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EnhancerHandler — HTTP-слой энхансера проекта (конфиг, прогоны, предложения).
type EnhancerHandler struct {
	service service.EnhancerService
}

// NewEnhancerHandler создаёт обработчик энхансера.
func NewEnhancerHandler(svc service.EnhancerService) *EnhancerHandler {
	return &EnhancerHandler{service: svc}
}

func writeEnhancerServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEnhancerRunNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrEnhancerInvalidCron),
		errors.Is(err, service.ErrEnhancerInvalidAutonomy),
		errors.Is(err, service.ErrEnhancerInvalidWindow),
		errors.Is(err, service.ErrEnhancerInvalidLimit):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrEnhancerRunInProgress):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	case errors.Is(err, service.ErrEnhancerChangeNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrEnhancerChangeBadState),
		errors.Is(err, service.ErrEnhancerChangeInvalidPayload):
		apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrUnprocessable, err.Error())
	case errors.Is(err, service.ErrEnhancerChangeConflict):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// GetConfig возвращает конфиг энхансера проекта (или дефолт, если не настроен).
// @Summary Конфиг энхансера проекта
// @Description Возвращает настройки энхансера; если проект ещё не настраивался — дефолт (выключен, propose).
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.EnhancerConfigResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer [get]
func (h *EnhancerHandler) GetConfig(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	cfg, err := h.service.GetConfig(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToEnhancerConfigResponse(cfg))
}

// UpdateConfig частично обновляет конфиг энхансера (создаёт при первом вызове).
// @Summary Обновление конфига энхансера
// @Description Частично обновляет настройки энхансера (тумблер, cron, окно анализа, лимит предложений). В фазе 1 autonomy принимает только 'propose'.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.UpdateEnhancerConfigRequest true "Изменения"
// @Success 200 {object} dto.EnhancerConfigResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON / cron / autonomy / лимиты"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer [put]
func (h *EnhancerHandler) UpdateConfig(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	var req dto.UpdateEnhancerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), userID, userRole, projectID, req)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToEnhancerConfigResponse(cfg))
}

// RunNow запускает прогон энхансера вручную.
// @Summary Ручной запуск прогона энхансера
// @Description Запускает анализ проекта немедленно (работает и при выключенном расписании). 409 — прогон уже идёт.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 202 {object} dto.EnhancerRunResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 409 {object} apierror.ErrorResponse "Прогон уже выполняется"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/run [post]
func (h *EnhancerHandler) RunNow(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	run, err := h.service.RunNow(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.ToEnhancerRunResponse(run))
}

// ListRuns возвращает прогоны энхансера проекта.
// @Summary Список прогонов энхансера
// @Description Возвращает последние прогоны энхансера проекта (новые сверху).
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.EnhancerRunListResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/runs [get]
func (h *EnhancerHandler) ListRuns(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	runs, err := h.service.ListRuns(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToEnhancerRunListResponse(runs))
}

// ApplyChange применяет предложение изменения.
// @Summary Применить предложение энхансера
// @Description Применяет proposed-предложение: оверрайд промпта агента (project_agent_overrides) или правку описания/настроек проекта. 409 — целевое значение изменилось с момента предложения.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param changeId path string true "Enhancer Change ID"
// @Success 200 {object} dto.EnhancerChangeResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Предложение / проект не найдены"
// @Failure 409 {object} apierror.ErrorResponse "Конфликт: цель изменилась с момента предложения"
// @Failure 422 {object} apierror.ErrorResponse "Неподходящий статус или невалидный payload"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/changes/{changeId}/apply [post]
func (h *EnhancerHandler) ApplyChange(c *gin.Context) {
	h.decideChange(c, h.service.ApplyChange)
}

// RejectChange отклоняет предложение изменения.
// @Summary Отклонить предложение энхансера
// @Description Переводит proposed-предложение в rejected. Ничего не применяется.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param changeId path string true "Enhancer Change ID"
// @Success 200 {object} dto.EnhancerChangeResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Предложение / проект не найдены"
// @Failure 422 {object} apierror.ErrorResponse "Неподходящий статус"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/changes/{changeId}/reject [post]
func (h *EnhancerHandler) RejectChange(c *gin.Context) {
	h.decideChange(c, h.service.RejectChange)
}

// RollbackChange откатывает применённое предложение.
// @Summary Откатить применённое предложение энхансера
// @Description Откатывает applied-предложение: пересборка оверрайда агента без него или возврат старого значения описания/настроек. 409 — текущее значение уже не то, что применялось.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param changeId path string true "Enhancer Change ID"
// @Success 200 {object} dto.EnhancerChangeResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Предложение / проект не найдены"
// @Failure 409 {object} apierror.ErrorResponse "Конфликт: цель менялась после применения"
// @Failure 422 {object} apierror.ErrorResponse "Неподходящий статус"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/changes/{changeId}/rollback [post]
func (h *EnhancerHandler) RollbackChange(c *gin.Context) {
	h.decideChange(c, h.service.RollbackChange)
}

// decideChange — общий каркас apply/reject/rollback.
func (h *EnhancerHandler) decideChange(
	c *gin.Context,
	action func(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, changeID uuid.UUID) (*models.EnhancerChange, error),
) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	changeID, err := uuid.Parse(c.Param("changeId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid change ID format")
		return
	}
	ch, err := action(c.Request.Context(), userID, userRole, projectID, changeID)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToEnhancerChangeResponse(ch))
}

// ListRunChanges возвращает предложения изменений одного прогона.
// @Summary Предложения изменений прогона
// @Description Возвращает предложения (enhancer_changes) указанного прогона энхансера.
// @Tags enhancer
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param runId path string true "Enhancer Run ID"
// @Success 200 {object} dto.EnhancerChangeListResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Прогон / проект не найдены"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/enhancer/runs/{runId}/changes [get]
func (h *EnhancerHandler) ListRunChanges(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid run ID format")
		return
	}
	changes, err := h.service.ListRunChanges(c.Request.Context(), userID, userRole, projectID, runID)
	if err != nil {
		writeEnhancerServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToEnhancerChangeListResponse(changes))
}
