package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentSettingsHandler — GET/PUT /agents/:id/settings (Sprint 15.23).
type AgentSettingsHandler struct {
	teamSvc service.TeamService
}

func NewAgentSettingsHandler(teamSvc service.TeamService) *AgentSettingsHandler {
	return &AgentSettingsHandler{teamSvc: teamSvc}
}

// Get возвращает per-agent настройки (provider, code_backend, code_backend_settings, sandbox_permissions).
// @Summary Получить настройки агента (Sprint 15)
// @Tags agents
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "UUID агента"
// @Success 200 {object} dto.AgentSettingsResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /agents/{id}/settings [get]
func (h *AgentSettingsHandler) Get(c *gin.Context) {
	if _, ok := getUserID(c); !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	a, err := h.teamSvc.GetAgentSettings(c.Request.Context(), id)
	if err != nil {
		mapAgentSettingsErr(c, err)
		return
	}
	c.JSON(http.StatusOK, agentSettingsToDTO(a))
}

// Update применяет частичное обновление настроек.
// @Summary Обновить настройки агента (Sprint 15)
// @Tags agents
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "UUID агента"
// @Param request body dto.UpdateAgentSettingsRequest true "Поля для обновления"
// @Success 200 {object} dto.AgentSettingsResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /agents/{id}/settings [put]
func (h *AgentSettingsHandler) Update(c *gin.Context) {
	if _, ok := getUserID(c); !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	var req dto.UpdateAgentSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid request body")
		return
	}
	a, err := h.teamSvc.UpdateAgentSettings(c.Request.Context(), id, req)
	if err != nil {
		mapAgentSettingsErr(c, err)
		return
	}
	c.JSON(http.StatusOK, agentSettingsToDTO(a))
}

func agentSettingsToDTO(a *models.Agent) dto.AgentSettingsResponse {
	resp := dto.AgentSettingsResponse{
		AgentID:             a.ID,
		LLMProviderID:       a.LLMProviderID,
		CodeBackendSettings: rawOrEmpty(a.CodeBackendSettings),
		SandboxPermissions:  rawOrEmpty(a.SandboxPermissions),
	}
	if a.CodeBackend != nil {
		s := string(*a.CodeBackend)
		resp.CodeBackend = &s
	}
	return resp
}

func rawOrEmpty(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}

func mapAgentSettingsErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTeamAgentNotFound):
		apierror.JSON(c, http.StatusNotFound, "agent_not_found", err.Error())
	case errors.Is(err, service.ErrTeamAgentInvalidCodeBackend):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	}
}
