package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

// TaskHandler HTTP-слой для задач (bind → service → DTO).
type TaskHandler struct {
	service         service.TaskService
	orchestratorSvc service.OrchestratorService
	controlBus      *service.UserTaskControlBus
}

// NewTaskHandler создаёт обработчик задач.
func NewTaskHandler(svc service.TaskService, orchestratorSvc service.OrchestratorService, controlBus *service.UserTaskControlBus) *TaskHandler {
	return &TaskHandler{
		service:         svc,
		orchestratorSvc: orchestratorSvc,
		controlBus:      controlBus,
	}
}

func (h *TaskHandler) publishTaskControl(ctx context.Context, kind service.UserTaskControlType, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) {
	if h.controlBus == nil {
		return
	}
	h.controlBus.PublishCommand(ctx, service.UserTaskControlCommand{
		Kind:     kind,
		TaskID:   taskID,
		UserID:   userID,
		UserRole: userRole,
	})
}

func normalizeTaskListPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func writeTaskServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTaskNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrTaskInvalidTransition):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	case errors.Is(err, service.ErrTaskTerminalStatus):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	case errors.Is(err, service.ErrTaskConcurrentUpdate):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	case errors.Is(err, service.ErrAgentNotInTeam):
		apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrUnprocessable, err.Error())
	case errors.Is(err, service.ErrTaskParentNotFound):
		apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrUnprocessable, err.Error())
	case errors.Is(err, service.ErrTaskInvalidTitle):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTaskInvalidPriority):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTaskInvalidStatus):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTaskMessageInvalidType):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrUserCorrectionTooLarge), errors.Is(err, service.ErrUserCorrectionInvalidUTF8),
		errors.Is(err, service.ErrUserCorrectionEmpty):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// Create создаёт задачу в проекте
// @Summary Создание задачи
// @Description Создаёт задачу в указанном проекте. Статус устанавливается pending.
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.CreateTaskRequest true "Данные задачи"
// @Success 201 {object} dto.TaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON, пустой title, невалидный priority"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 422 {object} apierror.ErrorResponse "Агент не в команде / parent task не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}

	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	task, err := h.service.Create(ctx, userID, userRole, projectID, req)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}

	// Запускаем оркестрацию в фоне
	if h.orchestratorSvc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in background task orchestration", "error", r, "task_id", task.ID)
				}
			}()
			if err := h.orchestratorSvc.ProcessTask(context.Background(), task.ID); err != nil {
				slog.Error("Background task orchestration failed", "error", err, "task_id", task.ID)
			}
		}()
	}

	c.JSON(http.StatusCreated, dto.ToTaskResponse(task))
}

// List возвращает список задач проекта
// @Summary Список задач проекта
// @Description Возвращает задачи проекта с фильтрацией и пагинацией
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Param status query string false "Фильтр по статусу"
// @Param statuses query []string false "Фильтр по нескольким статусам" collectionFormat(multi)
// @Param priority query string false "Фильтр по приоритету"
// @Param assigned_agent_id query string false "Фильтр по агенту"
// @Param created_by_type query string false "Фильтр по типу создателя (user/agent)"
// @Param created_by_id query string false "Фильтр по ID создателя"
// @Param parent_task_id query string false "Фильтр по parent задаче"
// @Param root_only query bool false "Только корневые задачи (без parent)"
// @Param branch_name query string false "Фильтр по git-ветке"
// @Param search query string false "Поиск по title/description"
// @Param limit query int false "Лимит (1–200, по умолчанию 50)"
// @Param offset query int false "Смещение"
// @Param order_by query string false "Поле сортировки"
// @Param order_dir query string false "Направление сортировки (asc/desc)"
// @Success 200 {object} dto.TaskListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные query params"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}

	var req dto.ListTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	req.Limit, req.Offset = normalizeTaskListPagination(req.Limit, req.Offset)

	tasks, total, err := h.service.List(ctx, userID, userRole, projectID, req)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTaskListResponse(tasks, total, req.Limit, req.Offset))
}

