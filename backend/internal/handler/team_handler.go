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
		errors.Is(err, service.ErrTeamAgentInvalidCodeBackend),
		errors.Is(err, service.ErrTeamAgentInvalidProviderKind),
		errors.Is(err, service.ErrTeamAgentInvalidToolBindings),
		errors.Is(err, service.ErrTeamAgentRoleImmutable),
		errors.Is(err, service.ErrTeamAgentInvalidRole),
		errors.Is(err, service.ErrTeamCannotDeleteDevelopment),
		errors.Is(err, service.ErrTeamTypeInvalid),
		errors.Is(err, service.ErrTeamTypeCannotDeleteSystem),
		errors.Is(err, service.ErrTeamTypeInUse),
		errors.Is(err, service.ErrAgentValidation):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrTeamAgentConflict),
		errors.Is(err, service.ErrAgentNameAlreadyTaken),
		errors.Is(err, service.ErrTeamTypeAlreadyExists):
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
// @Description PATCH полей агента: model, prompt_id, code_backend, is_active, tool_bindings (полная замена привязок или [] для снятия всех)
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

// ListByProjectID возвращает список всех команд проекта
// @Summary Получение списка команд проекта
// @Description Возвращает все команды с агентами для указанного проекта
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {array} dto.TeamResponse
// @Router /projects/{id}/teams [get]
func (h *TeamHandler) ListByProjectID(c *gin.Context) {
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

	teams, err := h.teamService.ListByProjectID(c.Request.Context(), projectID)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	resp := make([]dto.TeamResponse, 0, len(teams))
	for i := range teams {
		resp = append(resp, dto.ToTeamResponse(&teams[i]))
	}

	c.JSON(http.StatusOK, resp)
}

// CreateAgent создает нового агента в команде
// @Summary Создание агента в команде
// @Description Создает агента и привязывает его к указанной команде
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param teamId path string true "Team ID"
// @Param request body dto.CreateTeamAgentRequest true "Параметры создания агента"
// @Success 201 {object} dto.AgentResponse
// @Router /projects/{id}/teams/{teamId}/agents [post]
func (h *TeamHandler) CreateAgent(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}

	teamID, err := uuid.Parse(c.Param("teamId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid team ID format")
		return
	}

	if _, err := h.projectService.GetByID(c.Request.Context(), uid, role, projectID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	var req dto.CreateTeamAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	agent, err := h.teamService.CreateAgent(c.Request.Context(), projectID, teamID, req)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToAgentResponse(agent))
}

// DeleteAgent удаляет агента из команды
// @Summary Удаление агента команды
// @Description Удаляет агента, принадлежащего команде указанного проекта
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Param agentId path string true "Agent ID"
// @Success 204
// @Failure 404 {object} apierror.ErrorResponse
// @Router /projects/{id}/team/agents/{agentId} [delete]
func (h *TeamHandler) DeleteAgent(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
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

	if err := h.teamService.DeleteAgent(c.Request.Context(), projectID, agentID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Create создает новую команду проекта
// @Summary Создание команды проекта
// @Description Создает новую команду указанного типа (development, research, analytics) в проекте
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.CreateTeamRequest true "Параметры создания"
// @Success 201 {object} dto.TeamResponse
// @Router /projects/{id}/teams [post]
func (h *TeamHandler) Create(c *gin.Context) {
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

	var req dto.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	team, err := h.teamService.Create(c.Request.Context(), projectID, req)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTeamResponse(team))
}

// Delete удаляет команду проекта
// @Summary Удаление команды проекта
// @Description Удаляет команду из проекта (разрешено только для research и analytics)
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "Project ID"
// @Param teamId path string true "Team ID"
// @Success 204 "No Content"
// @Router /projects/{id}/teams/{teamId} [delete]
func (h *TeamHandler) Delete(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	teamID, err := uuid.Parse(c.Param("teamId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid team ID format")
		return
	}

	if _, err := h.projectService.GetByID(c.Request.Context(), uid, role, projectID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	if err := h.teamService.Delete(c.Request.Context(), projectID, teamID); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListTeamTypes возвращает список всех типов команд
// @Summary Получение списка типов команд
// @Description Возвращает все доступные типы команд
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Success 200 {array} dto.TeamTypeResponse
// @Router /team-types [get]
func (h *TeamHandler) ListTeamTypes(c *gin.Context) {
	_, _, ok := requireAuth(c)
	if !ok {
		return
	}

	list, err := h.teamService.ListTeamTypes(c.Request.Context())
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	resp := make([]dto.TeamTypeResponse, 0, len(list))
	for i := range list {
		resp = append(resp, dto.ToTeamTypeResponse(&list[i]))
	}

	c.JSON(http.StatusOK, resp)
}

// CreateTeamType создает новый тип команды (admin-only)
// @Summary Создание типа команды
// @Description Создает новый тип команды (только для администраторов)
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param request body dto.CreateTeamTypeRequest true "Параметры создания"
// @Success 201 {object} dto.TeamTypeResponse
// @Router /admin/team-types [post]
func (h *TeamHandler) CreateTeamType(c *gin.Context) {
	_, _, ok := requireAuth(c)
	if !ok {
		return
	}

	var req dto.CreateTeamTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	tt, err := h.teamService.CreateTeamType(c.Request.Context(), req)
	if err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTeamTypeResponse(tt))
}

// DeleteTeamType удаляет тип команды (admin-only)
// @Summary Удаление типа команды
// @Description Удаляет тип команды по коду (только для администраторов)
// @Tags teams
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param code path string true "Код типа команды"
// @Success 204 "No Content"
// @Router /admin/team-types/{code} [delete]
func (h *TeamHandler) DeleteTeamType(c *gin.Context) {
	_, _, ok := requireAuth(c)
	if !ok {
		return
	}

	code := c.Param("code")
	if code == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Code parameter is required")
		return
	}

	if err := h.teamService.DeleteTeamType(c.Request.Context(), code); err != nil {
		writeTeamHandlerError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
