package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AssistantMCPServerHandler — HTTP-слой per-project MCP-серверов ассистента.
// Доступ к проекту проверяется через ProjectService (ассистент работает от лица
// владельца проекта; кросс-проектный доступ по ID серверов запрещён).
type AssistantMCPServerHandler struct {
	service    service.AssistantMCPServerService
	projectSvc service.ProjectService
}

// NewAssistantMCPServerHandler создаёт обработчик MCP-серверов ассистента.
func NewAssistantMCPServerHandler(svc service.AssistantMCPServerService, projectSvc service.ProjectService) *AssistantMCPServerHandler {
	return &AssistantMCPServerHandler{service: svc, projectSvc: projectSvc}
}

func writeAssistantMCPError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, repository.ErrAssistantMCPServerNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrAssistantMCPInvalidName),
		errors.Is(err, service.ErrAssistantMCPInvalidTransport),
		errors.Is(err, service.ErrAssistantMCPInvalidURL),
		errors.Is(err, service.ErrAssistantMCPInvalidHeaders):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// requireProjectAccess проверяет доступ пользователя к проекту; при ошибке пишет
// ответ и возвращает false.
func (h *AssistantMCPServerHandler) requireProjectAccess(c *gin.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) bool {
	if _, err := h.projectSvc.GetByID(c.Request.Context(), userID, userRole, projectID); err != nil {
		writeAssistantMCPError(c, err)
		return false
	}
	return true
}

func headersJSON(m map[string]string) datatypes.JSON {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// List возвращает MCP-серверы ассистента проекта.
// @Summary MCP-серверы ассистента проекта
// @Description Список внешних MCP-серверов (remote http/sse), инструменты которых доступны ассистенту проекта.
// @Tags assistant-mcp
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.AssistantMCPServerListResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/assistant/mcp-servers [get]
func (h *AssistantMCPServerHandler) List(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	if !h.requireProjectAccess(c, userID, userRole, projectID) {
		return
	}
	items, err := h.service.List(c.Request.Context(), projectID)
	if err != nil {
		writeAssistantMCPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAssistantMCPServerListResponse(items))
}

// Create создаёт MCP-сервер ассистента.
// @Summary Создание MCP-сервера ассистента
// @Description Добавляет remote MCP-сервер (http/sse). Headers могут содержать ${secret:NAME} (резолвятся из «Переменных проекта»). require_confirmation по умолчанию true.
// @Tags assistant-mcp
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.CreateAssistantMCPServerRequest true "MCP-сервер"
// @Success 201 {object} dto.AssistantMCPServerResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные поля"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/assistant/mcp-servers [post]
func (h *AssistantMCPServerHandler) Create(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	if !h.requireProjectAccess(c, userID, userRole, projectID) {
		return
	}
	var req dto.CreateAssistantMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	cfg := &models.AssistantMCPServer{
		ProjectID:           projectID,
		Name:                req.Name,
		Transport:           models.MCPTransport(req.Transport),
		URL:                 req.URL,
		Headers:             headersJSON(req.Headers),
		RequireConfirmation: boolOr(req.RequireConfirmation, true),
		IsEnabled:           boolOr(req.IsEnabled, true),
	}
	if err := h.service.Create(c.Request.Context(), cfg); err != nil {
		writeAssistantMCPError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToAssistantMCPServerResponse(cfg))
}

// Update обновляет MCP-сервер ассистента.
// @Summary Обновление MCP-сервера ассистента
// @Tags assistant-mcp
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param serverId path string true "MCP Server ID"
// @Param request body dto.UpdateAssistantMCPServerRequest true "MCP-сервер"
// @Success 200 {object} dto.AssistantMCPServerResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные поля"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Не найдено"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/assistant/mcp-servers/{serverId} [put]
func (h *AssistantMCPServerHandler) Update(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, serverID, ok := h.parseAndAuthorizeServer(c, userID, userRole)
	if !ok {
		return
	}
	var req dto.UpdateAssistantMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	cfg := &models.AssistantMCPServer{
		ID:                  serverID,
		ProjectID:           projectID,
		Name:                req.Name,
		Transport:           models.MCPTransport(req.Transport),
		URL:                 req.URL,
		Headers:             headersJSON(req.Headers),
		RequireConfirmation: boolOr(req.RequireConfirmation, true),
		IsEnabled:           boolOr(req.IsEnabled, true),
	}
	if err := h.service.Update(c.Request.Context(), cfg); err != nil {
		writeAssistantMCPError(c, err)
		return
	}
	fresh, err := h.service.Get(c.Request.Context(), serverID)
	if err != nil {
		writeAssistantMCPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToAssistantMCPServerResponse(fresh))
}

// Delete удаляет MCP-сервер ассистента.
// @Summary Удаление MCP-сервера ассистента
// @Tags assistant-mcp
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param serverId path string true "MCP Server ID"
// @Success 204 "Удалено"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Не найдено"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/assistant/mcp-servers/{serverId} [delete]
func (h *AssistantMCPServerHandler) Delete(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	_, serverID, ok := h.parseAndAuthorizeServer(c, userID, userRole)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), serverID); err != nil {
		writeAssistantMCPError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// parseAndAuthorizeServer парсит project_id/server_id, проверяет доступ к проекту И
// что сервер принадлежит этому проекту (защита от кросс-проектного доступа по ID).
func (h *AssistantMCPServerHandler) parseAndAuthorizeServer(c *gin.Context, userID uuid.UUID, userRole models.UserRole) (projectID, serverID uuid.UUID, ok bool) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return uuid.Nil, uuid.Nil, false
	}
	serverID, err = uuid.Parse(c.Param("serverId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid server ID format")
		return uuid.Nil, uuid.Nil, false
	}
	if !h.requireProjectAccess(c, userID, userRole, projectID) {
		return uuid.Nil, uuid.Nil, false
	}
	existing, err := h.service.Get(c.Request.Context(), serverID)
	if err != nil {
		writeAssistantMCPError(c, err)
		return uuid.Nil, uuid.Nil, false
	}
	if existing == nil || existing.ProjectID != projectID {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "mcp server not found in this project")
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, serverID, true
}
