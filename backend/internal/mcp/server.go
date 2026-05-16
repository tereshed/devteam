package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
)

// Dependencies содержит зависимости MCP-сервера
type Dependencies struct {
	Config          config.MCPConfig
	LLMService      service.LLMService
	WorkflowEngine  service.WorkflowEngine
	PromptService   service.PromptService
	ProjectService  service.ProjectService
	TeamService     service.TeamService
	TaskService     service.TaskService
	ToolDefinitionService service.ToolDefinitionService
	ConversationSvc service.ConversationService
	OrchestratorSvc service.TaskOrchestrator
	ApiKeyService   service.ApiKeyService

	// Sprint 15.15: опционально. nil — инструменты не регистрируются.
	ClaudeCodeAuthService service.ClaudeCodeAuthService

	// UI Refactoring Stage 3a — git OAuth интеграции (опционально, nil → пропускаем).
	GitIntegrationService service.GitIntegrationService

	// Sprint 15.24 — реестр MCP-серверов и Claude Code skills (опционально).
	MCPServerRegistryRepo repository.MCPServerRegistryRepository
	AgentSkillRepo        repository.AgentSkillRepository

	// Sprint 17 / Sprint 5 — v2 orchestration tools.
	// Sprint 5 review fix #1 (layer violation): handlers depend on SERVICES, not repos.
	// Все поля опциональны: nil → соответствующие tools НЕ регистрируются.
	AgentSvcV2              *service.AgentService
	OrchestrationQuerySvcV2 *service.OrchestrationQueryService
	TaskLifecycleV2         *service.TaskLifecycleService

	// Sprint 17 / 6.3 — для destructive worktree_release MCP-инструмента.
	// nil → инструмент не регистрируется (legacy clone-path: WORKTREES_ROOT не задан).
	WorktreeMgrV2 *service.WorktreeManager
}

// NewMCPServer создает MCP-сервер с зарегистрированными инструментами
func NewMCPServer(deps Dependencies) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wibe-backend",
		Version: "1.0.0",
	}, nil)

	// Регистрируем инструменты
	RegisterLLMTools(server, deps.LLMService, deps.Config)
	RegisterWorkflowTools(server, deps.WorkflowEngine, deps.Config)
	RegisterPromptTools(server, deps.PromptService)
	RegisterProjectTools(server, deps.ProjectService)
	RegisterTeamTools(server, deps.ProjectService, deps.TeamService)
	RegisterToolDefinitionTools(server, deps.ToolDefinitionService)
	RegisterTaskTools(server, deps.TaskService, deps.OrchestratorSvc)
	RegisterConversationTools(server, deps.ConversationSvc)
	RegisterClaudeCodeAuthTools(server, deps.ClaudeCodeAuthService)
	RegisterGitIntegrationsTools(server, deps.GitIntegrationService)
	RegisterAgentSettingsTools(server, deps.TeamService, deps.MCPServerRegistryRepo, deps.AgentSkillRepo)

	// Sprint 16.C — Hermes-каталог (toolsets) для UI dropdown / агента.
	RegisterHermesTools(server)

	// Sprint 17 / Sprint 5 — v2 orchestration tools (опционально, через service-слой).
	if deps.AgentSvcV2 != nil {
		RegisterAgentV2Tools(server, deps.AgentSvcV2)
	}
	if deps.OrchestrationQuerySvcV2 != nil {
		RegisterOrchestrationV2Tools(server, deps.OrchestrationQuerySvcV2, deps.TaskLifecycleV2, deps.WorktreeMgrV2)
	}

	return server
}

// NewHTTPHandler создает HTTP-хендлер для MCP-сервера с аутентификацией
func NewHTTPHandler(mcpServer *mcp.Server, apiKeyService service.ApiKeyService) http.Handler {
	// Streamable HTTP транспорт — рекомендуемый для удалённых MCP-серверов
	mcpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	// Оборачиваем в auth middleware
	return NewAuthMiddleware(mcpHandler, apiKeyService)
}
