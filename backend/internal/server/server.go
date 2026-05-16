package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/devteam/backend/internal/handler"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
	"github.com/devteam/backend/pkg/jwt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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
	WorkflowHandler       *handler.WorkflowHandler
	WebhookHandler        *handler.WebhookHandler
	JWTManager            *jwt.Manager
	ApiKeyService         service.ApiKeyService
	WebSocketHandler      *ws.WebSocketHandler

	UserLlmCredentialHandler *handler.UserLlmCredentialHandler
	LlmCredentialsPatchRL    *middleware.LlmCredentialsPatchRateLimiter

	ClaudeCodeAuthHandler *handler.ClaudeCodeAuthHandler

	// UI Refactoring Stage 3a — git OAuth интеграции (GitHub / GitLab / BYO GitLab).
	GitIntegrationHandler *handler.GitIntegrationHandler

	// Sprint 15.23 — per-agent settings.
	AgentSettingsHandler *handler.AgentSettingsHandler

	// Sprint 17 / Sprint 5F.3 — CRUD + секреты для реестра агентов v2.
	AgentV2Handler *handler.AgentV2Handler

	// Sprint 17 / Orchestration v2 — read-only API для DAG / Router timeline / Worktrees.
	OrchestrationV2Handler *handler.OrchestrationV2Handler

	// Sprint 15.B5 — CRUD над llm_providers (admin-only).
	LLMProviderHandler *handler.LLMProviderHandler

	// Sprint 16.C — Hermes-каталог (toolsets) для UI dropdown'а.
	HermesHandler *handler.HermesHandler
}

// New создает новый экземпляр сервера
func New(db *gorm.DB, config ServerConfig, deps Dependencies) *Server {
	// Устанавливаем режим Gin в зависимости от окружения
	if gin.Mode() == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

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
		// Параллель MCP-инструментам tools_agents_v2.go; используется фронтендом.
		if deps.AgentV2Handler != nil {
			agentsV2 := api.Group("/agents")
			agentsV2.Use(authMW)
			{
				agentsV2.GET("", deps.AgentV2Handler.List)
				agentsV2.GET("/:id", deps.AgentV2Handler.Get)
				agentsV2.POST("", deps.AgentV2Handler.Create)
				agentsV2.PUT("/:id", deps.AgentV2Handler.Update)
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
				cc.GET("/status", deps.ClaudeCodeAuthHandler.Status)
				cc.DELETE("", deps.ClaudeCodeAuthHandler.Revoke)
			}
		}

		// UI Refactoring Stage 3a — git OAuth интеграции.
		if deps.GitIntegrationHandler != nil {
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

			projects.POST("/:id/tasks", deps.TaskHandler.Create)
			projects.GET("/:id/tasks", deps.TaskHandler.List)

			projects.GET("/:id", deps.ProjectHandler.GetByID)
			projects.PUT("/:id", deps.ProjectHandler.Update)
			projects.DELETE("/:id", deps.ProjectHandler.Delete)
			projects.POST("/:id/reindex", deps.ProjectHandler.Reindex)
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
			}
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

		// LLM routes (admin only)
		llmGroup := api.Group("/llm")
		llmGroup.Use(authMW)
		llmGroup.Use(middleware.AdminOnlyMiddleware())
		{
			llmGroup.POST("/chat", deps.LLMHandler.Chat)
			llmGroup.GET("/logs", deps.LLMHandler.ListLogs)
		}

		// Prompts routes (admin only)
		promptsGroup := api.Group("/prompts")
		promptsGroup.Use(authMW)
		promptsGroup.Use(middleware.AdminOnlyMiddleware())
		{
			promptsGroup.POST("", deps.PromptHandler.Create)
			promptsGroup.GET("", deps.PromptHandler.List)
			promptsGroup.GET("/:id", deps.PromptHandler.GetByID)
			promptsGroup.PUT("/:id", deps.PromptHandler.Update)
			promptsGroup.DELETE("/:id", deps.PromptHandler.Delete)
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

		// Webhook management routes (admin only)
		webhooksGroup := api.Group("/webhooks")
		webhooksGroup.Use(authMW)
		webhooksGroup.Use(middleware.AdminOnlyMiddleware())
		{
			webhooksGroup.POST("", deps.WebhookHandler.Create)
			webhooksGroup.GET("", deps.WebhookHandler.List)
			webhooksGroup.GET("/:id", deps.WebhookHandler.GetByID)
			webhooksGroup.PUT("/:id", deps.WebhookHandler.Update)
			webhooksGroup.DELETE("/:id", deps.WebhookHandler.Delete)
			webhooksGroup.GET("/:id/logs", deps.WebhookHandler.GetLogs)
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
