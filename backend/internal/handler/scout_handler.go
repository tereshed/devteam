package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ScoutHandler — HTTP-слой конфига разведчика проекта.
type ScoutHandler struct {
	service service.ScoutService
}

// NewScoutHandler создаёт обработчик разведчика.
func NewScoutHandler(svc service.ScoutService) *ScoutHandler {
	return &ScoutHandler{service: svc}
}

func writeScoutServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrScoutInvalidBackend),
		errors.Is(err, service.ErrScoutInvalidTimeout),
		errors.Is(err, service.ErrScoutInvalidSubscriptionID),
		errors.Is(err, service.ErrScoutInvalidProviderKind),
		errors.Is(err, service.ErrScoutProviderBackendMismatch),
		errors.Is(err, service.ErrScoutInvalidTemperature),
		errors.Is(err, service.ErrScoutInvalidSettings),
		errors.Is(err, service.ErrScoutEmptyProblem):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrScoutRunNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrScoutSubscriptionNotFound),
		errors.Is(err, service.ErrScoutNoRepositories),
		errors.Is(err, service.ErrScoutNoSubscription),
		errors.Is(err, service.ErrScoutBackendUnsupported):
		apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrUnprocessable, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// GetConfig возвращает конфиг разведчика проекта (или дефолт, если не настроен).
// @Summary Конфиг разведчика проекта
// @Description Возвращает настройки разведчика; если проект ещё не настраивался — дефолт (выключен, claude-code).
// @Tags scout
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.ScoutConfigResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scout [get]
func (h *ScoutHandler) GetConfig(c *gin.Context) {
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
		writeScoutServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScoutConfigResponse(cfg))
}

// UpdateConfig частично обновляет конфиг разведчика (создаёт при первом вызове).
// @Summary Обновление конфига разведчика
// @Description Частично обновляет настройки разведчика (тумблер, промпт, бэкенд, подписка, таймаут).
// @Tags scout
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.UpdateScoutConfigRequest true "Изменения"
// @Success 200 {object} dto.ScoutConfigResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON / бэкенд / таймаут"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 422 {object} apierror.ErrorResponse "Выбранная подписка не подключена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scout [put]
func (h *ScoutHandler) UpdateConfig(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	var req dto.UpdateScoutConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), userID, userRole, projectID, req)
	if err != nil {
		writeScoutServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScoutConfigResponse(cfg))
}

// Dispatch запускает прогон разведчика по постановке проблемы (асинхронно).
// @Summary Запуск разведчика
// @Description Запускает headless sandbox-прогон сбора контекста на подписке. Возвращает прогон в статусе running; досье появится по завершении.
// @Tags scout
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.DispatchScoutRequest true "Постановка проблемы"
// @Success 202 {object} dto.ScoutRunResponse
// @Failure 400 {object} apierror.ErrorResponse "Пустая постановка / невалидный JSON"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 422 {object} apierror.ErrorResponse "Нет репо / подписки / неподдерживаемый бэкенд"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scout/run [post]
func (h *ScoutHandler) Dispatch(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	var req dto.DispatchScoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	run, err := h.service.Dispatch(c.Request.Context(), userID, userRole, projectID, req.Problem)
	if err != nil {
		writeScoutServiceError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.ToScoutRunResponse(run))
}

// ListRuns возвращает последние прогоны разведчика проекта.
// @Summary Прогоны разведчика
// @Tags scout
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.ScoutRunListResponse
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scout/runs [get]
func (h *ScoutHandler) ListRuns(c *gin.Context) {
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
		writeScoutServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScoutRunListResponse(runs))
}

// GetRun возвращает один прогон разведчика (с досье).
// @Summary Прогон разведчика
// @Tags scout
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param runId path string true "Scout Run ID"
// @Success 200 {object} dto.ScoutRunResponse
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Прогон не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scout/runs/{runId} [get]
func (h *ScoutHandler) GetRun(c *gin.Context) {
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
	run, err := h.service.GetRun(c.Request.Context(), userID, userRole, projectID, runID)
	if err != nil {
		writeScoutServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScoutRunResponse(run))
}
