package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Sprint 21 — Assistant Sidebar (docs/tasks/21-assistant-sidebar.md §4).
//
// Handler — тонкий слой: парсит вход, делегирует сервису, маппит ошибки в HTTP.
// Никаких бизнес-решений, никакой работы с БД. Параллель conversation_handler.go.
//
// Error mapping:
//   ErrAssistantInvalidInput            → 400 bad_request
//   ErrAssistantSessionNotFound         → 404 not_found
//   ErrAssistantSessionBusy             → 409 conflict {error:"session_busy", ...}
//   ErrAssistantNoPendingConfirmation   → 409 conflict {error:"no_pending_confirmation"}
//   ErrAssistantAlreadyConfirmed        → 409 conflict {error:"already_confirmed"}
//   ErrAssistantAgentNotConfigured      → 500 internal_server_error
//
// Аутентификация — через тот же middleware, что у conversation_handler.go
// (AuthMiddleware → middleware.GetUserID).

const (
	// Лимиты ради защиты от bytes-bomb / DoS. Те же значения, что и у
	// conversation_handler.go (см. maxRequestBodySize, maxMessageLength).
	assistantMaxMessageLength = 4096

	// Курсорная пагинация: фронт листает чат вверх — limit разумный.
	assistantDefaultMessageLimit = 30
	assistantMaxMessageLimit     = 100

	// Sidebar показывает «recent sessions» — короткий список.
	assistantDefaultSessionLimit = 50
	assistantMaxSessionLimit     = 200
)

// AssistantHandler — HTTP-слой глобального ассистента.
type AssistantHandler struct {
	service service.AssistantService
}

// NewAssistantHandler создаёт хендлер.
func NewAssistantHandler(svc service.AssistantService) *AssistantHandler {
	return &AssistantHandler{service: svc}
}

// ─────────────────────────────────────────────────────────────────────────────
// Status
// ─────────────────────────────────────────────────────────────────────────────

// GetStatus возвращает статус конфигурации ассистента.
// @Summary Статус ассистента
// @Description Возвращает статус конфигурации ассистента (настроены ли ключи).
// @Tags assistant
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.AssistantStatusResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/status [get]
func (h *AssistantHandler) GetStatus(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	status, err := h.service.GetStatus(c.Request.Context(), uid)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// ─────────────────────────────────────────────────────────────────────────────
// Sessions
// ─────────────────────────────────────────────────────────────────────────────

// CreateSession создаёт новую assistant-сессию для текущего пользователя.
// @Summary Создание assistant-сессии
// @Description Создаёт пустую глобальную сессию ассистента (scope=user, без project_id). Title выставляется автоматически после первого ответа модели.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Success 201 {object} dto.AssistantSessionResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный запрос"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions [post]
func (h *AssistantHandler) CreateSession(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	var req dto.CreateAssistantSessionRequest
	// project_id is optional, so we ignore JSON binding EOF errors if body is empty.
	_ = c.ShouldBindJSON(&req)

	sess, err := h.service.CreateSession(c.Request.Context(), uid, req.ProjectID)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToAssistantSessionResponse(sess))
}

// ListSessions возвращает список сессий пользователя.
// @Summary Список assistant-сессий
// @Description Возвращает сессии пользователя, отсортированные по last_message_at DESC NULLS LAST. По умолчанию архивные исключены.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param include_archived query bool false "Включить архивные сессии"
// @Param limit query int false "Лимит (1–200, по умолчанию 50)"
// @Param project_id query string false "ID проекта (UUID)"
// @Success 200 {object} dto.AssistantSessionListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные параметры"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions [get]
func (h *AssistantHandler) ListSessions(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	var projectID *uuid.UUID
	projectIDStr := c.Query("project_id")
	if projectIDStr != "" {
		pid, err := uuid.Parse(projectIDStr)
		if err != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project_id format")
			return
		}
		projectID = &pid
	}

	includeArchived := parseBoolQuery(c, "include_archived", false)
	limit, err := parseLimitQuery(c, assistantDefaultSessionLimit, assistantMaxSessionLimit)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	sessions, err := h.service.ListSessions(c.Request.Context(), uid, projectID, includeArchived, limit)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAssistantSessionListResponse(sessions))
}

