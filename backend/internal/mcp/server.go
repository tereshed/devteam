package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/config"
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
	ApiKeyService   service.ApiKeyService
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
