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
	OrchestratorSvc service.OrchestratorService
	ApiKeyService   service.ApiKeyService

	// Sprint 15.15: опционально. nil — инструменты не регистрируются.
	ClaudeCodeAuthService service.ClaudeCodeAuthService

	// Sprint 15.24 — реестр MCP-серверов и Claude Code skills (опционально).
	MCPServerRegistryRepo repository.MCPServerRegistryRepository
	AgentSkillRepo        repository.AgentSkillRepository
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
	RegisterAgentSettingsTools(server, deps.TeamService, deps.MCPServerRegistryRepo, deps.AgentSkillRepo)

	// Sprint 16.C — Hermes-каталог (toolsets) для UI dropdown / агента.
	RegisterHermesTools(server)

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