// GetSession возвращает детали сессии.
// @Summary Получение assistant-сессии
// @Description Возвращает полную информацию о сессии (включая busy, busy_since, pending_tool_call_id для UI input-disable).
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Session ID" format(uuid)
// @Success 200 {object} dto.AssistantSessionResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 404 {object} apierror.ErrorResponse "Сессия не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions/{id} [get]
func (h *AssistantHandler) GetSession(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid session_id format")
		return
	}

	sess, err := h.service.GetSession(c.Request.Context(), sessionID, uid)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAssistantSessionResponse(sess))
}

// ArchiveSession переводит сессию в archived (soft-delete).
// @Summary Архивация assistant-сессии
// @Description Soft-delete: status='archived'. На busy=TRUE возвращает 409 — занятую петлю архивировать нельзя.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Session ID" format(uuid)
// @Success 204 "Сессия архивирована"
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 404 {object} apierror.ErrorResponse "Сессия не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Сессия занята (busy=TRUE)"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions/{id} [delete]
func (h *AssistantHandler) ArchiveSession(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid session_id format")
		return
	}

	if err := h.service.ArchiveSession(c.Request.Context(), sessionID, uid); err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Messages
// ─────────────────────────────────────────────────────────────────────────────

// GetMessages возвращает историю сообщений (курсорная пагинация).
// @Summary История сообщений assistant-сессии
// @Description Возвращает сообщения сессии в порядке (created_at, id) DESC. Курсор — пара (before_created_at, before_id) последнего элемента предыдущей страницы.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Session ID" format(uuid)
// @Param limit query int false "Лимит (1–100, по умолчанию 30)"
// @Param before_id query string false "Курсор: ID последнего сообщения предыдущей страницы" format(uuid)
// @Param before_created_at query string false "Курсор: created_at последнего сообщения предыдущей страницы (RFC3339)" format(date-time)
// @Success 200 {object} dto.AssistantMessageListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные параметры"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 404 {object} apierror.ErrorResponse "Сессия не найдена"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions/{id}/messages [get]
func (h *AssistantHandler) GetMessages(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid session_id format")
		return
	}

	limit, err := parseLimitQuery(c, assistantDefaultMessageLimit, assistantMaxMessageLimit)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	beforeID, beforeAt, err := parseCursorQuery(c)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	msgs, err := h.service.GetHistory(c.Request.Context(), sessionID, uid, limit, beforeAt, beforeID)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAssistantMessageListResponse(msgs, limit))
}

