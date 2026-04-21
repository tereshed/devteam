package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
// @Summary Создать новый чат
// @Description Создает новый чат в указанном проекте
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body dto.CreateConversationRequest true "Данные чата"
// @Success 201 {object} dto.ConversationResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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

	// Ограничиваем размер тела (1 МБ) и обязательно закрываем
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	defer c.Request.Body.Close()

	var req dto.CreateConversationRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid JSON body")
		return
	}

	// Валидация после TrimSpace
	title := strings.TrimSpace(req.Title)
	if title == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "title is required")
		return
	}
	if len(title) > 255 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "title too long (max 255)")
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
// @Description Возвращает список чатов проекта с пагинацией
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param limit query int false "Лимит (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение"
// @Success 200 {object} dto.ConversationListResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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

	var query struct {
		Limit  int `form:"limit"`
		Offset int `form:"offset"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Валидация пагинации
	if query.Limit < 0 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "limit must be positive")
		return
	}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "limit cannot exceed 100")
		return
	}
	if query.Offset < 0 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "offset must be positive")
		return
	}

	conversations, total, err := h.service.ListConversations(c.Request.Context(), uid, projectID, query.Limit, query.Offset)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToConversationListResponse(conversations, total, query.Limit, query.Offset))
}

// GetByID возвращает детали чата
// @Summary Детали чата
// @Description Возвращает детали чата по ID
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Success 200 {object} dto.ConversationResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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
// @Summary Отправить сообщение
// @Description Отправляет сообщение в чат и запускает оркестрацию
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Param X-Client-Message-ID header string true "Идемпотентный ключ (UUIDv4)"
// @Param request body dto.SendMessageRequest true "Текст сообщения"
// @Success 201 {object} dto.MessageResponse
// @Success 200 {object} dto.MessageResponse "Сообщение уже было отправлено (идемпотентность)"
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 429 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	defer c.Request.Body.Close()

	var req dto.SendMessageRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid JSON body")
		return
	}

	// Валидация после TrimSpace
	content := strings.TrimSpace(req.Content)
	if content == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "content is required")
		return
	}
	if len(content) > 4096 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "content too long (max 4096)")
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
// @Summary История сообщений
// @Description Возвращает историю сообщений чата с пагинацией
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Param limit query int false "Лимит (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение"
// @Success 200 {object} dto.MessageListResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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

	var query struct {
		Limit  int `form:"limit"`
		Offset int `form:"offset"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Валидация пагинации
	if query.Limit < 0 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "limit must be positive")
		return
	}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "limit cannot exceed 100")
		return
	}
	if query.Offset < 0 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "offset must be positive")
		return
	}

	messages, total, err := h.service.GetHistory(c.Request.Context(), uid, id, query.Limit, query.Offset)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToMessageListResponse(messages, total, query.Limit, query.Offset))
}

// Delete удаляет чат
// @Summary Удалить чат
// @Description Удаляет чат по ID
// @Tags conversations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Success 204 "No Content"
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
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
