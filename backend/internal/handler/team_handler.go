package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

// TeamHandler HTTP-слой для команды проекта (вложенный ресурс /projects/:id/team).
type TeamHandler struct {
	teamService    service.TeamService
	projectService service.ProjectService
}

// NewTeamHandler создаёт обработчик команды проекта.
func NewTeamHandler(teamService service.TeamService, projectService service.ProjectService) *TeamHandler {
	return &TeamHandler{
		teamService:    teamService,
		projectService: projectService,
	}
}

func writeTeamHandlerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrTeamNotFound),
		errors.Is(err, service.ErrTeamAgentNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrTeamInvalidName),
		errors.Is(err, service.ErrTeamAgentInvalidModel),
		errors.Is(err, service.ErrTeamAgentInvalidCodeBackend):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTeamAgentConflict):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// GetByProjectID возвращает команду проекта с агентами
// @Summary Получение команды проекта
// @Description Возвращает команду с агентами для указанного проекта
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.TeamResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id}/team [get]
func (h *TeamHandler) GetByProjectID(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	if _, err := h.projectService.GetByID(c.Request.Context(), uid, role, projectID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	team, err := h.teamService.GetByProjectID(c.Request.Context(), projectID)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTeamResponse(team))
}

// Update обновляет команду проекта (метаданные)
// @Summary Обновление команды
// @Description Обновляет настройки команды (название)
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.UpdateTeamRequest true "Данные обновления"
// @Success 200 {object} dto.TeamResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id}/team [put]
func (h *TeamHandler) Update(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	if _, err := h.projectService.GetByID(c.Request.Context(), uid, role, projectID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	var req dto.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	team, err := h.teamService.Update(c.Request.Context(), projectID, req)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTeamResponse(team))
}

// PatchAgent частично обновляет агента команды проекта.
// @Summary Частичное обновление агента
// @Description PATCH полей агента: model, prompt_id, code_backend, is_active
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param agentId path string true "Agent ID"
// @Param request body dto.PatchAgentRequest true "Поля патча"
// @Success 200 {object} dto.TeamResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id}/team/agents/{agentId} [patch]
func (h *TeamHandler) PatchAgent(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	agentID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid agent ID format")
		return
	}

	if _, err := h.projectService.GetByID(c.Request.Context(), uid, role, projectID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	var req dto.PatchAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	team, err := h.teamService.PatchAgent(c.Request.Context(), projectID, agentID, req)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTeamResponse(team))
}
