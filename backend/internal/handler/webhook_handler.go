package handler

import (
	"context"

	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

type WebhookHandler struct {
	repo     repository.WebhookRepository
	projRepo repository.ProjectRepository
	convSvc  service.ConversationService
	taskSvc  service.TaskService
	baseURL  string
	orchestratorSvc service.TaskOrchestrator
}

func NewWebhookHandler(
	repo repository.WebhookRepository,
	projRepo repository.ProjectRepository,
	convSvc service.ConversationService,
	taskSvc service.TaskService,
	baseURL string,
	orchestratorSvc service.TaskOrchestrator,
) *WebhookHandler {
	return &WebhookHandler{
		repo:     repo,
		projRepo: projRepo,
		convSvc:  convSvc,
		taskSvc:  taskSvc,
		baseURL:  baseURL,
		orchestratorSvc: orchestratorSvc,
	}
}

// Create создаёт новый webhook
// @Summary Создание webhook
// @Description Создаёт новый webhook-триггер для workflow
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param request body dto.CreateWebhookRequest true "Данные webhook"
// @Success 201 {object} dto.WebhookResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /webhooks [post]
func (h *WebhookHandler) Create(c *gin.Context) {
	var req dto.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	if req.ProjectID == nil && req.TeamID == nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "either project_id or team_id must be provided")
		return
	}

	var projectID, teamID *uuid.UUID
	if req.ProjectID != nil && *req.ProjectID != "" {
		id, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project_id")
			return
		}
		projectID = &id
	}
	if req.TeamID != nil && *req.TeamID != "" {
		id, err := uuid.Parse(*req.TeamID)
		if err != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid team_id")
			return
		}
		teamID = &id
	}

	// Генерируем секретный ключ
	secret, err := generateSecret(32)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to generate secret")
		return
	}

	webhook := &models.WebhookTrigger{
		Name:          req.Name,
		ProjectID:     projectID,
		TeamID:        teamID,
		Instructions:  req.Instructions,
		Secret:        secret,
		Description:             req.Description,
		TaskTitleTemplate:       req.TaskTitleTemplate,
		TaskDescriptionTemplate: req.TaskDescriptionTemplate,
		TaskPriorityTemplate:    req.TaskPriorityTemplate,
		AllowedIPs:              req.AllowedIPs,
		RequireSecret:           req.RequireSecret,
		IsActive:                true,
	}

	if err := h.repo.Create(c.Request.Context(), webhook); err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(webhook, true))
}

// List возвращает список webhooks
// @Summary Список webhooks
// @Description Возвращает все webhook-триггеры
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Success 200 {array} dto.WebhookResponse
// @Router /webhooks [get]
func (h *WebhookHandler) List(c *gin.Context) {
	webhooks, err := h.repo.List(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	response := make([]dto.WebhookResponse, 0)
	for _, wh := range webhooks {
		response = append(response, h.toResponse(&wh, false))
	}

	c.JSON(http.StatusOK, response)
}

// GetByID возвращает webhook по ID
// @Summary Получение webhook
// @Description Возвращает webhook-триггер по ID
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Webhook ID"
// @Success 200 {object} dto.WebhookResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /webhooks/{id} [get]
func (h *WebhookHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid UUID")
		return
	}

	webhook, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "webhook not found")
		return
	}

	c.JSON(http.StatusOK, h.toResponse(webhook, false))
}

