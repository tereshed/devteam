package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentRolePromptHandler — admin-only API для управления дефолтными промптами ролей агентов.
type AgentRolePromptHandler struct {
	repo repository.AgentRolePromptRepository
}

// NewAgentRolePromptHandler — конструктор.
func NewAgentRolePromptHandler(repo repository.AgentRolePromptRepository) *AgentRolePromptHandler {
	return &AgentRolePromptHandler{repo: repo}
}

// agentRolePromptResponse — DTO ответа.
type agentRolePromptResponse struct {
	ID          uuid.UUID  `json:"id"`
	Role        string     `json:"role"`
	Content     string     `json:"content"`
	Description *string    `json:"description,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *uuid.UUID `json:"updated_by,omitempty"`
}

type updateRolePromptRequest struct {
	Content     string  `json:"content" binding:"required"`
	Description *string `json:"description,omitempty"`
}

func toRolePromptResponse(p *models.AgentRolePrompt) agentRolePromptResponse {
	return agentRolePromptResponse{
		ID:          p.ID,
		Role:        p.Role,
		Content:     p.Content,
		Description: p.Description,
		UpdatedAt:   p.UpdatedAt,
		UpdatedBy:   p.UpdatedBy,
	}
}

// List возвращает все дефолтные промпты ролей.
// @Summary      List default role prompts
// @Description  Возвращает реестр дефолтных системных промптов для каждой роли агента. Используется в админке для просмотра и редактирования.
// @Tags         admin-role-prompts
// @Security     BearerAuth
// @Produce      json
// @Success      200 {array}  agentRolePromptResponse
// @Failure      401 {object} apierror.ErrorResponse
// @Failure      403 {object} apierror.ErrorResponse
// @Failure      500 {object} apierror.ErrorResponse
// @Router       /admin/agent-role-prompts [get]
func (h *AgentRolePromptHandler) List(c *gin.Context) {
	prompts, err := h.repo.List(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	result := make([]agentRolePromptResponse, 0, len(prompts))
	for _, p := range prompts {
		result = append(result, toRolePromptResponse(&p))
	}
	c.JSON(http.StatusOK, result)
}

// GetByRole возвращает дефолтный промпт для конкретной роли.
// @Summary      Get role prompt by role
// @Description  Возвращает дефолтный системный промпт для указанной роли агента.
// @Tags         admin-role-prompts
// @Security     BearerAuth
// @Produce      json
// @Param        role path string true "Роль агента (assistant, orchestrator, router, planner, ...)"
// @Success      200 {object} agentRolePromptResponse
// @Failure      401 {object} apierror.ErrorResponse
// @Failure      403 {object} apierror.ErrorResponse
// @Failure      404 {object} apierror.ErrorResponse
// @Failure      500 {object} apierror.ErrorResponse
// @Router       /admin/agent-role-prompts/{role} [get]
func (h *AgentRolePromptHandler) GetByRole(c *gin.Context) {
	role := c.Param("role")
	if role == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "role is required")
		return
	}

	prompt, err := h.repo.GetByRole(c.Request.Context(), role)
	if err != nil {
		if errors.Is(err, repository.ErrAgentRolePromptNotFound) {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "role prompt not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, toRolePromptResponse(prompt))
}

// Update обновляет дефолтный промпт для указанной роли.
// @Summary      Update role prompt
// @Description  Обновляет content и description дефолтного промпта. Только для admin. Сброс к дефолту = удалить запись в БД + перезапустить backend (seed пересоздаст).
// @Tags         admin-role-prompts
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        role path string true "Роль агента"
// @Param        body body updateRolePromptRequest true "Новый промпт"
// @Success      200 {object} agentRolePromptResponse
// @Failure      400 {object} apierror.ErrorResponse
// @Failure      401 {object} apierror.ErrorResponse
// @Failure      403 {object} apierror.ErrorResponse
// @Failure      404 {object} apierror.ErrorResponse
// @Failure      500 {object} apierror.ErrorResponse
// @Router       /admin/agent-role-prompts/{role} [put]
func (h *AgentRolePromptHandler) Update(c *gin.Context) {
	role := c.Param("role")
	if role == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "role is required")
		return
	}

	var req updateRolePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	existing, err := h.repo.GetByRole(c.Request.Context(), role)
	if err != nil {
		if errors.Is(err, repository.ErrAgentRolePromptNotFound) {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "role prompt not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	userID := getUserIDFromContext(c)
	existing.Content = req.Content
	existing.Description = req.Description
	existing.UpdatedBy = userID
	existing.UpdatedAt = time.Now().UTC()

	if err := h.repo.Upsert(c.Request.Context(), existing); err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, toRolePromptResponse(existing))
}

func getUserIDFromContext(c *gin.Context) *uuid.UUID {
	raw, exists := c.Get("user_id")
	if !exists {
		return nil
	}
	switch v := raw.(type) {
	case uuid.UUID:
		return &v
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return nil
		}
		return &id
	default:
		return nil
	}
}
