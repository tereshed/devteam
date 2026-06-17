package mcp

import (
	"net/http"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Dependencies содержит зависимости MCP-сервера
type Dependencies struct {
	Config                  config.MCPConfig
	LLMService              service.LLMService
	WorkflowEngine          service.WorkflowEngine
	PromptService           service.PromptService
	ProjectService          service.ProjectService
	TeamService             service.TeamService
	TaskService             service.TaskService
	ScheduledTaskService    service.ScheduledTaskService
	EnhancerService         service.EnhancerService
	SandboxServiceConfigSvc service.SandboxServiceConfigService
	ToolDefinitionService   service.ToolDefinitionService
	ConversationSvc         service.ConversationService
	OrchestratorSvc         service.TaskOrchestrator
	ApiKeyService           service.ApiKeyService

	// Sprint 15.15: опционально. nil — инструменты не регистрируются.
	ClaudeCodeAuthService  service.ClaudeCodeAuthService
	AntigravityAuthService service.AntigravityAuthService

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

	// Sprint 21 §5 — assistant-специфичные MCP-инструменты (app_navigate,
	// assistant_active_tasks_count, whoami). Все поля опциональны: nil →
	// соответствующий tool не регистрируется (см. RegisterAssistantTools).
	Hub      *ws.Hub
	UserRepo repository.UserRepository

	// Phase 5 — MCP-инструменты для project/user секретов (опционально).
	ProjectSecretSvc *service.ProjectSecretService
	UserSecretSvc    *service.UserSecretService
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
	RegisterProjectRepositoryTools(server, deps.ProjectService)
	RegisterTeamTools(server, deps.ProjectService, deps.TeamService)
	RegisterToolDefinitionTools(server, deps.ToolDefinitionService)
	RegisterTaskTools(server, deps.TaskService, deps.OrchestratorSvc)
	RegisterScheduledTaskTools(server, deps.ScheduledTaskService)
	RegisterEnhancerTools(server, deps.EnhancerService)
	RegisterSandboxServiceTools(server, deps.SandboxServiceConfigSvc)
	RegisterConversationTools(server, deps.ConversationSvc)
	RegisterClaudeCodeAuthTools(server, deps.ClaudeCodeAuthService)
	RegisterAntigravityAuthTools(server, deps.AntigravityAuthService)
	RegisterGitIntegrationsTools(server, deps.GitIntegrationService)
	RegisterAgentSettingsTools(server, deps.TeamService, deps.MCPServerRegistryRepo, deps.AgentSkillRepo)

	// Sprint 16.C — Hermes-каталог (toolsets) для UI dropdown / агента.
	RegisterHermesTools(server)

	// Sprint 17 / Sprint 5 — v2 orchestration tools (опционально, через service-слой).
	if deps.AgentSvcV2 != nil {
		RegisterAgentV2Tools(server, deps.AgentSvcV2)
		RegisterAgentMyTools(server, deps.AgentSvcV2)
	}
	if deps.OrchestrationQuerySvcV2 != nil {
		RegisterOrchestrationV2Tools(server, deps.OrchestrationQuerySvcV2, deps.TaskLifecycleV2, deps.WorktreeMgrV2)
	}

	// Sprint 21 §5 — assistant tools (app_navigate, assistant_active_tasks_count, whoami).
	// Каждое поле опционально (nil → tool пропускается).
	//
	// deps.Hub типа *ws.Hub удовлетворяет узкому UserNotifier (см. tools_assistant.go).
	// Передаём напрямую — типобезопасно, без рантайм-каста.
	var notifier UserNotifier
	if deps.Hub != nil {
		notifier = deps.Hub
	}
	RegisterAssistantTools(server, AssistantToolsDeps{
		Notifier:    notifier,
		TaskService: deps.TaskService,
		UserRepo:    deps.UserRepo,
	})

	// Phase 5 — project/user secret tools (опционально).
	RegisterSecretTools(server, deps.ProjectSecretSvc, deps.UserSecretSvc)

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