// Update обновляет webhook
// @Summary Обновление webhook
// @Description Обновляет webhook-триггер
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Webhook ID"
// @Param request body dto.UpdateWebhookRequest true "Данные для обновления"
// @Success 200 {object} dto.WebhookResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /webhooks/{id} [put]
func (h *WebhookHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid UUID")
		return
	}

	webhook, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "webhook not found")
		return
	}

	var req dto.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Обновляем поля
	if req.ProjectID != nil {
		if *req.ProjectID != "" {
			id, err := uuid.Parse(*req.ProjectID)
			if err != nil {
				apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project_id")
				return
			}
			webhook.ProjectID = &id
		} else {
			webhook.ProjectID = nil
		}
	}
	if req.TeamID != nil {
		if *req.TeamID != "" {
			id, err := uuid.Parse(*req.TeamID)
			if err != nil {
				apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid team_id")
				return
			}
			webhook.TeamID = &id
		} else {
			webhook.TeamID = nil
		}
	}
	if req.Description != nil {
		webhook.Description = *req.Description
	}
	if req.Instructions != nil {
		webhook.Instructions = *req.Instructions
	}
	if req.TaskTitleTemplate != nil {
		webhook.TaskTitleTemplate = *req.TaskTitleTemplate
	}
	if req.TaskDescriptionTemplate != nil {
		webhook.TaskDescriptionTemplate = *req.TaskDescriptionTemplate
	}
	if req.TaskPriorityTemplate != nil {
		webhook.TaskPriorityTemplate = *req.TaskPriorityTemplate
	}
	if req.AllowedIPs != nil {
		webhook.AllowedIPs = *req.AllowedIPs
	}
	if req.RequireSecret != nil {
		webhook.RequireSecret = *req.RequireSecret
	}
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
	}

	showSecret := false
	if req.RegenerateSecret {
		secret, err := generateSecret(32)
		if err != nil {
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to generate secret")
			return
		}
		webhook.Secret = secret
		showSecret = true
	}

	if err := h.repo.Update(c.Request.Context(), webhook); err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, h.toResponse(webhook, showSecret))
}

