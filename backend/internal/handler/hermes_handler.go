// Package handler / Sprint 16.C — Hermes-handlers (toolsets каталог).
package handler

import (
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// HermesHandler — read-only ручки про каталог Hermes (toolsets и т.п.).
// Не имеет зависимостей: каталог захардкожен в service.HermesToolsetCatalog
// и тянется напрямую (без обращения к БД / Hermes upstream API в рантайме).
type HermesHandler struct{}

// NewHermesHandler — конструктор.
func NewHermesHandler() *HermesHandler { return &HermesHandler{} }

// HermesToolsetItem — DTO для GET /hermes/toolsets.
type HermesToolsetItem struct {
	Name        string `json:"name" example:"file_ops"`
	Description string `json:"description,omitempty" example:"Read/Edit/Write/Glob/Grep over the workspace"`
}

// ListToolsets возвращает каталог Hermes toolsets для UI dropdown'а.
// @Summary Каталог Hermes toolsets
// @Description Статичный список Hermes toolsets, доступных для агента (Sprint 16.C).
// @Tags hermes
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {array} HermesToolsetItem
// @Failure 401 {object} apierror.ErrorResponse
// @Router /hermes/toolsets [get]
func (h *HermesHandler) ListToolsets(c *gin.Context) {
	if _, _, ok := requireAuth(c); !ok {
		return
	}
	cat := service.HermesToolsetCatalog
	out := make([]HermesToolsetItem, 0, len(cat))
	for _, t := range cat {
		out = append(out, HermesToolsetItem{Name: t.Name, Description: t.Description})
	}
	c.JSON(http.StatusOK, out)
}
