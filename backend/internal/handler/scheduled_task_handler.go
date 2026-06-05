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

// ScheduledTaskHandler — HTTP-слой регулярных (cron) задач проекта.
type ScheduledTaskHandler struct {
	service service.ScheduledTaskService
}

// NewScheduledTaskHandler создаёт обработчик регулярных задач.
func NewScheduledTaskHandler(svc service.ScheduledTaskService) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{service: svc}
}

func writeScheduledTaskServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrScheduledTaskNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrScheduledTaskInvalidName):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrScheduledTaskInvalidCron):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTaskInvalidPriority):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTeamNotInProject):
		apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrUnprocessable, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// Create создаёт регулярную задачу в проекте.
// @Summary Создание регулярной задачи
// @Description Создаёт расписание, по которому в проекте будут периодически создаваться задачи.
// @Tags scheduled-tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.CreateScheduledTaskRequest true "Данные расписания"
// @Success 201 {object} dto.ScheduledTaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON / cron / priority / имя"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 422 {object} apierror.ErrorResponse "Команда не принадлежит проекту"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scheduled-tasks [post]
func (h *ScheduledTaskHandler) Create(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	var req dto.CreateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	st, err := h.service.Create(c.Request.Context(), userID, userRole, projectID, req)
	if err != nil {
		writeScheduledTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToScheduledTaskResponse(st))
}

// List возвращает регулярные задачи проекта.
// @Summary Список регулярных задач проекта
// @Description Возвращает все расписания проекта.
// @Tags scheduled-tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.ScheduledTaskListResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scheduled-tasks [get]
func (h *ScheduledTaskHandler) List(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	items, err := h.service.List(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		writeScheduledTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScheduledTaskListResponse(items))
}

// Update частично обновляет регулярную задачу.
// @Summary Обновление регулярной задачи
// @Description Частично обновляет расписание (имя, описание, cron, приоритет, команду, активность).
// @Tags scheduled-tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param scheduleId path string true "Scheduled Task ID"
// @Param request body dto.UpdateScheduledTaskRequest true "Изменения"
// @Success 200 {object} dto.ScheduledTaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON / cron / priority / имя"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Расписание / проект не найдены"
// @Failure 422 {object} apierror.ErrorResponse "Команда не принадлежит проекту"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scheduled-tasks/{scheduleId} [put]
func (h *ScheduledTaskHandler) Update(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	scheduleID, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid scheduled task ID format")
		return
	}
	var req dto.UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	st, err := h.service.Update(c.Request.Context(), userID, userRole, projectID, scheduleID, req)
	if err != nil {
		writeScheduledTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToScheduledTaskResponse(st))
}

// Delete удаляет регулярную задачу.
// @Summary Удаление регулярной задачи
// @Description Удаляет расписание проекта. Уже созданные задачи не затрагиваются.
// @Tags scheduled-tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param scheduleId path string true "Scheduled Task ID"
// @Success 204 "No Content"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Расписание / проект не найдены"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/scheduled-tasks/{scheduleId} [delete]
func (h *ScheduledTaskHandler) Delete(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	scheduleID, err := uuid.Parse(c.Param("scheduleId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid scheduled task ID format")
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, userRole, projectID, scheduleID); err != nil {
		writeScheduledTaskServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
