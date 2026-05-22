package mcp

import (
	"context"

	"github.com/devteam/backend/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AntigravityAuthStatusParams — пустые параметры.
type AntigravityAuthStatusParams struct{}

// AntigravityAuthRevokeParams — параметры revoke.
type AntigravityAuthRevokeParams struct{}

// RegisterAntigravityAuthTools регистрирует MCP-инструменты по подписке Antigravity.
func RegisterAntigravityAuthTools(server *mcp.Server, svc service.AntigravityAuthService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "antigravity_auth_status",
		Description: "Возвращает статус OAuth-подписки Antigravity для текущего пользователя.",
	}, makeAntigravityAuthStatusHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "antigravity_auth_revoke",
		Description: "Отзывает OAuth-подписку Antigravity у текущего пользователя.",
	}, makeAntigravityAuthRevokeHandler(svc))
}

func makeAntigravityAuthStatusHandler(svc service.AntigravityAuthService) func(ctx context.Context, req *mcp.CallToolRequest, params *AntigravityAuthStatusParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *AntigravityAuthStatusParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		status, err := svc.Status(ctx, uid)
		if err != nil {
			return Err("failed to get antigravity auth status", err)
		}
		if status.Connected {
			return OK("connected", status)
		}
		return OK("not connected", status)
	}
}

func makeAntigravityAuthRevokeHandler(svc service.AntigravityAuthService) func(ctx context.Context, req *mcp.CallToolRequest, params *AntigravityAuthRevokeParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *AntigravityAuthRevokeParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		if err := svc.Revoke(ctx, uid); err != nil {
			return Err("failed to revoke antigravity subscription", err)
		}
		return OK("revoked", map[string]bool{"revoked": true})
	}
}
