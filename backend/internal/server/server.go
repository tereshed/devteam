package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/devteam/backend/internal/handler"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
	"github.com/devteam/backend/pkg/jwt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// Server представляет HTTP сервер приложения
type Server struct {
	router *gin.Engine
	db     *gorm.DB
	config ServerConfig
}

// ServerConfig содержит конфигурацию сервера
type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Dependencies содержит все зависимости для сервера
type Dependencies struct {
	AuthHandler           *handler.AuthHandler
	ApiKeyHandler         *handler.ApiKeyHandler
	LLMHandler            *handler.LLMHandler
	PromptHandler         *handler.PromptHandler
	ProjectHandler        *handler.ProjectHandler
	TeamHandler           *handler.TeamHandler
	ToolDefinitionHandler *handler.ToolDefinitionHandler
	TaskHandler           *handler.TaskHandler
	ScheduledTaskHandler  *handler.ScheduledTaskHandler
	EnhancerHandler       *handler.EnhancerHandler
	ScoutHandler          *handler.ScoutHandler
	SandboxServiceHandler *handler.SandboxServiceHandler
	WorkflowHandler       *handler.WorkflowHandler
	WebhookHandler        *handler.WebhookHandler
	ConversationHandler   *handler.ConversationHandler
	JWTManager            *jwt.Manager
	ApiKeyService         service.ApiKeyService
	WebSocketHandler      *ws.WebSocketHandler

	UserLlmCredentialHandler *handler.UserLlmCredentialHandler
	LlmCredentialsPatchRL    *middleware.LlmCredentialsPatchRateLimiter

	ClaudeCodeAuthHandler  *handler.ClaudeCodeAuthHandler
	AntigravityAuthHandler *handler.AntigravityAuthHandler

	// UI Refactoring Stage 3a — git OAuth интеграции (GitHub / GitLab / BYO GitLab).
	GitIntegrationHandler *handler.GitIntegrationHandler

	// Sprint 15.23 — per-agent settings.
	AgentSettingsHandler *handler.AgentSettingsHandler

	// Sprint 17 / Sprint 5F.3 — CRUD + секреты для реестра агентов v2.
	AgentV2Handler *handler.AgentV2Handler

	// Phase 4 §4.2 — user-level агенты (/me/agents).
	AgentMyHandler *handler.AgentMyHandler

	// Sprint 17 / Orchestration v2 — read-only API для DAG / Router timeline / Worktrees.
	OrchestrationV2Handler *handler.OrchestrationV2Handler

	// Sprint 15.B5 — CRUD над llm_providers (admin-only).
	LLMProviderHandler *handler.LLMProviderHandler

	// Sprint 16.C — Hermes-каталог (toolsets) для UI dropdown'а.
	HermesHandler *handler.HermesHandler

	// Sprint 21 — глобальный ассистент правой панели (docs/tasks/21-assistant-sidebar.md §4).
	AssistantHandler *handler.AssistantHandler

	// Per-project внешние MCP-серверы ассистента (remote http/sse).
	AssistantMCPHandler *handler.AssistantMCPServerHandler

	// Phase 1 §1.4 — admin API для дефолтных промптов ролей агентов.
	AgentRolePromptHandler *handler.AgentRolePromptHandler

	// Phase 5 — project/user secrets.
	ProjectSecretHandler *handler.ProjectSecretHandler
	UserSecretHandler    *handler.UserSecretHandler

	// «Инъекция env-файла» уровня репозитория (опционально, nil → ручки не регистрируются).
	RepositoryEnvFileHandler *handler.RepositoryEnvFileHandler

	// Phase 5 §5.6.1 — admin CRUD для реестра MCP-серверов.
	MCPServerRegistryHandler *handler.MCPServerRegistryHandler
}

// New создает новый экземпляр сервера
func New(db *gorm.DB, config ServerConfig, deps Dependencies) *Server {
	// Устанавливаем режим Gin в зависимости от окружения
	if gin.Mode() == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.LoggerWithFormatter(customLogFormatter), gin.Recovery())

	// Настраиваем CORS для работы с frontend
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // В production заменить на конкретные домены
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	s := &Server{
		router: router,
		db:     db,
		config: config,
	}

	s.setupRoutes(deps)

	return s
}

