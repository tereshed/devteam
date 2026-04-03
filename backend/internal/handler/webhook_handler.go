package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	wfRepo   repository.WorkflowRepository
	engine   service.WorkflowEngine
	baseURL  string
}

func NewWebhookHandler(
	repo repository.WebhookRepository,
	wfRepo repository.WorkflowRepository,
	engine service.WorkflowEngine,
	baseURL string,
) *WebhookHandler {
	return &WebhookHandler{
		repo:    repo,
		wfRepo:  wfRepo,
		engine:  engine,
		baseURL: baseURL,
	}
}

// Create создаёт новый webhook
// @Summary Создание webhook
// @Description Создаёт новый webhook-триггер для workflow
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
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

	// Проверяем существование workflow
	if _, err := h.wfRepo.GetWorkflowByName(c.Request.Context(), req.WorkflowName); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "workflow not found: "+req.WorkflowName)
		return
	}

	// Генерируем секретный ключ
	secret, err := generateSecret(32)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "failed to generate secret")
		return
	}

	webhook := &models.WebhookTrigger{
		Name:          req.Name,
		WorkflowName:  req.WorkflowName,
		Secret:        secret,
		Description:   req.Description,
		InputJSONPath: req.InputJSONPath,
		InputTemplate: req.InputTemplate,
		AllowedIPs:    req.AllowedIPs,
		RequireSecret: req.RequireSecret,
		IsActive:      true,
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
// @Success 200 {array} dto.WebhookResponse
// @Router /webhooks [get]
func (h *WebhookHandler) List(c *gin.Context) {
	webhooks, err := h.repo.List(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	var response []dto.WebhookResponse
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
	if req.WorkflowName != nil {
		// Проверяем существование workflow
		if _, err := h.wfRepo.GetWorkflowByName(c.Request.Context(), *req.WorkflowName); err != nil {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "workflow not found")
			return
		}
		webhook.WorkflowName = *req.WorkflowName
	}
	if req.Description != nil {
		webhook.Description = *req.Description
	}
	if req.InputJSONPath != nil {
		webhook.InputJSONPath = *req.InputJSONPath
	}
	if req.InputTemplate != nil {
		webhook.InputTemplate = *req.InputTemplate
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
		h.logTrigger(c, webhook, nil, false, "failed to read body", http.StatusBadRequest)
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
			h.logTrigger(c, webhook, nil, false, "IP not allowed: "+clientIP, http.StatusForbidden)
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
			h.logTrigger(c, webhook, nil, false, "invalid signature", http.StatusUnauthorized)
			apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "invalid signature")
			return
		}
	}

	// Извлекаем input
	input := body
	if webhook.InputJSONPath != "" {
		result := gjson.Get(body, webhook.InputJSONPath)
		if result.Exists() {
			input = result.String()
		}
	} else if webhook.InputTemplate != "" {
		// Можно добавить шаблонизацию в будущем
		input = webhook.InputTemplate
	}

	// Запускаем workflow
	execution, err := h.engine.StartWorkflow(c.Request.Context(), webhook.WorkflowName, input)
	if err != nil {
		h.logTrigger(c, webhook, nil, false, err.Error(), http.StatusInternalServerError)
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	// Обновляем статистику
	h.repo.IncrementTriggerCount(c.Request.Context(), webhook.ID)
	h.logTrigger(c, webhook, &execution.ID, true, "", http.StatusOK)

	c.JSON(http.StatusOK, dto.WebhookTriggerResponse{
		Success:     true,
		ExecutionID: execution.ID.String(),
		Message:     "Workflow started successfully",
	})
}

// GetLogs возвращает логи webhook
// @Summary Логи webhook
// @Description Возвращает историю вызовов webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
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
		response = append(response, dto.WebhookLogResponse{
			ID:           log.ID.String(),
			WebhookID:    log.WebhookID.String(),
			ExecutionID:  execID,
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

func (h *WebhookHandler) toResponse(wh *models.WebhookTrigger, showSecret bool) dto.WebhookResponse {
	resp := dto.WebhookResponse{
		ID:            wh.ID.String(),
		Name:          wh.Name,
		WorkflowName:  wh.WorkflowName,
		Description:   wh.Description,
		WebhookURL:    fmt.Sprintf("%s/api/v1/hooks/%s", h.baseURL, wh.Name),
		InputJSONPath: wh.InputJSONPath,
		InputTemplate: wh.InputTemplate,
		AllowedIPs:    wh.AllowedIPs,
		RequireSecret: wh.RequireSecret,
		TriggerCount:  wh.TriggerCount,
		LastTriggered: wh.LastTriggered,
		IsActive:      wh.IsActive,
		CreatedAt:     wh.CreatedAt,
	}
	if showSecret {
		resp.Secret = wh.Secret
	}
	return resp
}

func (h *WebhookHandler) logTrigger(c *gin.Context, webhook *models.WebhookTrigger, execID *uuid.UUID, success bool, errMsg string, code int) {
	headersJSON, _ := json.Marshal(c.Request.Header)

	log := &models.WebhookLog{
		WebhookID:    webhook.ID,
		ExecutionID:  execID,
		SourceIP:     c.ClientIP(),
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

