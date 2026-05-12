package mcp

import (
	"context"

	"github.com/devteam/backend/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClaudeCodeAuthStatusParams — пустые параметры (статус берётся для авторизованного пользователя).
type ClaudeCodeAuthStatusParams struct{}

// ClaudeCodeAuthRevokeParams — параметры revoke (тоже без полей).
type ClaudeCodeAuthRevokeParams struct{}

// RegisterClaudeCodeAuthTools регистрирует MCP-инструменты по подписке Claude Code (Sprint 15.15).
func RegisterClaudeCodeAuthTools(server *mcp.Server, svc service.ClaudeCodeAuthService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "claude_code_auth_status",
		Description: "Возвращает статус OAuth-подписки Claude Code для текущего пользователя.",
	}, makeClaudeCodeAuthStatusHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "claude_code_auth_revoke",
		Description: "Отзывает OAuth-подписку Claude Code у текущего пользователя.",
	}, makeClaudeCodeAuthRevokeHandler(svc))
}

func makeClaudeCodeAuthStatusHandler(svc service.ClaudeCodeAuthService) func(ctx context.Context, req *mcp.CallToolRequest, params *ClaudeCodeAuthStatusParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *ClaudeCodeAuthStatusParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		status, err := svc.Status(ctx, uid)
		if err != nil {
			return Err("failed to get claude code auth status", err)
		}
		if status.Connected {
			return OK("connected", status)
		}
		return OK("not connected", status)
	}
}

func makeClaudeCodeAuthRevokeHandler(svc service.ClaudeCodeAuthService) func(ctx context.Context, req *mcp.CallToolRequest, params *ClaudeCodeAuthRevokeParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *ClaudeCodeAuthRevokeParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		if err := svc.Revoke(ctx, uid); err != nil {
			return Err("failed to revoke claude code subscription", err)
		}
		return OK("revoked", map[string]bool{"revoked": true})
	}
}
