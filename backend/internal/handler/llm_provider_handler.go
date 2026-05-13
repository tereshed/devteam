package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LLMProviderHandler — Sprint 15.B5: REST API для CRUD над llm_providers.
// Доступ — admin-only (фронт UI Sprint 15.30 показывает форму только админу/проектному owner'у).
type LLMProviderHandler struct {
	svc service.LLMProviderService
	// onChange — необязательный hook (например, перегенерация free-claude-proxy config).
	onChange func(ctx context.Context)
}

// NewLLMProviderHandler собирает handler.
func NewLLMProviderHandler(svc service.LLMProviderService) *LLMProviderHandler {
	return &LLMProviderHandler{svc: svc}
}

// WithOnChange прицепляет hook на любое мутирующее действие (Sprint 15.B5: free-claude-proxy reload).
func (h *LLMProviderHandler) WithOnChange(hook func(ctx context.Context)) *LLMProviderHandler {
	h.onChange = hook
	return h
}

// requireAdmin — все ручки админ-only (см. RBAC docs/rules/backend.md §4).
func (h *LLMProviderHandler) requireAdmin(c *gin.Context) bool {
	role, ok := getUserRole(c)
	if !ok || role != models.RoleAdmin {
		apierror.JSON(c, http.StatusForbidden, apierror.ErrAccessDenied,
			"admin role required to manage llm providers")
		return false
	}
	return true
}

// List — GET /llm-providers.
// @Summary Список LLM-провайдеров
// @Tags llm-providers
// @Security BearerAuth
// @Produce json
// @Success 200 {array} dto.LLMProviderResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Router /llm-providers [get]
func (h *LLMProviderHandler) List(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	items, err := h.svc.List(c.Request.Context(), false)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	out := make([]dto.LLMProviderResponse, 0, len(items))
	for i := range items {
		out = append(out, toLLMProviderResponse(&items[i]))
	}
	c.JSON(http.StatusOK, out)
}

// Create — POST /llm-providers.
// @Summary Создать LLM-провайдера
// @Tags llm-providers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateLLMProviderRequest true "Параметры"
// @Success 201 {object} dto.LLMProviderResponse
// @Router /llm-providers [post]
func (h *LLMProviderHandler) Create(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	var req dto.CreateLLMProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid request body")
		return
	}
	p, err := h.svc.Create(c.Request.Context(), llmProviderInput(req))
	if err != nil {
		h.mapErr(c, err)
		return
	}
	h.fireOnChange(c)
	c.JSON(http.StatusCreated, toLLMProviderResponse(p))
}

// Update — PUT /llm-providers/:id.
// @Summary Обновить LLM-провайдера
// @Tags llm-providers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "UUID провайдера"
// @Param request body dto.UpdateLLMProviderRequest true "Параметры"
// @Success 200 {object} dto.LLMProviderResponse
// @Router /llm-providers/{id} [put]
func (h *LLMProviderHandler) Update(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid id")
		return
	}
	var req dto.UpdateLLMProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid request body")
		return
	}
	p, err := h.svc.Update(c.Request.Context(), id, llmProviderInput(req))
	if err != nil {
		h.mapErr(c, err)
		return
	}
	h.fireOnChange(c)
	c.JSON(http.StatusOK, toLLMProviderResponse(p))
}

// Delete — DELETE /llm-providers/:id.
// @Summary Удалить LLM-провайдера
// @Tags llm-providers
// @Security BearerAuth
// @Param id path string true "UUID провайдера"
// @Success 204
// @Router /llm-providers/{id} [delete]
func (h *LLMProviderHandler) Delete(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.mapErr(c, err)
		return
	}
	h.fireOnChange(c)
	c.Status(http.StatusNoContent)
}

// HealthCheck — POST /llm-providers/:id/health-check.
// @Summary Проверка здоровья провайдера
// @Tags llm-providers
// @Security BearerAuth
// @Param id path string true "UUID провайдера"
// @Success 204
// @Failure 502 {object} apierror.ErrorResponse
// @Router /llm-providers/{id}/health-check [post]
func (h *LLMProviderHandler) HealthCheck(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid id")
		return
	}
	if err := h.svc.HealthCheck(c.Request.Context(), id); err != nil {
		// Не утекаем полный текст ошибки клиенту — там может быть URL с токеном.
		apierror.JSON(c, http.StatusBadGateway, "health_check_failed", "provider is not healthy")
		return
	}
	c.Status(http.StatusNoContent)
}

// TestConnection — POST /llm-providers/test-connection.
// @Summary Тест подключения к провайдеру (без сохранения)
// @Tags llm-providers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.TestLLMProviderConnectionRequest true "Параметры"
// @Success 204
// @Failure 502 {object} apierror.ErrorResponse
// @Router /llm-providers/test-connection [post]
func (h *LLMProviderHandler) TestConnection(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	var req dto.TestLLMProviderConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid request body")
		return
	}
	if err := h.svc.TestConnection(c.Request.Context(), llmProviderInput(req)); err != nil {
		// Не пробрасываем text ошибки наружу — может содержать credential.
		// Полный текст уходит только в server-side log.
		slog.Default().Warn("llm provider test_connection failed",
			"name", req.Name, "kind", req.Kind, "err", err.Error())
		apierror.JSON(c, http.StatusBadGateway, "test_connection_failed",
			"provider rejected the request; see server logs for details")
		return
	}
	c.Status(http.StatusNoContent)
}

// === helpers ===

func toLLMProviderResponse(p *models.LLMProvider) dto.LLMProviderResponse {
	return dto.LLMProviderResponse{
		ID:           p.ID,
		Name:         p.Name,
		Kind:         string(p.Kind),
		BaseURL:      p.BaseURL,
		AuthType:     string(p.AuthType),
		DefaultModel: p.DefaultModel,
		Enabled:      p.Enabled,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func llmProviderInput(req dto.CreateLLMProviderRequest) service.LLMProviderInput {
	authType := models.LLMProviderAuthType(req.AuthType)
	if authType == "" {
		authType = models.LLMProviderAuthAPIKey
	}
	return service.LLMProviderInput{
		Name:         req.Name,
		Kind:         models.LLMProviderKind(req.Kind),
		BaseURL:      req.BaseURL,
		AuthType:     authType,
		Credential:   req.Credential,
		DefaultModel: req.DefaultModel,
		Enabled:      req.Enabled,
	}
}

func (h *LLMProviderHandler) mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrLLMProviderNotFound):
		apierror.JSON(c, http.StatusNotFound, "llm_provider_not_found", err.Error())
	case errors.Is(err, repository.ErrLLMProviderNameExists):
		apierror.JSON(c, http.StatusConflict, "llm_provider_name_exists", err.Error())
	case errors.Is(err, service.ErrLLMProviderInvalid):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}

func (h *LLMProviderHandler) fireOnChange(c *gin.Context) {
	if h.onChange == nil {
		return
	}
	// Hook вызывается синхронно, но в отдельной горутине через детачнутый ctx —
	// чтобы клиент не ждал перегенерации YAML.
	go h.onChange(context.WithoutCancel(c.Request.Context()))
}
