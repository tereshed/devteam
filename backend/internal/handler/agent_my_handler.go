package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentMyHandler — /api/v1/me/agents — user-level агенты текущего пользователя.
// ABAC: каждый handler проверяет agent.UserID == currentUser.
type AgentMyHandler struct {
	svc *service.AgentService
}

func NewAgentMyHandler(svc *service.AgentService) *AgentMyHandler {
	return &AgentMyHandler{svc: svc}
}

// ─────────────────────────────────────────────────────────────────────────────
// DTO
// ─────────────────────────────────────────────────────────────────────────────

type updateMyAgentRequest struct {
	RoleDescription    *string  `json:"role_description,omitempty"`
	SystemPrompt       *string  `json:"system_prompt,omitempty"`
	Model              *string  `json:"model,omitempty"`
	ProviderKind       *string  `json:"provider_kind,omitempty"`
	Temperature        *float64 `json:"temperature,omitempty"`
	MaxTokens          *int     `json:"max_tokens,omitempty"`
	IsActive           *bool    `json:"is_active,omitempty"`
	InternalMCPEnabled *bool    `json:"internal_mcp_enabled,omitempty"`
	Settings           *map[string]any `json:"settings,omitempty"`

	// Запрещённые поля — early-return 400 если переданы.
	TeamID        *uuid.UUID `json:"team_id,omitempty"`
	Role          *string    `json:"role,omitempty"`
	ExecutionKind *string    `json:"execution_kind,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// List — GET /api/v1/me/agents
// ─────────────────────────────────────────────────────────────────────────────

// List возвращает список агентов текущего пользователя.
// @Summary List my agents
// @Description Список user-level агентов текущего пользователя (assistant и т.д.).
// @Tags my-agents
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} apierror.ErrorResponse
// @Router /me/agents [get]
func (h *AgentMyHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "missing user context")
		return
	}

	f := repository.AgentFilter{UserID: &userID}
	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": items,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Get — GET /api/v1/me/agents/:id
// ─────────────────────────────────────────────────────────────────────────────

// Get возвращает полную запись user-level агента.
// @Summary Get my agent by ID
// @Tags my-agents
// @Security BearerAuth
// @Produce json
// @Param id path string true "Agent UUID"
// @Success 200 {object} models.Agent
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /me/agents/{id} [get]
func (h *AgentMyHandler) Get(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "missing user context")
		return
	}
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}

	agent, err := h.svc.GetByID(c.Request.Context(), agentID)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}

	if agent.UserID == nil || *agent.UserID != userID {
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "access denied")
		return
	}

	c.JSON(http.StatusOK, agent)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetAssistant — GET /api/v1/me/assistant
// ─────────────────────────────────────────────────────────────────────────────

// GetAssistant возвращает user-агента ассистента, провижен при отсутствии.
// Нужен вкладке настроек «Ассистент»: она может открыться раньше первого чата,
// когда per-user агент ещё не создан. Редактирование — PUT /me/agents/{id}.
// @Summary Get my assistant agent (provision if missing)
// @Tags my-agents
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.Agent
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /me/assistant [get]
func (h *AgentMyHandler) GetAssistant(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "missing user context")
		return
	}
	agent, err := h.svc.EnsureAssistantAgent(c.Request.Context(), userID)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, agent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — PUT /api/v1/me/agents/:id
// ─────────────────────────────────────────────────────────────────────────────

// Update обновляет настройки user-level агента.
// @Summary Update my agent
// @Tags my-agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent UUID"
// @Param body body updateMyAgentRequest true "patch fields"
// @Success 200 {object} models.Agent
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 422 {object} apierror.ErrorResponse
// @Router /me/agents/{id} [put]
func (h *AgentMyHandler) Update(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUnauthorized, "missing user context")
		return
	}
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}

	// ABAC first: проверяем права ДО парсинга body (защита от DoS тяжёлым JSON).
	agent, err := h.svc.GetByID(c.Request.Context(), agentID)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	if agent.UserID == nil || *agent.UserID != userID {
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "access denied")
		return
	}

	var req updateMyAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Early-return: запрещённые поля.
	if req.TeamID != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "team_id cannot be set on user-level agent")
		return
	}
	if req.Role != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "role cannot be changed (requires recreate)")
		return
	}
	if req.ExecutionKind != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "execution_kind cannot be changed (requires recreate)")
		return
	}

	in := service.UpdateAgentInput{
		RoleDescription:    req.RoleDescription,
		SystemPrompt:       req.SystemPrompt,
		Model:              req.Model,
		Temperature:        req.Temperature,
		MaxTokens:          req.MaxTokens,
		IsActive:           req.IsActive,
		InternalMCPEnabled: req.InternalMCPEnabled,
		Settings:           req.Settings,
	}
	if req.ProviderKind != nil {
		pk := models.AgentProviderKind(*req.ProviderKind)
		if !pk.IsValid() {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid provider_kind")
			return
		}
		in.ProviderKind = &pk
	}

	// §4.3 — валидация провайдера.
	providerToCheck := in.ProviderKind
	if providerToCheck == nil {
		providerToCheck = agent.ProviderKind
	}
	if in.Model != nil || in.ProviderKind != nil {
		if err := h.svc.ValidateProviderConnected(c.Request.Context(), userID, providerToCheck); err != nil {
			if errors.Is(err, service.ErrAgentProviderNotConnected) {
				apierror.JSON(c, http.StatusUnprocessableEntity, apierror.ErrBadRequest, err.Error())
				return
			}
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
			return
		}
	}

	updated, err := h.svc.Update(c.Request.Context(), agentID, in)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}
