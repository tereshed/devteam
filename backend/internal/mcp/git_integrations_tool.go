package mcp

import (
	"context"

	"github.com/devteam/backend/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListGitIntegrationsParams — пустые параметры. Список читается для текущего пользователя
// (UserID берётся из MCP-context'а, см. UserIDFromContext).
type ListGitIntegrationsParams struct{}

// RegisterGitIntegrationsTools — регистрирует read-only MCP-инструмент list_git_integrations
// (UI Refactoring Stage 3a, CLAUDE.md правило про MCP для новых публичных ручек).
//
// Write-операции (init/callback/revoke) НЕ экспонируются: они требуют браузерного OAuth-флоу
// (consent screen в браузере пользователя). У MCP-клиента нет ни UI-контекста, ни средств для
// auth-redirect — write-инструмент был бы бесполезен и одновременно опасен (вернул бы
// authorize_url, который без браузера nutzen нельзя). Зеркало решения для claude_code_auth.
func RegisterGitIntegrationsTools(server *mcp.Server, svc service.GitIntegrationService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_git_integrations",
		Description: "Возвращает список подключённых git-провайдеров (GitHub / GitLab / BYO GitLab) для текущего пользователя. Read-only.",
	}, makeListGitIntegrationsHandler(svc))
}

func makeListGitIntegrationsHandler(svc service.GitIntegrationService) func(ctx context.Context, req *mcp.CallToolRequest, params *ListGitIntegrationsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *ListGitIntegrationsParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		items, err := svc.ListStatuses(ctx, uid)
		if err != nil {
			return Err("failed to list git integrations", err)
		}
		return OK("ok", map[string]any{"integrations": items})
	}
}