// SendMessage отправляет user-сообщение и стартует агент-петлю.
// @Summary Отправка сообщения ассистенту
// @Description Записывает user-сообщение и запускает агент-петлю в горутине. Возвращает 202 Accepted. Идемпотентность — через client_message_id в теле или X-Client-Message-ID в заголовке. Если сессия busy — 409 {error:"session_busy", pending_tool_call_id}.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Session ID" format(uuid)
// @Param X-Client-Message-ID header string false "Ключ идемпотентности (UUIDv4). Альтернатива — поле client_message_id в теле."
// @Param request body dto.SendAssistantMessageRequest true "Текст сообщения + client_message_id"
// @Success 202 {object} dto.SendAssistantMessageResponse "Сообщение принято, агент-петля стартует"
// @Success 200 {object} dto.SendAssistantMessageResponse "Дубликат по client_message_id (идемпотентность)"
// @Failure 400 {object} apierror.ErrorResponse "Невалидные данные"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 404 {object} apierror.ErrorResponse "Сессия не найдена"
// @Failure 409 {object} apierror.ErrorResponse "Сессия занята (session_busy)"
// @Failure 413 {object} apierror.ErrorResponse "Payload слишком большой"
// @Failure 415 {object} apierror.ErrorResponse "Неверный Content-Type"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions/{id}/messages [post]
func (h *AssistantHandler) SendMessage(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid session_id format")
		return
	}

	if c.ContentType() != "application/json" {
		apierror.JSON(c, http.StatusUnsupportedMediaType, apierror.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)
	defer c.Request.Body.Close()

	var req dto.SendAssistantMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apierror.JSON(c, http.StatusRequestEntityTooLarge, apierror.ErrRequestEntityTooLarge, "Payload too large (max 1MB)")
			return
		}
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	content := strings.TrimSpace(req.Content)

	// client_message_id: либо в body, либо в заголовке X-Client-Message-ID
	// (соглашение conversation_handler.go). Тело имеет приоритет, если оба
	// заданы. Пустая строка — допустимо (идемпотентность опциональна).
	clientMsgID := strings.TrimSpace(req.ClientMessageID)
	if clientMsgID == "" {
		clientMsgID = strings.TrimSpace(c.GetHeader("X-Client-Message-ID"))
	}
	if clientMsgID != "" {
		// Если задан — обязан быть валидный UUID, иначе уникальный индекс
		// в БД не защитит (любая шумная строка пройдёт).
		if _, perr := uuid.Parse(clientMsgID); perr != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "client_message_id must be a valid UUID")
			return
		}
	}

	msg, isDuplicate, err := h.service.SendMessage(c.Request.Context(), sessionID, uid, content, clientMsgID)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, dto.SendAssistantMessageResponse{
		Message:   dto.ToAssistantMessageResponse(msg),
		Duplicate: isDuplicate,
	})
}

// ConfirmToolCall — подтверждение/отказ destructive операции.
// @Summary Подтверждение destructive операции
// @Description Resume агент-петли после destructive-confirm. approved=true → исполняется MCP-tool и петля резюмируется; approved=false → синтетический deny payload, петля резюмируется и LLM объяснит отказ. Идемпотентно через атомарный UPDATE по tool_result IS NULL.
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Session ID" format(uuid)
// @Param request body dto.ConfirmToolCallRequest true "tool_call_id + approved + (опц.) client_request_id"
// @Success 202 {object} dto.ConfirmToolCallResponse "Принято, агент-петля резюмируется"
// @Failure 400 {object} apierror.ErrorResponse "Невалидные данные"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 404 {object} apierror.ErrorResponse "Сессия не найдена"
// @Failure 409 {object} apierror.ErrorResponse "no_pending_confirmation | already_confirmed | session_busy"
// @Failure 413 {object} apierror.ErrorResponse "Payload слишком большой"
// @Failure 415 {object} apierror.ErrorResponse "Неверный Content-Type"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/sessions/{id}/confirm [post]
func (h *AssistantHandler) ConfirmToolCall(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid session_id format")
		return
	}

	if c.ContentType() != "application/json" {
		apierror.JSON(c, http.StatusUnsupportedMediaType, apierror.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)
	defer c.Request.Body.Close()

	var req dto.ConfirmToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apierror.JSON(c, http.StatusRequestEntityTooLarge, apierror.ErrRequestEntityTooLarge, "Payload too large (max 1MB)")
			return
		}
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	toolCallID := strings.TrimSpace(req.ToolCallID)

	if err := h.service.ConfirmToolCall(c.Request.Context(), sessionID, uid, toolCallID, req.Approved); err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.ConfirmToolCallResponse{Accepted: true})
}

// ─────────────────────────────────────────────────────────────────────────────
// Active tasks (Tasks-tab правой панели)
// ─────────────────────────────────────────────────────────────────────────────

// ListActiveTasks — все in-progress задачи проектов пользователя.
// @Summary Активные задачи пользователя
// @Description Live-список task.state=active по всем проектам пользователя (для Tasks-tab правой панели). Возвращается одним JOIN'ом (tasks ⋈ projects WHERE projects.user_id = ?).
// @Tags assistant
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Success 200 {object} dto.AssistantActiveTasksResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /assistant/active-tasks [get]
func (h *AssistantHandler) ListActiveTasks(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	tasks, err := h.service.ListActiveTasks(c.Request.Context(), uid)
	if err != nil {
		h.respondAssistantError(c, err)
		return
	}
	c.JSON(http.StatusOK, toActiveTasksResponse(tasks))
}

