package mcp

import (
	"context"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListGitIntegrationsParams — пустые параметры. Список читается для текущего пользователя
// (UserID берётся из MCP-context'а, см. UserIDFromContext).
type ListGitIntegrationsParams struct{}

// ListGitRepositoriesParams — параметры для получения списка репозиториев подключенного провайдера.
type ListGitRepositoriesParams struct {
	Provider  string `json:"provider" jsonschema:"Провайдер git-интеграции (github или gitlab)"`
	AccountID string `json:"account_id,omitempty" jsonschema:"git_integration_credential_id выбранного аккаунта (опц.; пусто = первый аккаунт провайдера)"`
}

// CreateGitRepositoryParams — параметры для создания репозитория.
type CreateGitRepositoryParams struct {
	Provider    string `json:"provider" jsonschema:"Провайдер git-интеграции (github или gitlab)"`
	AccountID   string `json:"account_id,omitempty" jsonschema:"git_integration_credential_id выбранного аккаунта (опц.; пусто = первый аккаунт провайдера)"`
	Name        string `json:"name" jsonschema:"Имя нового репозитория"`
	Private     bool   `json:"private" jsonschema:"Сделать ли репозиторий приватным"`
	Description string `json:"description,omitempty" jsonschema:"Описание нового репозитория"`
}

// parseOptionalAccountID разбирает опциональный account_id. Пусто → uuid.Nil (фолбэк на первый аккаунт).
func parseOptionalAccountID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(raw)
}

// RegisterGitIntegrationsTools — регистрирует MCP-инструменты для интеграции с Git (list_git_integrations, list_git_repositories, create_git_repository).
func RegisterGitIntegrationsTools(server *mcp.Server, svc service.GitIntegrationService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_git_integrations",
		Description: "Возвращает список подключённых git-провайдеров (GitHub / GitLab / BYO GitLab) для текущего пользователя. Read-only.",
	}, makeListGitIntegrationsHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_git_repositories",
		Description: "Возвращает список репозиториев подключённого git-провайдера (GitHub или GitLab) для текущего пользователя.",
	}, makeListGitRepositoriesHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_git_repository",
		Description: "Создаёт новый репозиторий у подключённого git-провайдера (GitHub или GitLab) для текущего пользователя.",
	}, makeCreateGitRepositoryHandler(svc))
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

func makeListGitRepositoriesHandler(svc service.GitIntegrationService) func(ctx context.Context, req *mcp.CallToolRequest, params *ListGitRepositoriesParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ListGitRepositoriesParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.Provider == "" {
			return ValidationErr("provider is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		provider := models.GitIntegrationProvider(params.Provider)
		if provider != models.GitIntegrationProviderGitHub && provider != models.GitIntegrationProviderGitLab {
			return ValidationErr("invalid provider, must be 'github' or 'gitlab'")
		}
		accountID, err := parseOptionalAccountID(params.AccountID)
		if err != nil {
			return ValidationErr("invalid account_id format")
		}

		repos, err := svc.ListRepositories(ctx, uid, provider, accountID)
		if err != nil {
			return Err("failed to list git repositories", err)
		}
		return OK("ok", map[string]any{"repositories": repos})
	}
}

func makeCreateGitRepositoryHandler(svc service.GitIntegrationService) func(ctx context.Context, req *mcp.CallToolRequest, params *CreateGitRepositoryParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *CreateGitRepositoryParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.Provider == "" || params.Name == "" {
			return ValidationErr("provider and name are required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		provider := models.GitIntegrationProvider(params.Provider)
		if provider != models.GitIntegrationProviderGitHub && provider != models.GitIntegrationProviderGitLab {
			return ValidationErr("invalid provider, must be 'github' or 'gitlab'")
		}
		accountID, err := parseOptionalAccountID(params.AccountID)
		if err != nil {
			return ValidationErr("invalid account_id format")
		}

		repo, err := svc.CreateRepository(ctx, uid, provider, accountID, params.Name, params.Private, params.Description)
		if err != nil {
			return Err("failed to create git repository", err)
		}
		return OK("ok", map[string]any{"repository": repo})
	}
}
