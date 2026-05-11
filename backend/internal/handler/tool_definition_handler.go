package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

// ToolDefinitionHandler — GET каталога tool_definitions.
type ToolDefinitionHandler struct {
	svc service.ToolDefinitionService
}

// NewToolDefinitionHandler создаёт хендлер.
func NewToolDefinitionHandler(svc service.ToolDefinitionService) *ToolDefinitionHandler {
	return &ToolDefinitionHandler{svc: svc}
}

// List активных инструментов (глобальный каталог для UI).
// @Summary Список определений инструментов
// @Description Возвращает активные записи реестра tool_definitions, отсортированные по category и name
// @Tags tool-definitions
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Success 200 {array} dto.ToolDefinitionListItemResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /tool-definitions [get]
func (h *ToolDefinitionHandler) List(c *gin.Context) {
	if _, _, ok := requireAuth(c); !ok {
		return
	}
	list, err := h.svc.ListActiveCatalog(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
		return
	}
	c.JSON(http.StatusOK, list)
}
