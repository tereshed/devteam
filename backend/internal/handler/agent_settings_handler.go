package handler

import (
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
	actor, ok := actorFromContext(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	a, err := h.teamSvc.GetAgentSettings(c.Request.Context(), actor, id)
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
	actor, ok := actorFromContext(c)
	if !ok {
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
	a, err := h.teamSvc.UpdateAgentSettings(c.Request.Context(), actor, id, req)
	if err != nil {
		mapAgentSettingsErr(c, err)
		return
	}
	c.JSON(http.StatusOK, agentSettingsToDTO(a))
}

// actorFromContext извлекает userID + флаг admin из gin.Context, заполненного middleware.AuthMiddleware.
// Sprint 15.minor: если userID есть, но roleParse не удался — actor.IsAdmin=false (deny by default).
// Раньше getUserRole-ошибка глоталась, что эквивалентно ложному "не admin" поведению, но без признака.
func actorFromContext(c *gin.Context) (service.AgentSettingsActor, bool) {
	uid, ok := getUserID(c)
	if !ok {
		return service.AgentSettingsActor{}, false
	}
	role, roleOK := getUserRole(c)
	if !roleOK {
		// Auth middleware прошёл — userID есть, но role отсутствует/некорректна.
		// Безопаснее всего — actor с IsAdmin=false; service-уровень сделает ownership-check.
		return service.AgentSettingsActor{UserID: uid, IsAdmin: false}, true
	}
	return service.AgentSettingsActor{UserID: uid, IsAdmin: role == models.RoleAdmin}, true
}

// agentSettingsToDTO делегирует в общий dto.AgentSettingsResponseFromModel (Sprint 15.Major DRY).
func agentSettingsToDTO(a *models.Agent) dto.AgentSettingsResponse {
	return dto.AgentSettingsResponseFromModel(a)
}

func mapAgentSettingsErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTeamAgentAccessDenied):
		// Не утекаем "такой агент есть, просто не ваш" — отдаём 404 как при отсутствии.
		apierror.JSON(c, http.StatusNotFound, "agent_not_found", "agent not found")
	case errors.Is(err, service.ErrTeamAgentNotFound):
		apierror.JSON(c, http.StatusNotFound, "agent_not_found", err.Error())
	case errors.Is(err, service.ErrTeamAgentInvalidCodeBackend):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	}
}