// setupRoutes настраивает маршруты приложения
func (s *Server) setupRoutes(deps Dependencies) {
	// Создаем middleware аутентификации (JWT + API Key)
	authMW := middleware.AuthMiddleware(deps.JWTManager, deps.ApiKeyService)

	// Health check endpoint
	s.router.GET("/health", s.healthCheck)

	// Prometheus metrics endpoint. Открыт без auth — потребляет
	// scrape'ом Prometheus/Grafana внутри dev-stack; в проде закрывается
	// сетевыми правилами (отдельный listener / firewall), как и /health.
	// Никаких user-данных не отдаёт — только счётчики и гистограммы из
	// internal/metrics, internal/service/*_service.go, sandbox/, domain/events/.
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger документация
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 routes
	api := s.router.Group("/api/v1")
	{
		// Auth routes (публичные)
		auth := api.Group("/auth")
		{
			auth.POST("/register", deps.AuthHandler.Register)
			auth.POST("/login", deps.AuthHandler.Login)
			auth.POST("/refresh", deps.AuthHandler.Refresh)

			// Защищенные маршруты (JWT + API Key)
			authProtected := auth.Group("")
			authProtected.Use(authMW)
			{
				authProtected.GET("/me", deps.AuthHandler.Me)
				authProtected.POST("/logout", deps.AuthHandler.Logout)

				// API Keys management (доступно любому авторизованному пользователю)
				authProtected.POST("/api-keys", deps.ApiKeyHandler.Create)
				authProtected.GET("/api-keys", deps.ApiKeyHandler.List)
				authProtected.GET("/api-keys/mcp-config", deps.ApiKeyHandler.GetMCPConfig)
				authProtected.POST("/api-keys/:id/revoke", deps.ApiKeyHandler.Revoke)
				authProtected.DELETE("/api-keys/:id", deps.ApiKeyHandler.Delete)
			}
		}

		// /me/* — канон путей для глобальных настроек пользователя (Sprint 13.5)
		me := api.Group("/me")
		me.Use(authMW)
		{
			me.GET("/llm-credentials", deps.UserLlmCredentialHandler.Get)
			me.PATCH("/llm-credentials", deps.LlmCredentialsPatchRL.Handler(), deps.UserLlmCredentialHandler.Patch)
		}

		// Phase 5 — user secrets (/me/secrets).
		if deps.UserSecretHandler != nil {
			meSecrets := api.Group("/me/secrets")
			meSecrets.Use(authMW)
			{
				meSecrets.GET("", deps.UserSecretHandler.List)
				meSecrets.POST("", deps.UserSecretHandler.Set)
				meSecrets.DELETE("/:secret_id", deps.UserSecretHandler.Delete)
			}
		}

		// Phase 4 §4.2 — /me/agents — user-level агенты.
		if deps.AgentMyHandler != nil {
			meAgents := api.Group("/me/agents")
			meAgents.Use(authMW)
			{
				meAgents.GET("", deps.AgentMyHandler.List)
				meAgents.GET("/:id", deps.AgentMyHandler.Get)
				meAgents.PUT("/:id", deps.AgentMyHandler.Update)
			}
			// Вкладка настроек «Ассистент»: получить (и при отсутствии спровижить)
			// своего ассистента без знания его id.
			api.GET("/me/assistant", authMW, deps.AgentMyHandler.GetAssistant)
		}

		// Каталог tool_definitions (тот же auth, что у /projects)
		api.GET("/tool-definitions", authMW, deps.ToolDefinitionHandler.List)

		// Sprint 16.C — Hermes toolsets каталог (UI dropdown).
		if deps.HermesHandler != nil {
			api.GET("/hermes/toolsets", authMW, deps.HermesHandler.ListToolsets)
		}

		// Agents settings (Sprint 15.23)
		if deps.AgentSettingsHandler != nil {
			agents := api.Group("/agents")
			agents.Use(authMW)
			{
				agents.GET("/:id/settings", deps.AgentSettingsHandler.Get)
				agents.PUT("/:id/settings", deps.AgentSettingsHandler.Update)
			}
		}

		// Agents v2 — Sprint 17 / Sprint 5F.3 — CRUD + секреты для реестра агентов v2.
		// Admin-only: проектные агенты управляются через этот API; без AdminOnly — IDOR.
		if deps.AgentV2Handler != nil {
			agentsV2 := api.Group("/agents")
			agentsV2.Use(authMW, middleware.AdminOnlyMiddleware())
			{
				agentsV2.GET("", deps.AgentV2Handler.List)
				agentsV2.GET("/:id", deps.AgentV2Handler.Get)
				agentsV2.POST("", deps.AgentV2Handler.Create)
				agentsV2.PUT("/:id", deps.AgentV2Handler.Update)
				agentsV2.DELETE("/:id", deps.AgentV2Handler.Delete)
				agentsV2.POST("/:id/secrets", deps.AgentV2Handler.SetSecret)
				agentsV2.DELETE("/secrets/:secret_id", deps.AgentV2Handler.DeleteSecret)
			}
		}

		// LLM providers — admin-only CRUD (Sprint 15.B5)
		if deps.LLMProviderHandler != nil {
			llmp := api.Group("/llm-providers")
			llmp.Use(authMW)
			{
				llmp.GET("", deps.LLMProviderHandler.List)
				llmp.POST("", deps.LLMProviderHandler.Create)
				llmp.POST("/test-connection", deps.LLMProviderHandler.TestConnection)
				llmp.POST("/:id/health-check", deps.LLMProviderHandler.HealthCheck)
				llmp.PUT("/:id", deps.LLMProviderHandler.Update)
				llmp.DELETE("/:id", deps.LLMProviderHandler.Delete)
			}
		}

		// Claude Code OAuth (Sprint 15.12)
		if deps.ClaudeCodeAuthHandler != nil {
			cc := api.Group("/claude-code/auth")
			cc.Use(authMW)
			{
				cc.POST("/init", deps.ClaudeCodeAuthHandler.Init)
				cc.POST("/callback", deps.ClaudeCodeAuthHandler.Callback)
				cc.PUT("/manual-token", deps.ClaudeCodeAuthHandler.ManualToken)
				cc.GET("/status", deps.ClaudeCodeAuthHandler.Status)
				cc.DELETE("", deps.ClaudeCodeAuthHandler.Revoke)
			}
		}

		// Antigravity OAuth
		if deps.AntigravityAuthHandler != nil {
			ag := api.Group("/antigravity/auth")
			ag.Use(authMW)
			{
				ag.POST("/init", deps.AntigravityAuthHandler.Init)
				ag.POST("/callback", deps.AntigravityAuthHandler.Callback)
				ag.PUT("/manual-token", deps.AntigravityAuthHandler.ManualToken)
				ag.GET("/status", deps.AntigravityAuthHandler.Status)
				ag.DELETE("", deps.AntigravityAuthHandler.Revoke)
			}
		}

		// UI Refactoring Stage 3a — git OAuth интеграции.
		if deps.GitIntegrationHandler != nil {
			// Публичные GET /callback — браузерный redirect от провайдера.
			// Auth не требуется: state-токен привязан к user в Init и идентифицирует его.
			api.GET("/integrations/github/auth/callback", deps.GitIntegrationHandler.BrowserCallbackGitHub)
			api.GET("/integrations/gitlab/auth/callback", deps.GitIntegrationHandler.BrowserCallbackGitLab)

			gh := api.Group("/integrations/github/auth")
			gh.Use(authMW)
			{
				gh.POST("/init", deps.GitIntegrationHandler.InitGitHub)
				gh.POST("/callback", deps.GitIntegrationHandler.CallbackGitHub)
				gh.GET("/status", deps.GitIntegrationHandler.StatusGitHub)
				gh.DELETE("/revoke", deps.GitIntegrationHandler.RevokeGitHub)
			}
			gl := api.Group("/integrations/gitlab/auth")
			gl.Use(authMW)
			{
				gl.POST("/init", deps.GitIntegrationHandler.InitGitLab)
				gl.POST("/callback", deps.GitIntegrationHandler.CallbackGitLab)
				gl.GET("/status", deps.GitIntegrationHandler.StatusGitLab)
				gl.DELETE("/revoke", deps.GitIntegrationHandler.RevokeGitLab)
			}

			// Мульти-аккаунт: список всех подключённых OAuth-аккаунтов + disconnect по id.
			accounts := api.Group("/integrations/accounts")
			accounts.Use(authMW)
			{
				accounts.GET("", deps.GitIntegrationHandler.ListAccounts)
				accounts.DELETE("/:id", deps.GitIntegrationHandler.RevokeAccount)
			}

			repos := api.Group("/integrations/:provider/repos")
			repos.Use(authMW)
			{
				repos.GET("", deps.GitIntegrationHandler.ListRepositories)
				repos.POST("", deps.GitIntegrationHandler.CreateRepository)
			}
		}

		// Projects (авторизованный пользователь)
		projects := api.Group("/projects")
		projects.Use(authMW)
		{
			projects.POST("", deps.ProjectHandler.Create)
			projects.GET("", deps.ProjectHandler.List)

			// WebSocket эндпоинт проекта (Sprint 7)
			// Должен быть ДО /:id, чтобы Gin не сматчил "ws" как :id
			projects.GET("/:id/ws", deps.WebSocketHandler.Connect)

			// Вложенный ресурс team — до /:id, иначе Gin сопоставит "team" как :id
			projects.GET("/:id/team", deps.TeamHandler.GetByProjectID)
			projects.PUT("/:id/team", deps.TeamHandler.Update)
			projects.PATCH("/:id/team/agents/:agentId", deps.TeamHandler.PatchAgent)
			projects.DELETE("/:id/team/agents/:agentId", deps.TeamHandler.DeleteAgent)
			projects.GET("/:id/teams", deps.TeamHandler.ListByProjectID)
			projects.POST("/:id/teams", deps.TeamHandler.Create)
			projects.POST("/:id/teams/:teamId/agents", deps.TeamHandler.CreateAgent)
			projects.DELETE("/:id/teams/:teamId", deps.TeamHandler.Delete)

			projects.POST("/:id/tasks", deps.TaskHandler.Create)
			projects.GET("/:id/tasks", deps.TaskHandler.List)

			// Регулярные (cron) задачи проекта.
			if deps.ScheduledTaskHandler != nil {
				projects.POST("/:id/scheduled-tasks", deps.ScheduledTaskHandler.Create)
				projects.GET("/:id/scheduled-tasks", deps.ScheduledTaskHandler.List)
				projects.PUT("/:id/scheduled-tasks/:scheduleId", deps.ScheduledTaskHandler.Update)
				projects.DELETE("/:id/scheduled-tasks/:scheduleId", deps.ScheduledTaskHandler.Delete)
			}

			// Энхансер проекта: конфиг, прогоны анализа, предложения изменений.
			if deps.EnhancerHandler != nil {
				projects.GET("/:id/enhancer", deps.EnhancerHandler.GetConfig)
				projects.PUT("/:id/enhancer", deps.EnhancerHandler.UpdateConfig)
				projects.POST("/:id/enhancer/run", deps.EnhancerHandler.RunNow)
				projects.GET("/:id/enhancer/runs", deps.EnhancerHandler.ListRuns)
				projects.GET("/:id/enhancer/runs/:runId/changes", deps.EnhancerHandler.ListRunChanges)
				projects.POST("/:id/enhancer/changes/:changeId/apply", deps.EnhancerHandler.ApplyChange)
				projects.POST("/:id/enhancer/changes/:changeId/reject", deps.EnhancerHandler.RejectChange)
				projects.POST("/:id/enhancer/changes/:changeId/rollback", deps.EnhancerHandler.RollbackChange)
			}

			// Разведчик проекта: конфиг агента сбора контекста (sandbox, на подписке) + прогоны.
			if deps.ScoutHandler != nil {
				projects.GET("/:id/scout", deps.ScoutHandler.GetConfig)
				projects.PUT("/:id/scout", deps.ScoutHandler.UpdateConfig)
				projects.POST("/:id/scout/run", deps.ScoutHandler.Dispatch)
				projects.GET("/:id/scout/runs", deps.ScoutHandler.ListRuns)
				projects.GET("/:id/scout/runs/:runId", deps.ScoutHandler.GetRun)
			}

			// Sprint 22: эфемерные сервис-сайдкары проекта (postgres для тестов с БД).
			if deps.SandboxServiceHandler != nil {
				projects.GET("/:id/sandbox-services", deps.SandboxServiceHandler.List)
				projects.PUT("/:id/sandbox-services", deps.SandboxServiceHandler.Upsert)
				projects.DELETE("/:id/sandbox-services/:serviceId", deps.SandboxServiceHandler.Delete)
			}

			// Внешние MCP-серверы ассистента проекта (remote http/sse).
			if deps.AssistantMCPHandler != nil {
				projects.GET("/:id/assistant/mcp-servers", deps.AssistantMCPHandler.List)
				projects.POST("/:id/assistant/mcp-servers", deps.AssistantMCPHandler.Create)
				projects.PUT("/:id/assistant/mcp-servers/:serverId", deps.AssistantMCPHandler.Update)
				projects.DELETE("/:id/assistant/mcp-servers/:serverId", deps.AssistantMCPHandler.Delete)
			}

			// Мульти-репо: управление git-репозиториями проекта.
			projects.GET("/:id/repositories", deps.ProjectHandler.ListRepositories)
			projects.POST("/:id/repositories", deps.ProjectHandler.AddRepository)
			projects.PUT("/:id/repositories/:repoId", deps.ProjectHandler.UpdateRepository)
			projects.DELETE("/:id/repositories/:repoId", deps.ProjectHandler.RemoveRepository)

			// «Инъекция env-файлов» уровня репозитория (несколько файлов на репо).
			if deps.RepositoryEnvFileHandler != nil {
				projects.GET("/:id/repositories/:repoId/env-files", deps.RepositoryEnvFileHandler.List)
				projects.POST("/:id/repositories/:repoId/env-files", deps.RepositoryEnvFileHandler.Create)
				projects.PUT("/:id/repositories/:repoId/env-files/:fileId", deps.RepositoryEnvFileHandler.Update)
				projects.DELETE("/:id/repositories/:repoId/env-files/:fileId", deps.RepositoryEnvFileHandler.Delete)
			}

			projects.GET("/:id", deps.ProjectHandler.GetByID)
			projects.PUT("/:id", deps.ProjectHandler.Update)
			projects.DELETE("/:id", deps.ProjectHandler.Delete)
			projects.POST("/:id/reindex", deps.ProjectHandler.Reindex)
			projects.POST("/:id/conversations", deps.ConversationHandler.Create)
			projects.GET("/:id/conversations", deps.ConversationHandler.List)

			// Phase 5 — project secrets.
			if deps.ProjectSecretHandler != nil {
				projects.GET("/:id/secrets", deps.ProjectSecretHandler.List)
				projects.POST("/:id/secrets", deps.ProjectSecretHandler.Set)
				projects.DELETE("/:id/secrets/:secret_id", deps.ProjectSecretHandler.Delete)
			}
		}

		tasks := api.Group("/tasks")
		tasks.Use(authMW)
		{
			tasks.GET("/:id", deps.TaskHandler.GetByID)
			tasks.PUT("/:id", deps.TaskHandler.Update)
			tasks.DELETE("/:id", deps.TaskHandler.Delete)
			tasks.POST("/:id/pause", deps.TaskHandler.Pause)
			tasks.POST("/:id/cancel", deps.TaskHandler.Cancel)
			tasks.POST("/:id/resume", deps.TaskHandler.Resume)
			tasks.POST("/:id/correct", deps.TaskHandler.Correct)
			tasks.GET("/:id/messages", deps.TaskHandler.ListMessages)
			tasks.POST("/:id/messages", deps.TaskHandler.AddMessage)

			// Sprint 17 / Orchestration v2 — read-only API для UI (DAG / Router timeline).
			if deps.OrchestrationV2Handler != nil {
				tasks.GET("/:id/artifacts", deps.OrchestrationV2Handler.ListArtifacts)
				tasks.GET("/:id/artifacts/:artifactId", deps.OrchestrationV2Handler.GetArtifact)
				tasks.GET("/:id/router-decisions", deps.OrchestrationV2Handler.ListRouterDecisions)
				tasks.GET("/:id/events", deps.OrchestrationV2Handler.ListTaskEvents)
			}
		}

		conversations := api.Group("/conversations")
		conversations.Use(authMW)
		{
			conversations.GET("/:id", deps.ConversationHandler.GetByID)
			conversations.POST("/:id/messages", deps.ConversationHandler.SendMessage)
			conversations.GET("/:id/messages", deps.ConversationHandler.GetHistory)
			conversations.DELETE("/:id", deps.ConversationHandler.Delete)
		}

		// Sprint 17 / Orchestration v2 — debug-эндпоинт для worktrees.
		if deps.OrchestrationV2Handler != nil {
			worktrees := api.Group("/worktrees")
			worktrees.Use(authMW)
			{
				worktrees.GET("", deps.OrchestrationV2Handler.ListWorktrees)
				// Sprint 17 / 6.3 — manual unstick. Admin-only гард внутри handler'а
				// (а не через AdminOnlyMiddleware), чтобы 401 имел приоритет над 403:
				// с middleware'м неавторизованный пользователь видел бы "forbidden"
				// вместо "unauthorized", и фронт по 403 не вышибал бы login flow.
				worktrees.POST("/:id/release", deps.OrchestrationV2Handler.ReleaseWorktree)
			}
		}

		// LLM routes (admin only except models)
		llmGroup := api.Group("/llm")
		llmGroup.Use(authMW)
		{
			llmGroup.GET("/models", deps.LLMHandler.ListModels)

			adminLLMGroup := llmGroup.Group("")
			adminLLMGroup.Use(middleware.AdminOnlyMiddleware())
			{
				adminLLMGroup.POST("/chat", deps.LLMHandler.Chat)
				adminLLMGroup.GET("/logs", deps.LLMHandler.ListLogs)
			}
		}

		// Prompts routes (read for all authenticated users, write for admin only)
		promptsGroup := api.Group("/prompts")
		promptsGroup.Use(authMW)
		{
			promptsGroup.GET("", deps.PromptHandler.List)
			promptsGroup.GET("/:id", deps.PromptHandler.GetByID)

			adminPrompts := promptsGroup.Group("")
			adminPrompts.Use(middleware.AdminOnlyMiddleware())
			{
				adminPrompts.POST("", deps.PromptHandler.Create)
				adminPrompts.PUT("/:id", deps.PromptHandler.Update)
				adminPrompts.DELETE("/:id", deps.PromptHandler.Delete)
			}
		}

		// Workflow routes (admin only)
		workflowsGroup := api.Group("/workflows")
		workflowsGroup.Use(authMW)
		workflowsGroup.Use(middleware.AdminOnlyMiddleware())
		{
			workflowsGroup.GET("", deps.WorkflowHandler.List)
			workflowsGroup.POST("/:name/start", deps.WorkflowHandler.Start)
		}

		// Execution routes (admin only)
		executionsGroup := api.Group("/executions")
		executionsGroup.Use(authMW)
		executionsGroup.Use(middleware.AdminOnlyMiddleware())
		{
			executionsGroup.GET("", deps.WorkflowHandler.ListExecutions)
			executionsGroup.GET("/:id", deps.WorkflowHandler.GetExecution)
			executionsGroup.GET("/:id/steps", deps.WorkflowHandler.GetExecutionSteps)
		}

		// Webhook management routes
		webhooksGroup := api.Group("/webhooks")
		webhooksGroup.Use(authMW)
		{
			webhooksGroup.POST("", deps.WebhookHandler.Create)
			webhooksGroup.GET("", deps.WebhookHandler.List)
			webhooksGroup.GET("/:id", deps.WebhookHandler.GetByID)
			webhooksGroup.PUT("/:id", deps.WebhookHandler.Update)
			webhooksGroup.DELETE("/:id", deps.WebhookHandler.Delete)
			webhooksGroup.GET("/:id/logs", deps.WebhookHandler.GetLogs)
		}

		// Phase 1 §1.4 — admin API для дефолтных промптов ролей агентов.
		if deps.AgentRolePromptHandler != nil {
			rolePrompts := api.Group("/admin/agent-role-prompts")
			rolePrompts.Use(authMW)
			rolePrompts.Use(middleware.AdminOnlyMiddleware())
			{
				rolePrompts.GET("", deps.AgentRolePromptHandler.List)
				rolePrompts.GET("/:role", deps.AgentRolePromptHandler.GetByRole)
				rolePrompts.PUT("/:role", deps.AgentRolePromptHandler.Update)
			}
		}

		// Phase 5 §5.6.1 — admin CRUD для реестра MCP-серверов.
		if deps.MCPServerRegistryHandler != nil {
			mcpServers := api.Group("/admin/mcp-servers")
			mcpServers.Use(authMW)
			mcpServers.Use(middleware.AdminOnlyMiddleware())
			{
				mcpServers.GET("", deps.MCPServerRegistryHandler.List)
				mcpServers.GET("/:id", deps.MCPServerRegistryHandler.Get)
				mcpServers.POST("", deps.MCPServerRegistryHandler.Create)
				mcpServers.PUT("/:id", deps.MCPServerRegistryHandler.Update)
				mcpServers.DELETE("/:id", deps.MCPServerRegistryHandler.Delete)
			}
		}

		// Dynamic Team Types endpoints
		api.GET("/team-types", authMW, deps.TeamHandler.ListTeamTypes)
		adminTeamTypes := api.Group("/admin/team-types")
		adminTeamTypes.Use(authMW)
		adminTeamTypes.Use(middleware.AdminOnlyMiddleware())
		{
			adminTeamTypes.POST("", deps.TeamHandler.CreateTeamType)
			adminTeamTypes.DELETE("/:code", deps.TeamHandler.DeleteTeamType)
		}

		// Sprint 21 — глобальный ассистент (правая панель).
		// scope=user, без project_id; идёт собственной группой /assistant/*.
		// Все эндпоинты требуют auth.
		if deps.AssistantHandler != nil {
			assistant := api.Group("/assistant")
			assistant.Use(authMW)
			{
				// Tasks-tab правой панели — live-список in-progress задач юзера.
				assistant.GET("/active-tasks", deps.AssistantHandler.ListActiveTasks)

				// Статус конфигурации ассистента (есть ли ключи у пользователя).
				assistant.GET("/status", deps.AssistantHandler.GetStatus)

				// Распознавание аудио
				assistant.POST("/transcribe", deps.AssistantHandler.Transcribe)

				// Сессии: CRUD + история + send + confirm.
				assistant.POST("/sessions", deps.AssistantHandler.CreateSession)
				assistant.GET("/sessions", deps.AssistantHandler.ListSessions)
				assistant.GET("/sessions/:id", deps.AssistantHandler.GetSession)
				assistant.DELETE("/sessions/:id", deps.AssistantHandler.ArchiveSession)
				assistant.GET("/sessions/:id/messages", deps.AssistantHandler.GetMessages)
				assistant.POST("/sessions/:id/messages", deps.AssistantHandler.SendMessage)
				assistant.POST("/sessions/:id/confirm", deps.AssistantHandler.ConfirmToolCall)
				assistant.POST("/sessions/:id/stop", deps.AssistantHandler.StopRun)
			}
		}

		// Public webhook trigger endpoint (NO AUTH)
		// Формат: POST /api/v1/hooks/{webhook_name}
		hooksGroup := api.Group("/hooks")
		{
			hooksGroup.POST("/:name", deps.WebhookHandler.Trigger)
			hooksGroup.GET("/:name", deps.WebhookHandler.Trigger) // Поддержка GET для простых интеграций
		}
	}
}

// healthCheck обрабатывает запрос проверки здоровья сервера
func (s *Server) healthCheck(c *gin.Context) {
	// Проверяем подключение к БД
	sqlDB, err := s.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "database connection failed",
		})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "database ping failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	fmt.Printf("Server starting on %s\n", addr)
	return srv.ListenAndServe()
}

// Shutdown gracefully останавливает сервер
func (s *Server) Shutdown(ctx context.Context) error {
	// Здесь можно добавить логику graceful shutdown
	return nil
}

func customLogFormatter(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}

	rawPath := param.Path
	u, err := url.ParseRequestURI(rawPath)
	if err == nil {
		q := u.Query()
		if len(q) > 0 {
			for k := range q {
				q.Set(k, "REDACTED")
			}
			u.RawQuery = q.Encode()
			rawPath = u.String()
		}
	}

	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		rawPath,
		param.ErrorMessage,
	)
}
