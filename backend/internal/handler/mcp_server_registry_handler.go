package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type MCPServerRegistryHandler struct {
	svc *service.MCPServerRegistryService
}

func NewMCPServerRegistryHandler(svc *service.MCPServerRegistryService) *MCPServerRegistryHandler {
	return &MCPServerRegistryHandler{svc: svc}
}

type mcpServerRegistryRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Transport   string         `json:"transport" binding:"required"`
	Command     string         `json:"command"`
	Args        datatypes.JSON `json:"args"`
	URL             string         `json:"url"`
	EnvTemplate     datatypes.JSON `json:"env_template"`
	HeadersTemplate datatypes.JSON `json:"headers_template"`
	Scope           string         `json:"scope"`
	IsActive        *bool          `json:"is_active"`
}

// List returns all MCP servers in the registry.
// @Summary List MCP servers
// @Tags admin-mcp-servers
// @Security BearerAuth
// @Produce json
// @Param only_active query bool false "Only active servers"
// @Success 200 {array} models.MCPServerRegistry
// @Router /admin/mcp-servers [get]
func (h *MCPServerRegistryHandler) List(c *gin.Context) {
	onlyActive := c.Query("only_active") == "true"

	items, err := h.svc.List(c.Request.Context(), onlyActive)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, items)
}

// Get returns a single MCP server by ID.
// @Summary Get MCP server
// @Tags admin-mcp-servers
// @Security BearerAuth
// @Produce json
// @Param id path string true "Server UUID"
// @Success 200 {object} models.MCPServerRegistry
// @Router /admin/mcp-servers/{id} [get]
func (h *MCPServerRegistryHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid server id")
		return
	}

	srv, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeMCPRegistryError(c, err)
		return
	}
	c.JSON(http.StatusOK, srv)
}

// Create adds a new MCP server to the registry.
// @Summary Create MCP server
// @Tags admin-mcp-servers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body mcpServerRegistryRequest true "Server definition"
// @Success 201 {object} models.MCPServerRegistry
// @Router /admin/mcp-servers [post]
func (h *MCPServerRegistryHandler) Create(c *gin.Context) {
	var req mcpServerRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	srv, err := h.svc.Create(c.Request.Context(), service.CreateMCPServerInput{
		Name:            req.Name,
		Description:     req.Description,
		Transport:       req.Transport,
		Command:         req.Command,
		Args:            req.Args,
		URL:             req.URL,
		EnvTemplate:     req.EnvTemplate,
		HeadersTemplate: req.HeadersTemplate,
		Scope:           req.Scope,
		IsActive:        req.IsActive,
	})
	if err != nil {
		writeMCPRegistryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, srv)
}

// Update modifies an existing MCP server.
// @Summary Update MCP server
// @Tags admin-mcp-servers
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Server UUID"
// @Param body body mcpServerRegistryRequest true "Server definition"
// @Success 200 {object} models.MCPServerRegistry
// @Router /admin/mcp-servers/{id} [put]
func (h *MCPServerRegistryHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid server id")
		return
	}

	var req mcpServerRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	srv, err := h.svc.Update(c.Request.Context(), id, service.UpdateMCPServerInput{
		Name:            req.Name,
		Description:     req.Description,
		Transport:       req.Transport,
		Command:         req.Command,
		Args:            req.Args,
		URL:             req.URL,
		EnvTemplate:     req.EnvTemplate,
		HeadersTemplate: req.HeadersTemplate,
		Scope:           req.Scope,
		IsActive:        req.IsActive,
	})
	if err != nil {
		writeMCPRegistryError(c, err)
		return
	}
	c.JSON(http.StatusOK, srv)
}

// Delete soft-deletes an MCP server (sets is_active = false).
// @Summary Delete MCP server
// @Tags admin-mcp-servers
// @Security BearerAuth
// @Param id path string true "Server UUID"
// @Success 204
// @Router /admin/mcp-servers/{id} [delete]
func (h *MCPServerRegistryHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid server id")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeMCPRegistryError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeMCPRegistryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMCPServerNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "mcp server not found")
	case errors.Is(err, service.ErrMCPServerValidation):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrMCPServerDuplicateName):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, "mcp server with this name already exists")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