// Delete удаляет webhook
// @Summary Удаление webhook
// @Description Удаляет webhook-триггер
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Webhook ID"
// @Success 204
// @Failure 404 {object} apierror.ErrorResponse
// @Router /webhooks/{id} [delete]
func (h *WebhookHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid UUID")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// Trigger обрабатывает входящий webhook запрос (ПУБЛИЧНЫЙ ЭНДПОИНТ)
// @Summary Триггер webhook
// @Description Публичный эндпоинт для запуска workflow через webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Param name path string true "Имя webhook"
// @Param X-Webhook-Signature header string false "HMAC подпись (если требуется)"
// @Success 200 {object} dto.WebhookTriggerResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /hooks/{name} [post]
func (h *WebhookHandler) Trigger(c *gin.Context) {
	name := c.Param("name")

	webhook, err := h.repo.GetByName(c.Request.Context(), name)
	if err != nil {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "webhook not found")
		return
	}

	// Читаем тело запроса
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logTrigger(c, webhook, nil, nil, false, "failed to read body", http.StatusBadRequest)
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "failed to read request body")
		return
	}
	body := string(bodyBytes)

	// Проверяем IP если указаны ограничения
	if webhook.AllowedIPs != "" {
		clientIP := c.ClientIP()
		allowed := false
		for _, ip := range strings.Split(webhook.AllowedIPs, ",") {
			if strings.TrimSpace(ip) == clientIP {
				allowed = true
				break
			}
		}
		if !allowed {
			h.logTrigger(c, webhook, nil, nil, false, "IP not allowed: "+clientIP, http.StatusForbidden)
			apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "IP not allowed")
			return
		}
	}

	// Проверяем подпись если требуется
	if webhook.RequireSecret {
		signature := c.GetHeader("X-Webhook-Signature")
		if signature == "" {
			signature = c.GetHeader("X-Hub-Signature-256") // GitHub формат
		}

		if !h.verifySignature(body, signature, webhook.Secret) {
			h.logTrigger(c, webhook, nil, nil, false, "invalid signature", http.StatusUnauthorized)
			apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "invalid signature")
			return
		}
	}

	input := body

	// Запускаем создание Conversation ИЛИ создание Task
	if webhook.TeamID != nil {
		project, err := h.projRepo.GetByID(c.Request.Context(), *webhook.ProjectID)
		if err != nil {
			h.logTrigger(c, webhook, nil, nil, false, "failed to load project", http.StatusInternalServerError)
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to load project")
			return
		}

		title := fmt.Sprintf("Webhook: %s", webhook.Name)
		if webhook.TaskTitleTemplate != "" {
			title = InterpolateWithGJSON(webhook.TaskTitleTemplate, input)
		}

		desc := input
		if webhook.TaskDescriptionTemplate != "" {
			desc = InterpolateWithGJSON(webhook.TaskDescriptionTemplate, input)
		} else if webhook.Instructions != "" {
			desc = fmt.Sprintf("System Instructions:\n%s\n\nWebhook Payload:\n%s", webhook.Instructions, input)
		}

		priority := "medium"
		if webhook.TaskPriorityTemplate != "" {
			p := InterpolateWithGJSON(webhook.TaskPriorityTemplate, input)
			p = strings.ToLower(strings.TrimSpace(p))
			if p == "low" || p == "medium" || p == "high" || p == "critical" {
				priority = p
			}
		}

		taskReq := dto.CreateTaskRequest{
			Title:       title,
			Description: desc,
			Priority:    priority,
			TeamID:      webhook.TeamID,
		}

		task, err := h.taskSvc.Create(c.Request.Context(), project.UserID, models.RoleAdmin, project.ID, taskReq)
		if err != nil {
			h.logTrigger(c, webhook, nil, nil, false, err.Error(), http.StatusInternalServerError)
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
			return
		}

		if err := h.orchestratorSvc.EnqueueInitialStep(context.Background(), task.ID); err != nil {
			// Логируем ошибку, но задачу считаем созданной
			h.logTrigger(c, webhook, nil, nil, false, fmt.Sprintf("failed to enqueue step: %v", err), http.StatusInternalServerError)
		}

		h.repo.IncrementTriggerCount(c.Request.Context(), webhook.ID)
		h.logTrigger(c, webhook, nil, nil, true, "", http.StatusOK)

		c.JSON(http.StatusOK, dto.WebhookTriggerResponse{
			Success: true,
			Message: "Task created successfully",
		})
	} else if webhook.ProjectID != nil {
		// Создаем новый чат с проектным ассистентом
		// Чтобы создать чат, нужен UserID. Возьмем владельца проекта.
		project, err := h.projRepo.GetByID(c.Request.Context(), *webhook.ProjectID)
		if err != nil {
			h.logTrigger(c, webhook, nil, nil, false, "failed to load project", http.StatusInternalServerError)
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to load project")
			return
		}

		// Создаем Conversation
		convTitle := fmt.Sprintf("Webhook: %s", webhook.Name)
		conv, err := h.convSvc.CreateConversation(c.Request.Context(), project.UserID, project.ID, convTitle)
		if err != nil {
			h.logTrigger(c, webhook, nil, nil, false, "failed to create conversation", http.StatusInternalServerError)
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to create conversation")
			return
		}

		// Формируем payload
		content := input
		if webhook.Instructions != "" {
			content = fmt.Sprintf("System Instructions:\n%s\n\nWebhook Payload:\n%s", webhook.Instructions, input)
		}

		_, err = h.convSvc.SendMessage(c.Request.Context(), project.UserID, conv.ID, content, uuid.Nil)
		if err != nil {
			h.logTrigger(c, webhook, nil, &conv.ID, false, "failed to send message", http.StatusInternalServerError)
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to send message")
			return
		}

		h.repo.IncrementTriggerCount(c.Request.Context(), webhook.ID)
		h.logTrigger(c, webhook, nil, &conv.ID, true, "", http.StatusOK)

		c.JSON(http.StatusOK, dto.WebhookTriggerResponse{
			Success:        true,
			ConversationID: conv.ID.String(),
			Message:        "Conversation created successfully",
		})
	} else {
		h.logTrigger(c, webhook, nil, nil, false, "neither workflow nor project configured", http.StatusBadRequest)
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "neither workflow nor project configured")
	}
}

