package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devteam/backend/docs"
	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/handler"
	"github.com/devteam/backend/internal/indexer"
	mcpserver "github.com/devteam/backend/internal/mcp"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/internal/server"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
	"github.com/devteam/backend/pkg/agentprompts"
	"github.com/devteam/backend/pkg/agentsloader"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/devteam/backend/pkg/jwt"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/devteam/backend/pkg/password"
	"github.com/devteam/backend/pkg/promptsloader"
	"github.com/devteam/backend/pkg/secrets"
	"github.com/devteam/backend/pkg/workflowloader"
	"github.com/docker/docker/client"
	"github.com/pressly/goose/v3"
	"log/slog"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Инициализируем документацию (для swagger)
func init() {
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
}

// @title           Backend API
// @version         1.0
// @description     Backend API с авторизацией на JWT токенах
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Long-lived API key for programmatic access. Format: wibe_<key>

// @securityDefinitions.oauth2.password OAuth2Password
// @tokenUrl /api/v1/auth/login

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var encryptor service.Encryptor = service.NoopEncryptor{}
	if len(cfg.Encryption.Key) == 32 {
		aesEnc, err := crypto.NewAESEncryptor(cfg.Encryption.Key)
		if err != nil {
			log.Fatalf("Failed to init AES encryptor: %v", err)
		}
		encryptor = aesEnc
	}

	// Подключаемся к базе данных
	db, err := initDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Запускаем миграции
	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Создаем админа если нужно
	userRepo := repository.NewUserRepository(db)
	if err := ensureAdmin(context.Background(), userRepo, cfg.Admin); err != nil {
		log.Printf("Failed to ensure admin user: %v", err)
	}

	// Инициализация зависимостей
	// Repositories
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	apiKeyRepo := repository.NewApiKeyRepository(db)
	promptRepo := repository.NewPromptRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	toolDefRepo := repository.NewToolDefinitionRepository(db)
	gitCredRepo := repository.NewGitCredentialRepository(db)
	txManager := repository.NewTransactionManager(db)
	workflowRepo := repository.NewWorkflowRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	taskMsgRepo := repository.NewTaskMessageRepository(db)
	llmRepo := repository.NewLLMRepository(db)
	llmModelRepo := repository.NewLLMModelRepository(db)

	// Загрузка промптов из файлов
	log.Println("Loading prompts from backend/prompts...")
	promptsLoader := promptsloader.New(promptRepo)
	if err := promptsLoader.LoadFromDir(context.Background(), "prompts"); err != nil {
		// Не падаем, если не смогли загрузить промпты (например, нет папки), но логируем ошибку
		log.Printf("Failed to load prompts: %v", err)
	} else {
		log.Println("Prompts loaded successfully")
	}

	// Загрузка workflows и агентов
	log.Println("Loading workflows and agents...")
	wfLoader := workflowloader.New(workflowRepo, promptRepo, db)
	if err := wfLoader.LoadAgents(context.Background(), "agents"); err != nil {
		log.Printf("Failed to load agents: %v", err)
	}
	if err := wfLoader.LoadWorkflows(context.Background(), "workflows"); err != nil {
		log.Printf("Failed to load workflows: %v", err)
	}
	if err := wfLoader.LoadSchedules(context.Background(), "schedules"); err != nil {
		// Не падаем, если нет папки или ошибка, просто логируем
		log.Printf("Failed to load schedules: %v", err)
	}

	// Services
	modelCatalogService := service.NewModelCatalogService(llmModelRepo, cfg.LLM.OpenRouterAPIKey)

	// --- WebSocket & Event Bus (Sprint 7) ---
	// rootCtx для long-living компонентов (Hub, Bridge)
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	var wg sync.WaitGroup

	hub := ws.NewHub()
	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run(rootCtx)
	}()

	eventBus := events.NewInMemoryBus(events.NewPrometheusMetrics(), slog.Default())
	defer eventBus.Close()

	hubBridge := ws.NewHubBridge(
		eventBus,
		hub,
		secrets.NewScrubber(),
		slog.Default(),
		ws.NewPrometheusBridgeMetrics(),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		hubBridge.Run(rootCtx)
	}()

	jwtManager := jwt.NewManager(cfg.JWT.SecretKey, cfg.JWT.AccessTokenExpiry, cfg.JWT.RefreshTokenExpiry)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, jwtManager, eventBus)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo)
	promptService := service.NewPromptService(promptRepo)
	gitFactory := gitprovider.NewFactory()

	// --- Indexer (Sprint 9) ---
	syncRepo := repository.NewSyncStateRepository(db)
	vectorRepo := repository.NewVectorRepository(nil) // TODO: pass Weaviate client
	codeIndexer, _ := indexer.NewCodeIndexer(syncRepo, vectorRepo, nil, 4, slog.Default())

	projectService := service.NewProjectService(
		projectRepo,
		teamRepo,
		gitCredRepo,
		txManager,
		gitFactory,
		encryptor,
		eventBus,
		codeIndexer,
		cfg.Git.ImportDir,
	)
	toolDefinitionService := service.NewToolDefinitionService(toolDefRepo)
	teamService := service.NewTeamService(teamRepo, toolDefRepo)
	taskIndexer := indexer.NewTaskIndexer(taskRepo, taskMsgRepo, vectorRepo, slog.Default())
	taskService := service.NewTaskService(taskRepo, taskMsgRepo, projectService, teamService, txManager, eventBus, taskIndexer, slog.Default())

	// --- IndexerService координатор (Sprint 9.5) ---
	// Конструируем все зависимости даже если часть из них пока не активна
	// (vectordb.Client не сконфигурирован → используем NoopVectorDeleter).
	conversationRepo := repository.NewConversationRepository(db)
	conversationMsgRepo := repository.NewConversationMessageRepository(db)
	conversationIndexer, err := indexer.NewConversationIndexer(conversationRepo, conversationMsgRepo, vectorRepo, eventBus, slog.Default())
	if err != nil {
		log.Fatalf("failed to construct conversation indexer: %v", err)
	}
	indexerLocker := service.NewInMemoryLocker()
	indexerService := service.NewIndexerService(
		slog.Default(),
		service.NoopVectorDeleter{}, // TODO: заменить на *vectordb.Client когда он будет сконфигурирован
		codeIndexer,
		taskIndexer,
		conversationIndexer,
		projectService,
		syncRepo,
		indexerLocker,
	)
	// TODO(Sprint 9.5): мигрировать хуки 9.6/9.7/9.8 и ProjectService.Reindex на indexerService.
	// На текущий момент существующие хуки используют под-индексаторы напрямую (см. project_service.go,
	// task_service.go, conversation_service.go); IndexerService инстанцирован, но не подключен к ним.
	_ = indexerService

	// Запускаем первичную синхронизацию моделей (в фоне)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		log.Println("Starting initial OpenRouter model sync...")
		if err := modelCatalogService.SyncOpenRouterModels(ctx); err != nil {
			log.Printf("Initial model sync failed: %v", err)
		}
	}()

	// LLM
	llmFactory := factory.New()
	llmService := service.NewLLMService(llmFactory, cfg.LLM, llmRepo, llmModelRepo)
	llmHandler := handler.NewLLMHandler(llmService)

	// Workflow Engine
	workflowEngine := service.NewWorkflowEngine(workflowRepo, llmService)

	// Docker Client for Sandbox
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Warning: Failed to init Docker client (sandbox tasks will fail): %v", err)
	}

	// Agent Executors
	llmAgentExecutor := agent.NewLLMAgentExecutor(llmService)
	// Sandbox Runner (учитываем лимиты из конфига)
	sandboxLogAdapter := ws.NewSandboxLogAdapter(eventBus, secrets.NewScrubber())
	sandboxRunner := sandbox.NewDockerSandboxRunner(dockerCli, []string{
		"devteam/sandbox-claude:local",
		"devteam/sandbox-aider:local",
		"devteam/sandbox-hermes:local",
	}, sandbox.WithLogPublisher(sandboxLogAdapter))
	// Sprint 16: per-backend образа. claude-code/aider/custom уходят в default
	// (sandbox-claude), hermes — в свой образ. Если в будущем у aider будет
	// отдельный образ — добавляем в map здесь же без правок executor'а.
	sandboxAgentExecutor := agent.NewSandboxAgentExecutor(
		sandboxRunner,
		"devteam/sandbox-claude:local",
		map[string]string{
			string(models.CodeBackendHermes): "devteam/sandbox-hermes:local",
		},
	)

	// Orchestrator Components
	orchestratorPipeline := service.NewPipelineEngine(5)

	agentConfigCache, err := agentsloader.NewCache("agents", "prompts")
	if err != nil {
		log.Fatalf("agent YAML configs: %v", err)
	}
	if err := agentConfigCache.ValidateRequiredAgents(); err != nil {
		log.Fatalf("agent YAML validation: %v", err)
	}
	log.Println("Agent default configs: loaded and validated (backend/agents)")

	var pipelinePromptComposer service.PipelinePromptComposer
	if pc, err := agentprompts.NewComposer("prompts"); err != nil {
		log.Printf("Pipeline agent prompts (YAML) not active: %v", err)
	} else {
		pipelinePromptComposer = pc
		log.Println("Pipeline agent prompts: loaded base + role composition (backend/prompts)")
	}
	// В sandbox прокидываем только API-ключ: claude-code CLI 2.x использует свой
	// канонический endpoint, а наш LLMService — собственный (ANTHROPIC_BASE_URL с "/v1").
	// Пробрасывать BASE_URL в контейнер опасно: CLI собирает путь по-другому и фейлится 404.
	sandboxSecrets := map[string]string{
		sandbox.EnvAnthropicAPIKey: cfg.LLM.Anthropic.APIKey,
	}
	// Sprint 15.B/C — OAuth Claude Code subscription.
	// Создаём заранее (до orchestrator), чтобы динамический резолвер аутентификации sandbox-а
	// (Sprint 15.18) мог опираться на ClaudeCodeAuthService.
	claudeCodeSubRepo := repository.NewClaudeCodeSubscriptionRepository(db)
	claudeCodeOAuthProvider := service.NewClaudeCodeOAuthProvider(service.ClaudeCodeOAuthConfig{
		ClientID:      cfg.ClaudeCodeOAuth.ClientID,
		DeviceCodeURL: cfg.ClaudeCodeOAuth.DeviceCodeURL,
		TokenURL:      cfg.ClaudeCodeOAuth.TokenURL,
		RevokeURL:     cfg.ClaudeCodeOAuth.RevokeURL,
		Scopes:        cfg.ClaudeCodeOAuth.Scopes,
	})
	claudeCodeAuthSvc := service.NewClaudeCodeAuthService(claudeCodeSubRepo, encryptor, claudeCodeOAuthProvider)

	// User-per-credential service (нужен резолверу аутентификации sandbox).
	llmCredRepo := repository.NewUserLlmCredentialRepository(db)
	llmCredSvc := service.NewUserLlmCredentialService(llmCredRepo, txManager, encryptor, slog.Default())

	// Sprint 15.18 — динамический резолвер аутентификации sandbox (OAuth subscription / per-user creds / api key).
	sandboxAuthResolver := service.NewSandboxAuthEnvResolver(
		claudeCodeAuthSvc,
		llmCredSvc,
		cfg.LLM.Anthropic.APIKey,
		slog.Default(),
	)
	// Sprint 16.C — AgentSettingsService с полными зависимостями (MCP-реестр + secret-резолвер).
	// MCP-реестр нужен и для Claude (.mcp.json) и для Hermes (mcp.json).
	// DatabaseSecretResolver резолвит ${secret:<provider>} → user_llm_credentials по
	// project.UserID. Без него hermes-MCP-сервер с секрет-шаблоном получит явную
	// ошибку «secret resolver not configured» при сборке артефактов — лучше падение
	// чем тихий пустой токен в HERMES_MCP_*_TOKEN.
	mcpRegistryLookup := service.NewMCPRepositoryLookupAdapter(repository.NewMCPServerRegistryRepository(db))
	secretResolver := service.NewDatabaseSecretResolver(db, encryptor)
	agentSettingsSvc := service.NewAgentSettingsServiceWithDeps(mcpRegistryLookup, secretResolver)

	// Sprint 15.M7 — функциональная опция вместо type-assertion-шима WithSandboxAuthResolver.
	orchestratorContextBuilder := service.NewContextBuilderFull(
		encryptor, pipelinePromptComposer, agentConfigCache, sandboxSecrets, taskMsgRepo,
		service.WithSandboxAuthResolverOption(sandboxAuthResolver),
		// Sprint 16.C — без этой опции AgentSettingsBundle никогда не доезжает
		// до sandbox-runner'а: hermes-config/skills/permission-mode становятся мёртвым кодом.
		service.WithAgentSettingsServiceOption(agentSettingsSvc),
	)

	llmProviderRepo := repository.NewLLMProviderRepository(db)
	llmProviderSvc := service.NewLLMProviderService(llmProviderRepo, encryptor)
	llmProviderHandler := handler.NewLLMProviderHandler(llmProviderSvc)

	taskControlBus := service.NewUserTaskControlBus()

	// Orchestrator Service
	orchestratorService := service.NewOrchestratorService(
		taskRepo,
		taskMsgRepo,
		workflowRepo,
		projectService,
		txManager,
		llmAgentExecutor,
		sandboxAgentExecutor,
		taskService,
		orchestratorPipeline,
		orchestratorContextBuilder,
		codeIndexer,
		sandboxRunner,
		taskControlBus,
		service.WithTeamRepository(teamRepo),
		service.WithPullRequestPublisher(service.NewGitPRPublisher(gitFactory, encryptor, slog.Default())),
	)

	// Запускаем оркестратор (очистка зомби-задач)
	if err := orchestratorService.Start(context.Background()); err != nil {
		log.Printf("Failed to start orchestrator: %v", err)
	}

	// Запускаем Workflow Worker в фоне (отключить: WORKFLOW_WORKER_ENABLED=false)
	ctxWorker, cancelWorker := context.WithCancel(context.Background())
	if cfg.WorkflowWorkerEnabled {
		go workflowEngine.RunWorker(ctxWorker)
	} else {
		log.Println("Workflow worker is disabled (WORKFLOW_WORKER_ENABLED=false); pending executions will not run until re-enabled")
	}

	// Scheduler: внутри cron свой жизненный цикл; останавливаем через Stop() при shutdown.
	scheduler := service.NewScheduler(workflowRepo, workflowEngine, modelCatalogService)
	if err := scheduler.Start(ctxWorker); err != nil {
		log.Printf("Failed to start scheduler: %v", err)
	}

	// Handlers
	authHandler := handler.NewAuthHandler(authService, jwtManager)
	apiKeyHandler := handler.NewApiKeyHandler(apiKeyService, &cfg.MCP)
	promptHandler := handler.NewPromptHandler(promptService)
	projectHandler := handler.NewProjectHandler(projectService)
	teamHandler := handler.NewTeamHandler(teamService, projectService)
	toolDefinitionHandler := handler.NewToolDefinitionHandler(toolDefinitionService)
	taskHandler := handler.NewTaskHandler(taskService, orchestratorService, taskControlBus)
	webhookPublicBase := fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	webhookHandler := handler.NewWebhookHandler(webhookRepo, workflowRepo, workflowEngine, webhookPublicBase)
	workflowHandler := handler.NewWorkflowHandler(workflowEngine)

	llmCredRL := middleware.NewLlmCredentialsPatchRateLimiter(30, time.Minute)
	llmCredHandler := handler.NewUserLlmCredentialHandler(llmCredSvc)

	claudeCodeAuthHandler := handler.NewClaudeCodeAuthHandler(claudeCodeAuthSvc)
	if cfg.ClaudeCodeOAuth.ClientID != "" {
		refresher := service.NewClaudeCodeTokenRefresher(claudeCodeSubRepo, claudeCodeAuthSvc, slog.Default())
		go refresher.Run(ctxWorker)
		log.Println("Claude Code token refresher: started")
	} else {
		log.Println("Claude Code OAuth: disabled (set CLAUDE_CODE_OAUTH_CLIENT_ID to enable)")
	}

	// WebSocket Handler
	wsHandler := ws.NewWebSocketHandler(hub, projectService, ws.HandlerConfig{
		AllowedOrigins:         cfg.WebSocket.AllowedOrigins,
		MaxConnsPerUserProject: cfg.WebSocket.MaxConnsPerUserProject,
		ReadBufferSize:         1024,
		WriteBufferSize:        1024,
	}, slog.Default())

	// Создаем и запускаем сервер
	srv := server.New(db, server.ServerConfig{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}, server.Dependencies{
		AuthHandler:           authHandler,
		ApiKeyHandler:         apiKeyHandler,
		LLMHandler:            llmHandler,
		PromptHandler:         promptHandler,
		ProjectHandler:        projectHandler,
		TeamHandler:           teamHandler,
		ToolDefinitionHandler: toolDefinitionHandler,
		TaskHandler:           taskHandler,
		WorkflowHandler:       workflowHandler,
		WebhookHandler:        webhookHandler,
		JWTManager:            jwtManager,
		ApiKeyService:         apiKeyService,
		WebSocketHandler:      wsHandler,

		UserLlmCredentialHandler: llmCredHandler,
		LlmCredentialsPatchRL:    llmCredRL,

		ClaudeCodeAuthHandler: claudeCodeAuthHandler,
		AgentSettingsHandler:  handler.NewAgentSettingsHandler(teamService),
		LLMProviderHandler:    llmProviderHandler,
		HermesHandler:         handler.NewHermesHandler(),
	})

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	// --- MCP-сервер (условный запуск) ---
	var mcpHTTPServer *http.Server

	if cfg.MCP.Enabled {
		mcpSrv := mcpserver.NewMCPServer(mcpserver.Dependencies{
			Config:                cfg.MCP,
			LLMService:            llmService,
			WorkflowEngine:        workflowEngine,
			PromptService:         promptService,
			ProjectService:        projectService,
			TeamService:           teamService,
			TaskService:           taskService,
			ToolDefinitionService: toolDefinitionService,
			OrchestratorSvc:       orchestratorService,
			ApiKeyService:         apiKeyService,
			ClaudeCodeAuthService: claudeCodeAuthSvc,
			MCPServerRegistryRepo: repository.NewMCPServerRegistryRepository(db),
			AgentSkillRepo:        repository.NewAgentSkillRepository(db),
		})

		mcpHandler := mcpserver.NewHTTPHandler(mcpSrv, apiKeyService)

		mcpAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.MCP.Port)

		// Оборачиваем MCP handler в ServeMux с health-эндпоинтом (для Docker и мониторинга)
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","service":"mcp"}`))
		})
		mux.Handle("/", mcpHandler) // всё остальное → MCP

		mcpHTTPServer = &http.Server{
			Addr:    mcpAddr,
			Handler: mux,
			// WriteTimeout=0: MCP StreamableHTTP использует SSE (long-lived connections).
			// Go http.Server.WriteTimeout покрывает ВЕСЬ ответ, не отдельные записи,
			// поэтому ограничение по времени сломает SSE-сессии.
			// Таймауты управляются на уровне MCP SDK.
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0,
		}

		go func() {
			log.Printf("MCP server starting on %s (public URL: %s)", mcpAddr, cfg.MCP.PublicURL)
			if err := mcpHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Failed to run MCP server: %v", err)
			}
		}()
	} else {
		log.Println("MCP server is disabled (set MCP_ENABLED=true to enable)")
	}

	// Ожидаем сигнал завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	// Общий таймаут на graceful shutdown всех серверов.
	// MCP shutdown обычно мгновенный; основной запас — для Gin-сервера с активными соединениями.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Останавливаем планировщик и воркер
	scheduler.Stop()
	cancelWorker()

	// Graceful shutdown MCP-сервера
	if mcpHTTPServer != nil {
		log.Println("Shutting down MCP server...")
		if err := mcpHTTPServer.Shutdown(ctx); err != nil {
			log.Printf("MCP server forced to shutdown: %v", err)
		}
	}

	// Graceful shutdown основного сервера
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Останавливаем Hub и Bridge
	rootCancel()
	wg.Wait()

	log.Println("All servers exited")
}

func runMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(sqlDB, "db/migrations"); err != nil {
		return err
	}

	return nil
}

func ensureAdmin(ctx context.Context, userRepo repository.UserRepository, cfg config.AdminConfig) error {
	if cfg.Email == "" || cfg.Password == "" {
		log.Println("Admin credentials not configured, skipping admin creation")
		return nil
	}

	_, err := userRepo.GetByEmail(ctx, cfg.Email)
	if err == nil {
		log.Println("Admin user already exists")
		return nil
	}

	log.Println("Creating admin user...")
	passwordHash, err := password.Hash(cfg.Password)
	if err != nil {
		return err
	}

	admin := &models.User{
		Email:        cfg.Email,
		PasswordHash: passwordHash,
		Role:         models.RoleAdmin,
	}

	return userRepo.Create(ctx, admin)
}

// initDatabase инициализирует подключение к базе данных
func initDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Проверяем подключение
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