// GetByID возвращает задачу по ID
// @Summary Получение задачи
// @Description Возвращает задачу по UUID с агентом и подзадачами
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} dto.TaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id} [get]
func (h *TaskHandler) GetByID(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	task, err := h.service.GetByID(ctx, userID, userRole, taskID)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// Update обновляет задачу
// @Summary Обновление задачи
// @Description Частичное обновление полей задачи (title, description, priority, status, assigned_agent_id, branch_name)
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body dto.UpdateTaskRequest true "Поля для обновления"
// @Success 200 {object} dto.TaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON/UUID, невалидный priority/status"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Недопустимый переход статуса"
// @Failure 422 {object} apierror.ErrorResponse "Агент не в команде проекта"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id} [put]
func (h *TaskHandler) Update(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	task, err := h.service.Update(ctx, userID, userRole, taskID, req)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// Delete удаляет задачу
// @Summary Удаление задачи
// @Description Удаляет задачу (каскадно удаляет messages, SET NULL для подзадач)
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Success 204 "Задача успешно удалена"
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id} [delete]
func (h *TaskHandler) Delete(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	if err := h.service.Delete(ctx, userID, userRole, taskID); err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Pause приостанавливает задачу
// @Summary Приостановка задачи
// @Description Переводит задачу в статус paused. Допустимо из: planning, in_progress, review, changes_requested, testing.
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} dto.TaskResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Недопустимый переход (задача в pending/completed/cancelled/failed)"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id}/pause [post]
func (h *TaskHandler) Pause(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	task, err := h.service.Pause(ctx, userID, userRole, taskID)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	h.publishTaskControl(ctx, service.UserTaskControlPause, userID, userRole, taskID)
	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// Cancel отменяет задачу
// @Summary Отмена задачи
// @Description Переводит задачу в терминальный статус cancelled. Устанавливает completed_at.
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} dto.TaskResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Задача уже в терминальном статусе (completed/cancelled)"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id}/cancel [post]
func (h *TaskHandler) Cancel(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	task, err := h.service.Cancel(ctx, userID, userRole, taskID)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	h.publishTaskControl(ctx, service.UserTaskControlCancel, userID, userRole, taskID)
	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// Resume возобновляет задачу
// @Summary Возобновление задачи
// @Description Переводит задачу из paused/failed в pending (полный рестарт пайплайна). Сбрасывает completed_at.
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} dto.TaskResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Задача не в статусе paused/failed"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id}/resume [post]
func (h *TaskHandler) Resume(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	task, err := h.service.Resume(ctx, userID, userRole, taskID)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	h.publishTaskControl(ctx, service.UserTaskControlResume, userID, userRole, taskID)

	// Запускаем оркестрацию в фоне
	if h.orchestratorSvc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in background task orchestration (resume)", "error", r, "task_id", task.ID)
				}
			}()
			if err := h.orchestratorSvc.ProcessTask(context.Background(), task.ID); err != nil {
				slog.Error("Background task orchestration failed (resume)", "error", err, "task_id", task.ID)
			}
		}()
	}

	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// Correct применяет коррекцию к задаче (контекст + при необходимости возврат к разработке).
// @Summary Коррекция задачи
// @Description Обновляет контекст задачи валидированным текстом; из review/testing переводит в in_progress.
// @Tags tasks
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body dto.CorrectTaskRequest true "Текст коррекции"
// @Success 200 {object} dto.TaskResponse
// @Failure 400 {object} apierror.ErrorResponse "Слишком длинный или невалидный текст"
// @Router /tasks/{id}/correct [post]
func (h *TaskHandler) Correct(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	var req dto.CorrectTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	task, err := h.service.Correct(ctx, userID, userRole, taskID, req.Text)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	h.publishTaskControl(ctx, service.UserTaskControlCorrect, userID, userRole, taskID)

	c.JSON(http.StatusOK, dto.ToTaskResponse(task))
}

// ListMessages возвращает сообщения задачи
// @Summary Список сообщений задачи
// @Description Возвращает пагинированный список сообщений задачи с фильтрацией
// @Tags task-messages
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Task ID"
// @Param message_type query string false "Фильтр по типу сообщения"
// @Param sender_type query string false "Фильтр по типу отправителя (user/agent/system)"
// @Param limit query int false "Лимит (1–200, по умолчанию 50)"
// @Param offset query int false "Смещение"
// @Success 200 {object} dto.TaskMessageListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные query params"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id}/messages [get]
func (h *TaskHandler) ListMessages(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	var req dto.ListTaskMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	req.Limit, req.Offset = normalizeTaskListPagination(req.Limit, req.Offset)

	msgs, total, err := h.service.ListMessages(ctx, userID, userRole, taskID, req)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToTaskMessageListResponse(msgs, total, req.Limit, req.Offset))
}

// AddMessage добавляет сообщение к задаче
// @Summary Добавление сообщения
// @Description Добавляет пользовательское сообщение (коррекция, комментарий) к задаче
// @Tags task-messages
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body dto.CreateTaskMessageRequest true "Данные сообщения"
// @Success 201 {object} dto.TaskMessageResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON, пустой content, невалидный message_type"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту задачи"
// @Failure 404 {object} apierror.ErrorResponse "Задача не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /tasks/{id}/messages [post]
func (h *TaskHandler) AddMessage(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid task ID format")
		return
	}

	var req dto.CreateTaskMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	msg, err := h.service.AddMessage(ctx, userID, userRole, taskID, req)
	if err != nil {
		writeTaskServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToTaskMessageResponse(msg))
}