// GetLogs возвращает логи webhook
// @Summary Логи webhook
// @Description Возвращает историю вызовов webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Webhook ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} dto.WebhookLogResponse
// @Router /webhooks/{id}/logs [get]
func (h *WebhookHandler) GetLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid UUID")
		return
	}

	limit := 20
	offset := 0

	logs, _, err := h.repo.ListLogs(c.Request.Context(), id, limit, offset)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	var response []dto.WebhookLogResponse
	for _, log := range logs {
		var execID *string
		if log.ExecutionID != nil {
			s := log.ExecutionID.String()
			execID = &s
		}
		var convID *string
		if log.ConversationID != nil {
			s := log.ConversationID.String()
			convID = &s
		}
		response = append(response, dto.WebhookLogResponse{
			ID:             log.ID.String(),
			WebhookID:      log.WebhookID.String(),
			ExecutionID:    execID,
			ConversationID: convID,
			SourceIP:     log.SourceIP,
			Method:       log.Method,
			Success:      log.Success,
			ErrorMessage: log.ErrorMessage,
			ResponseCode: log.ResponseCode,
			CreatedAt:    log.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// Вспомогательные функции

// InterpolateWithGJSON находит плейсхолдеры вида {path.to.json} и заменяет их на значения из payload
func InterpolateWithGJSON(template string, payload string) string {
	if template == "" {
		return ""
	}

	re := regexp.MustCompile(`\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		path := match[1 : len(match)-1] // убираем фигурные скобки
		res := gjson.Get(payload, path)
		if res.Exists() {
			return res.String()
		}
		return match // если путь не найден, оставляем плейсхолдер
	})
}

func (h *WebhookHandler) toResponse(wh *models.WebhookTrigger, includeSecret bool) dto.WebhookResponse {
	showSecret := includeSecret
	if !showSecret {
		wh.Secret = ""
	}

	var projID, teamID *string
	if wh.ProjectID != nil {
		id := wh.ProjectID.String()
		projID = &id
	}
	if wh.TeamID != nil {
		id := wh.TeamID.String()
		teamID = &id
	}

	resp := dto.WebhookResponse{
		ID:                      wh.ID.String(),
		Name:                    wh.Name,
		ProjectID:               projID,
		TeamID:                  teamID,
		Instructions:            wh.Instructions,
		Description:             wh.Description,
		WebhookURL:              fmt.Sprintf("%s/api/v1/hooks/%s", h.baseURL, wh.Name),
		AllowedIPs:              wh.AllowedIPs,
		RequireSecret:           wh.RequireSecret,
		TaskTitleTemplate:       wh.TaskTitleTemplate,
		TaskDescriptionTemplate: wh.TaskDescriptionTemplate,
		TaskPriorityTemplate:    wh.TaskPriorityTemplate,
		TriggerCount:            wh.TriggerCount,
		LastTriggered:           wh.LastTriggered,
		IsActive:                wh.IsActive,
		CreatedAt:               wh.CreatedAt,
	}
	if showSecret {
		resp.Secret = wh.Secret
	}
	return resp
}

func (h *WebhookHandler) logTrigger(c *gin.Context, webhook *models.WebhookTrigger, execID *uuid.UUID, convID *uuid.UUID, success bool, errMsg string, code int) {
	headersJSON, _ := json.Marshal(c.Request.Header)

	log := &models.WebhookLog{
		WebhookID:      webhook.ID,
		ExecutionID:    execID,
		ConversationID: convID,
		SourceIP:       c.ClientIP(),
		Method:       c.Request.Method,
		Headers:      string(headersJSON),
		Success:      success,
		ErrorMessage: errMsg,
		ResponseCode: code,
	}

	h.repo.CreateLog(c.Request.Context(), log)
}

func (h *WebhookHandler) verifySignature(body, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Поддерживаем формат "sha256=xxx" (GitHub)
	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

func generateSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