// toActiveTasksResponse — локальный маппер service.ActiveTaskSummary → DTO.
// Живёт в handler (а не в пакете dto), чтобы избежать import-cycle:
// service уже импортирует dto (conversation_service.go), обратная ссылка
// запрещена компилятором.
func toActiveTasksResponse(tasks []service.ActiveTaskSummary) dto.AssistantActiveTasksResponse {
	out := make([]dto.AssistantActiveTaskResponse, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, dto.AssistantActiveTaskResponse{
			TaskID:      t.TaskID,
			ProjectID:   t.ProjectID,
			ProjectName: t.ProjectName,
			Title:       t.Title,
			State:       string(t.State),
			UpdatedAt:   t.UpdatedAt,
		})
	}
	return dto.AssistantActiveTasksResponse{Tasks: out}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// respondAssistantError — централизованный маппинг service errors → HTTP.
// Не используем httputil.RespondError, потому что у assistant'а свои коды
// ошибок (session_busy / no_pending_confirmation / already_confirmed),
// которые фронт различает по полю `error`.
func (h *AssistantHandler) respondAssistantError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAssistantInvalidInput):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrAssistantSessionNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrAssistantSessionBusy):
		apierror.JSON(c, http.StatusConflict, "session_busy", err.Error())
	case errors.Is(err, service.ErrAssistantNoPendingConfirmation):
		apierror.JSON(c, http.StatusConflict, "no_pending_confirmation", err.Error())
	case errors.Is(err, service.ErrAssistantAlreadyConfirmed):
		apierror.JSON(c, http.StatusConflict, "already_confirmed", err.Error())
	case errors.Is(err, service.ErrAssistantAgentNotConfigured):
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// parseBoolQuery читает bool из query-параметра. Невалидное значение → fallback.
// Простой helper без import strconv-only — оставляем strconv.ParseBool вместо
// маппинга «1/0/true/false» руками.
func parseBoolQuery(c *gin.Context, key string, fallback bool) bool {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

// parseLimitQuery — читает limit с диапазоном [1, max]; пустое значение → def.
// Возвращает ошибку только если limit задан, но не парсится или вне диапазона —
// тихий клемп тут опасен (запрос «limit=99999999» может означать баг фронта,
// который лучше явно показать).
func parseLimitQuery(c *gin.Context, def, max int) (int, error) {
	raw := c.Query("limit")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if n <= 0 {
		return 0, fmt.Errorf("limit must be > 0")
	}
	if n > max {
		return 0, fmt.Errorf("limit too large (max %d)", max)
	}
	return n, nil
}

// parseCursorQuery — извлекает (before_id, before_created_at) для курсорной
// пагинации. Оба параметра опциональны, но если задан один — второй обязан
// быть тоже (иначе row-comparison `(created_at, id) < (?, ?)` теряет смысл).
//
// before_created_at принимается в RFC3339. Если оба пустые — возвращает
// (uuid.Nil, time.Time{}) — репо интерпретирует это как «первая страница».
func parseCursorQuery(c *gin.Context) (uuid.UUID, time.Time, error) {
	rawID := strings.TrimSpace(c.Query("before_id"))
	rawAt := strings.TrimSpace(c.Query("before_created_at"))

	if rawID == "" && rawAt == "" {
		return uuid.Nil, time.Time{}, nil
	}
	if rawID == "" || rawAt == "" {
		return uuid.Nil, time.Time{}, fmt.Errorf("before_id and before_created_at must be provided together")
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("before_id must be a valid UUID")
	}
	t, err := time.Parse(time.RFC3339Nano, rawAt)
	if err != nil {
		// Поддерживаем также RFC3339 без nano (фронт может слать упрощённый).
		t, err = time.Parse(time.RFC3339, rawAt)
		if err != nil {
			return uuid.Nil, time.Time{}, fmt.Errorf("before_created_at must be RFC3339 timestamp")
		}
	}
	return id, t, nil
}
