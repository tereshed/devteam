package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxRequestBodySize = 1024 * 1024 // 1 MB
	maxTitleLength     = 255
	maxMessageLength   = 4096
	defaultLimit       = 20
	maxLimit           = 100
)

// ConversationHandler HTTP-слой для чатов
type ConversationHandler struct {
	service service.ConversationService
}

// NewConversationHandler создаёт обработчик чатов
func NewConversationHandler(svc service.ConversationService) *ConversationHandler {
	return &ConversationHandler{service: svc}
}

// Create создает новый чат
// @Summary Создание чата
// @Description Создает новый чат в указанном проекте для текущего пользователя
// @Tags conversations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID" format(uuid)
// @Param request body dto.CreateConversationRequest true "Данные чата"
// @Success 201 {object} dto.ConversationResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID или JSON"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{project_id}/conversations [post]
func (h *ConversationHandler) Create(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project_id format")
		return
	}

	// Проверка Content-Type
	if c.ContentType() != "application/json" {
		apierror.JSON(c, http.StatusUnsupportedMediaType, apierror.ErrBadRequest, "Content-Type must be application/json")
		return
	}

	// Ограничиваем размер тела (1 МБ) и обязательно закрываем
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)
	defer c.Request.Body.Close()

	var req dto.CreateConversationRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apierror.JSON(c, http.StatusRequestEntityTooLarge, apierror.ErrBadRequest, "Payload too large (max 1MB)")
			return
		}
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid JSON format")
		return
	}

	// Валидация после TrimSpace
	title := strings.TrimSpace(req.Title)
	if title == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "title is required")
		return
	}
	if len(title) > maxTitleLength {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, fmt.Sprintf("title too long (max %d)", maxTitleLength))
		return
	}

	conv, err := h.service.CreateConversation(c.Request.Context(), uid, projectID, title)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToConversationResponse(conv))
}

// List возвращает список чатов проекта
// @Summary Список чатов проекта
// @Description Возвращает пагинированный список чатов проекта, к которым у пользователя есть доступ
// @Tags conversations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param project_id path string true "Project ID" format(uuid)
// @Param limit query int false "Лимит (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение"
// @Success 200 {object} dto.ConversationListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID или параметры пагинации"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{project_id}/conversations [get]
func (h *ConversationHandler) List(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project_id format")
		return
	}

	limit, offset, err := httputil.ParsePagination(c)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	conversations, total, err := h.service.ListConversations(c.Request.Context(), uid, projectID, limit, offset)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToConversationListResponse(conversations, total, limit, offset))
}

// GetByID возвращает детали чата
// @Summary Получение чата
// @Description Возвращает полную информацию о чате по его ID
// @Tags conversations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Conversation ID" format(uuid)
// @Success 200 {object} dto.ConversationResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к чату"
// @Failure 404 {object} apierror.ErrorResponse "Чат не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /conversations/{id} [get]
func (h *ConversationHandler) GetByID(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid conversation_id format")
		return
	}

	conv, err := h.service.GetConversation(c.Request.Context(), uid, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToConversationResponse(conv))
}

// SendMessage отправляет сообщение в чат
// @Summary Отправка сообщения
// @Description Добавляет сообщение пользователя в чат и запускает процесс оркестрации. Поддерживает идемпотентность через заголовок X-Client-Message-ID.
// @Tags conversation-messages
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID" format(uuid)
// @Param X-Client-Message-ID header string true "Ключ идемпотентности (UUIDv4)"
// @Param request body dto.SendMessageRequest true "Текст сообщения"
// @Success 201 {object} dto.MessageResponse "Сообщение успешно создано"
// @Success 200 {object} dto.MessageResponse "Сообщение уже было отправлено (идемпотентность)"
// @Failure 400 {object} apierror.ErrorResponse "Невалидный JSON, пустой контент или невалидный ID идемпотентности"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к чату"
// @Failure 404 {object} apierror.ErrorResponse "Чат не найден"
// @Failure 429 {object} apierror.ErrorResponse "Слишком много запросов"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /conversations/{id}/messages [post]
func (h *ConversationHandler) SendMessage(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid conversation_id format")
		return
	}

	// Проверка Content-Type
	if c.ContentType() != "application/json" {
		apierror.JSON(c, http.StatusUnsupportedMediaType, apierror.ErrBadRequest, "Content-Type must be application/json")
		return
	}

	// Валидация X-Client-Message-ID (ОБЯЗАТЕЛЬНО по ТЗ)
	headerID := c.GetHeader("X-Client-Message-ID")
	if headerID == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "X-Client-Message-ID header is required")
		return
	}
	clientMsgID, err := uuid.Parse(headerID)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "X-Client-Message-ID must be a valid UUIDv4")
		return
	}

	// Ограничиваем размер тела (1 МБ) и обязательно закрываем
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)
	defer c.Request.Body.Close()

	var req dto.SendMessageRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apierror.JSON(c, http.StatusRequestEntityTooLarge, apierror.ErrBadRequest, "Payload too large (max 1MB)")
			return
		}
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid JSON format")
		return
	}

	// Валидация после TrimSpace
	content := strings.TrimSpace(req.Content)
	if content == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "content is required")
		return
	}
	if len(content) > maxMessageLength {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, fmt.Sprintf("content too long (max %d)", maxMessageLength))
		return
	}

	msg, err := h.service.SendMessage(c.Request.Context(), uid, id, content, clientMsgID)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateMessage) {
			// В нашей реализации сервис возвращает существующее сообщение при этой ошибке
			c.JSON(http.StatusOK, dto.ToMessageResponse(msg))
			return
		}
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToMessageResponse(msg))
}

// GetHistory возвращает историю сообщений чата
// @Summary История сообщений чата
// @Description Возвращает пагинированный список сообщений чата (от новых к старым)
// @Tags conversation-messages
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Conversation ID" format(uuid)
// @Param limit query int false "Лимит (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение"
// @Success 200 {object} dto.MessageListResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID или параметры пагинации"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к чату"
// @Failure 404 {object} apierror.ErrorResponse "Чат не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /conversations/{id}/messages [get]
func (h *ConversationHandler) GetHistory(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid conversation_id format")
		return
	}

	limit, offset, err := httputil.ParsePagination(c)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	messages, total, err := h.service.GetHistory(c.Request.Context(), uid, id, limit, offset)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToMessageListResponse(messages, total, limit, offset))
}

// Delete удаляет чат
// @Summary Удаление чата
// @Description Удаляет чат и все связанные с ним сообщения (Soft Delete)
// @Tags conversations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Conversation ID" format(uuid)
// @Success 204 "Чат успешно удален"
// @Failure 400 {object} apierror.ErrorResponse "Невалидный UUID"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к чату"
// @Failure 404 {object} apierror.ErrorResponse "Чат не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /conversations/{id} [delete]
func (h *ConversationHandler) Delete(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid conversation_id format")
		return
	}

	if err := h.service.DeleteConversation(c.Request.Context(), uid, id); err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
