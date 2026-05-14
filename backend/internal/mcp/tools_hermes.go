// Package mcp / Sprint 16.C — Hermes MCP-инструменты.
package mcp

import (
	"context"

	"github.com/devteam/backend/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HermesToolsetsListParams — пустой params (read-only без фильтров).
type HermesToolsetsListParams struct{}

// HermesToolsetItemDTO — элемент ответа hermes_toolsets_list.
type HermesToolsetItemDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// RegisterHermesTools — Sprint 16.C. Регистрирует инструмент hermes_toolsets_list.
//
// Зависимостей нет: каталог захардкожен в service.HermesToolsetCatalog. Если
// в будущем появится upstream-fetch — заменить параметр на интерфейс/сервис,
// сохранив сигнатуру Register*.
func RegisterHermesTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "hermes_toolsets_list",
		Description: "Каталог Hermes toolsets для UI dropdown / агента (Sprint 16.C). Read-only.",
	}, hermesToolsetsListHandler)
}

func hermesToolsetsListHandler(ctx context.Context, _ *mcp.CallToolRequest, _ *HermesToolsetsListParams) (*mcp.CallToolResult, any, error) {
	if _, ok := UserIDFromContext(ctx); !ok {
		return ValidationErr("authentication required")
	}
	cat := service.HermesToolsetCatalog
	out := make([]HermesToolsetItemDTO, 0, len(cat))
	for _, t := range cat {
		out = append(out, HermesToolsetItemDTO{Name: t.Name, Description: t.Description})
	}
	return OK("hermes toolsets", out)
}
