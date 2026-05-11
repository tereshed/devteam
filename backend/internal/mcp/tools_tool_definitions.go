package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/service"
)

// RegisterToolDefinitionTools регистрирует MCP-инструменты каталога tool_definitions.
func RegisterToolDefinitionTools(server *mcp.Server, toolDefSvc service.ToolDefinitionService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tool_definitions_list",
		Description: "Получить глобальный каталог активных инструментов (GET /api/v1/tool-definitions).",
	}, makeToolDefinitionsListHandler(toolDefSvc))
}

func makeToolDefinitionsListHandler(toolDefSvc service.ToolDefinitionService) func(ctx context.Context, req *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		list, err := toolDefSvc.ListActiveCatalog(ctx)
		if err != nil {
			return Err("failed to list tool definitions", err)
		}
		return OK("tool definitions", list)
	}
}
