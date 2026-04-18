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

	"github.com/docker/docker/client"
	"github.com/pressly/goose/v3"
	"github.com/devteam/backend/docs"
	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/handler"
	mcpserver "github.com/devteam/backend/internal/mcp"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/internal/server"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/devteam/backend/pkg/jwt"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/devteam/backend/pkg/password"
	"github.com/devteam/backend/pkg/agentprompts"
	"github.com/devteam/backend/pkg/agentsloader"
	"github.com/devteam/backend/pkg/promptsloader"
	"github.com/devteam/backend/pkg/workflowloader"

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

	jwtManager := jwt.NewManager(cfg.JWT.SecretKey, cfg.JWT.AccessTokenExpiry, cfg.JWT.RefreshTokenExpiry)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, jwtManager)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo)
	promptService := service.NewPromptService(promptRepo)
	gitFactory := gitprovider.NewFactory()
	projectService := service.NewProjectService(
		projectRepo,
		teamRepo,
		gitCredRepo,
		txManager,
		gitFactory,
		encryptor,
		cfg.Git.ImportDir,
	)
	teamService := service.NewTeamService(teamRepo)
	taskService := service.NewTaskService(taskRepo, taskMsgRepo, projectService, teamService)

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
	sandboxRunner := sandbox.NewDockerSandboxRunner(dockerCli, []string{
		"devteam/sandbox-claude:latest",
		"devteam/sandbox-aider:latest",
	})
	sandboxAgentExecutor := agent.NewSandboxAgentExecutor(sandboxRunner, "devteam/sandbox-claude:latest")

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
	orchestratorContextBuilder := service.NewContextBuilder(encryptor, pipelinePromptComposer, agentConfigCache)

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
		sandboxRunner,
		taskControlBus,
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
	taskHandler := handler.NewTaskHandler(taskService, orchestratorService, taskControlBus)
	webhookPublicBase := fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	webhookHandler := handler.NewWebhookHandler(webhookRepo, workflowRepo, workflowEngine, webhookPublicBase)
	workflowHandler := handler.NewWorkflowHandler(workflowEngine)

	// Создаем и запускаем сервер
	srv := server.New(db, server.ServerConfig{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}, server.Dependencies{
		AuthHandler:     authHandler,
		ApiKeyHandler:   apiKeyHandler,
		LLMHandler:      llmHandler,
		PromptHandler:   promptHandler,
		ProjectHandler:  projectHandler,
		TeamHandler:     teamHandler,
		TaskHandler:     taskHandler,
		WorkflowHandler: workflowHandler,
		WebhookHandler:  webhookHandler,
		JWTManager:      jwtManager,
		ApiKeyService:   apiKeyService,
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
			Config:          cfg.MCP,
			LLMService:      llmService,
			WorkflowEngine:  workflowEngine,
			PromptService:   promptService,
			ProjectService:  projectService,
			TeamService:     teamService,
			TaskService:     taskService,
			OrchestratorSvc: orchestratorService,
			ApiKeyService:   apiKeyService,
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
