package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/database"
	"github.com/devteam/backend/internal/handler"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/logging"
	mcpserver "github.com/devteam/backend/internal/mcp"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/internal/seed"
	"github.com/devteam/backend/internal/server"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
	"github.com/devteam/backend/pkg/agentprompts"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/devteam/backend/pkg/jwt"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/devteam/backend/pkg/password"
	"github.com/devteam/backend/pkg/promptsloader"
	"github.com/devteam/backend/pkg/secrets"
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/devteam/backend/pkg/workflowloader"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Run собирает все зависимости и запускает процесс в заданной роли. Swagger-аннотации и
// docs.SwaggerInfo живут в cmd/api/main.go (бинарь, который читает swag -g). Зависимости и
// фоновые компоненты строятся одинаково для всех ролей; роль гейтит лишь точки старта
// (HTTP/MCP, пулы воркеров, leader-tasks) — см. role.go.
func Run(role Role) {
	log.Printf("Starting in role=%s", role)
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

	// Миграции. В multi-instance деплое AUTO_MIGRATE=false и накат через отдельный
	// one-shot job (cmd/migrate) — иначе goose.Up бежит на каждой реплике одновременно.
	if cfg.AutoMigrate {
		if err := database.RunMigrations(db); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		log.Println("AUTO_MIGRATE=false: skipping migrations on boot (expecting external cmd/migrate job)")
	}

	// Создаем админа если нужно
	userRepo := repository.NewUserRepository(db)
	if err := ensureAdmin(context.Background(), userRepo, cfg.Admin); err != nil {
		log.Printf("Failed to ensure admin user: %v", err)
	}

	// instanceID — уникальный идентификатор процесса для кросс-нодовой координации
	// (WS fan-out + leader election). Один на процесс.
	instanceID := uuid.NewString()

	// Leader election: только один инстанс выполняет процессы-синглтоны (cron-планировщик,
	// токен-рефрешеры, retention, stale-recovery, workflow-worker), чтобы при N репликах
	// они не дублировались (см. service.LeaderElector). Задачи регистрируются через
	// OnLeader ниже по ходу сборки, elector.Run запускается после неё. Step/agent-воркеры
	// под лидера НЕ заводим — они claim-safe (task_events + SKIP LOCKED) и должны работать
	// на всех репликах.
	leaderElector := service.NewLeaderElector(service.NewDBLeaseStore(db), "singletons", instanceID, slog.Default())

	// Redis — единый клиент на процесс для: low-latency wakeup воркеров, кросс-нодового
	// WS fan-out (ws.ClusterBridge) и shared эфемерных сторов (locker, git-oauth-state,
	// device-code, rate-limit). Строим рано, чтобы передать сторам ниже по сборке. При
	// cfg.Redis.Required (по умолчанию в production) отсутствие/недоступность Redis фатальны:
	// multi-instance деплой без него теряет fan-out и shared-состояние. nil → single-instance
	// режим (in-memory сторы, polling-only воркеры) — поведение прежнее.
	var redisClient *redis.Client
	if redisURL := cfg.Redis.URL; redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			if cfg.Redis.Required {
				log.Fatalf("REDIS_URL parse failed and REDIS_REQUIRED is set: %v", err)
			}
			log.Printf("REDIS_URL parse failed: %v — single-instance mode (in-memory stores)", err)
		} else {
			rc := redis.NewClient(opts)
			if pingErr := rc.Ping(context.Background()).Err(); pingErr != nil {
				if cfg.Redis.Required {
					log.Fatalf("Redis ping failed and REDIS_REQUIRED is set: %v", pingErr)
				}
				log.Printf("Redis ping failed: %v — single-instance mode (in-memory stores)", pingErr)
				_ = rc.Close()
			} else {
				redisClient = rc
				log.Printf("Redis active: %s", redisURL)
			}
		}
	} else if cfg.Redis.Required {
		log.Fatalf("REDIS_URL not set but REDIS_REQUIRED is set: multi-instance deploy requires Redis")
	} else {
		log.Println("REDIS_URL not set; single-instance mode (in-memory stores, polling-only workers, no cross-node WS fan-out)")
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

	// Phase 2 — AgentService (created early: needed by AuthService and ProjectService).
	rolePromptRepo := repository.NewAgentRolePromptRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	agentSvcV2 := service.NewAgentService(
		agentRepo,
		repository.NewAgentSecretRepository(db),
		encryptor,
		txManager,
	)
	agentSvcV2.WithRolePromptRepo(rolePromptRepo).WithApiKeyRepo(apiKeyRepo)

	// Logger с redact-обёрткой — создаём рано, чтобы использовать и в secret-сервисах, и в v2-компонентах.
	v2Logger := slog.New(logging.NewHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// Phase 5 — project/user secrets + MCP registry.
	secretSvc := service.NewSecretService(encryptor)
	projectSecretRepo := repository.NewProjectSecretRepository(db)
	userSecretRepo := repository.NewUserSecretRepository(db)
	projectSecretSvc := service.NewProjectSecretService(projectSecretRepo, secretSvc, v2Logger)
	userSecretSvc := service.NewUserSecretService(userSecretRepo, secretSvc, v2Logger)
	mcpRegistryRepo := repository.NewMCPServerRegistryRepository(db)
	mcpRegistrySvc := service.NewMCPServerRegistryService(mcpRegistryRepo)

	// Загрузка промптов из файлов
	log.Println("Loading prompts from backend/prompts...")
	promptsLoader := promptsloader.New(promptRepo)
	if err := promptsLoader.LoadFromDir(context.Background(), "prompts"); err != nil {
		// Не падаем, если не смогли загрузить промпты (например, нет папки), но логируем ошибку
		log.Printf("Failed to load prompts: %v", err)
	} else {
		log.Println("Prompts loaded successfully")
	}

	// Загрузка workflows и расписаний (агенты создаются через БД, YAML-конфиги удалены — Phase 3)
	log.Println("Loading workflows...")
	wfLoader := workflowloader.New(workflowRepo)
	if err := wfLoader.LoadWorkflows(context.Background(), "workflows"); err != nil {
		log.Printf("Failed to load workflows: %v", err)
	}
	if err := wfLoader.LoadSchedules(context.Background(), "schedules"); err != nil {
		// Не падаем, если нет папки или ошибка, просто логируем
		log.Printf("Failed to load schedules: %v", err)
	}

	// Phase 1 §1.4 — seed дефолтных промптов для ролей агентов.
	// ON CONFLICT DO NOTHING: уважаем правки админа.
	if err := seed.SeedRolePrompts(context.Background(), db, slog.Default()); err != nil {
		log.Printf("Failed to seed role prompts: %v", err)
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
	authService := service.NewAuthServiceWithAgents(userRepo, refreshTokenRepo, jwtManager, eventBus, agentSvcV2, txManager)
	apiKeyService := service.NewApiKeyService(apiKeyRepo, userRepo)
	promptService := service.NewPromptService(promptRepo)
	gitFactory := gitprovider.NewFactory()

	// --- Indexer (Sprint 9) ---
	var weaviateClient *vectordb.Client
	if cfg.Weaviate.Host != "" {
		var err error
		weaviateClient, err = vectordb.NewClient(&vectordb.Config{
			Host:   cfg.Weaviate.Host,
			Scheme: cfg.Weaviate.Scheme,
		})
		if err != nil {
			slog.Error("Failed to create Weaviate client", slog.Any("error", err))
		} else {
			// Проверяем доступность Weaviate при старте
			if err := weaviateClient.HealthCheck(rootCtx); err != nil {
				slog.Warn("Weaviate health check failed, vector features may be unavailable", slog.Any("error", err))
			} else {
				slog.Info("Successfully connected to Weaviate", slog.String("host", cfg.Weaviate.Host))
			}
		}
	} else {
		slog.Info("Weaviate host is not configured, vector features are disabled")
	}

	syncRepo := repository.NewSyncStateRepository(db)
	vectorRepo := repository.NewVectorRepository(weaviateClient)
	codeIndexer, _ := indexer.NewCodeIndexer(syncRepo, vectorRepo, nil, 4, slog.Default())

	// Конструируем git_integration_credentials репозиторий заранее: ProjectService
	// использует его для fallback на OAuth-токен при создании проекта без явного
	// git_credential_id.
	gitIntegrationRepo := repository.NewGitIntegrationCredentialRepository(db)
	projectService := service.WithAgentService(service.NewProjectService(
		projectRepo,
		teamRepo,
		gitCredRepo,
		gitIntegrationRepo,
		txManager,
		gitFactory,
		encryptor,
		eventBus,
		codeIndexer,
		cfg.Git.ImportDir,
	), agentSvcV2)
	toolDefinitionService := service.NewToolDefinitionService(toolDefRepo)
	teamService := service.WithTransactionManager(service.WithAgentServiceForTeam(service.NewTeamService(teamRepo, toolDefRepo), agentSvcV2), txManager)
	taskIndexer := indexer.NewTaskIndexer(taskRepo, taskMsgRepo, vectorRepo, slog.Default())
	taskService := service.NewTaskService(taskRepo, taskMsgRepo, projectService, teamService, txManager, eventBus, taskIndexer, slog.Default())

	// --- IndexerService координатор (Sprint 9.5) ---
	conversationRepo := repository.NewConversationRepository(db)
	conversationMsgRepo := repository.NewConversationMessageRepository(db)
	conversationIndexer, err := indexer.NewConversationIndexer(conversationRepo, conversationMsgRepo, vectorRepo, eventBus, slog.Default())
	if err != nil {
		log.Fatalf("failed to construct conversation indexer: %v", err)
	}

	var vectorDeleter service.VectorDeleter = service.NoopVectorDeleter{}
	if weaviateClient != nil {
		vectorDeleter = weaviateClient
	}

	// Локер переиндексации: Redis (distributed) при multi-instance, иначе in-memory.
	// Без shared-лока две реплики могли бы переиндексировать один проект параллельно
	// (двойная запись/конфликты в Weaviate).
	var indexerLocker service.Locker = service.NewInMemoryLocker()
	if redisClient != nil {
		indexerLocker = service.NewRedisLocker(redisClient)
	}
	indexerService := service.NewIndexerService(
		slog.Default(),
		vectorDeleter,
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

	// Первичная синхронизация моделей — задача-синглтон (только лидер): N реплик не должны
	// одновременно тянуть и апсертить каталог моделей при старте.
	leaderElector.OnLeader("model-sync", func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		log.Println("Starting initial OpenRouter model sync...")
		if err := modelCatalogService.SyncOpenRouterModels(ctx); err != nil {
			log.Printf("Initial model sync failed: %v", err)
		}
	})

	// LLM
	llmFactory := factory.New()
	llmService := service.NewLLMService(llmFactory, cfg.LLM, llmRepo, llmModelRepo)

	// Workflow Engine
	workflowEngine := service.NewWorkflowEngine(workflowRepo, agentRepo, llmService)

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
		"devteam/sandbox-antigravity:local",
	},
		sandbox.WithLogPublisher(sandboxLogAdapter),
		sandbox.WithResourceLimitPolicy(sandbox.ResourceLimitPolicy{
			MemoryFloorBytes: cfg.Sandbox.MemoryFloorBytes,
			MemoryCeilBytes:  cfg.Sandbox.MemoryCeilBytes,
		}),
	)
	// Sprint 16: per-backend образа. claude-code/aider/custom уходят в default
	// (sandbox-claude), hermes — в свой образ. Если в будущем у aider будет
	// отдельный образ — добавляем в map здесь же без правок executor'а.
	sandboxAgentExecutor := agent.NewSandboxAgentExecutor(
		sandboxRunner,
		"devteam/sandbox-claude:local",
		map[string]string{
			string(models.CodeBackendHermes):      "devteam/sandbox-hermes:local",
			string(models.CodeBackendAntigravity): "devteam/sandbox-antigravity:local",
		},
	)

	// Sprint 17 / Orchestration v2 — legacy PipelineEngine удалён.
	// Новый Orchestrator (orchestrator_v2.go) подключается ниже.

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
	// Device-code store: Redis (shared) при multi-instance, иначе in-memory по умолчанию.
	// Без shared-стора device-flow init и polling /callback на разных репликах не сойдутся.
	if redisClient != nil {
		claudeCodeAuthSvc = service.WithClaudeCodeDeviceStore(claudeCodeAuthSvc, service.NewRedisDeviceCodeStore(redisClient))
	}
	// UI Refactoring §4a.4 — публикация IntegrationConnectionChanged для realtime-обновления
	// экрана LLM Integrations (без поллинга). HubBridge маршрутизирует событие в Hub.SendToUser.
	claudeCodeAuthSvc = service.WithClaudeCodeEventBus(claudeCodeAuthSvc, eventBus)

	antigravitySubRepo := repository.NewAntigravitySubscriptionRepository(db)
	antigravityOAuthProvider := service.NewAntigravityOAuthProvider(service.AntigravityOAuthConfig{
		ClientID:      cfg.AntigravityOAuth.ClientID,
		ClientSecret:  cfg.AntigravityOAuth.ClientSecret,
		DeviceCodeURL: cfg.AntigravityOAuth.DeviceCodeURL,
		TokenURL:      cfg.AntigravityOAuth.TokenURL,
		RevokeURL:     cfg.AntigravityOAuth.RevokeURL,
		Scopes:        cfg.AntigravityOAuth.Scopes,
	})
	antigravityAuthSvc := service.NewAntigravityAuthService(antigravitySubRepo, encryptor, antigravityOAuthProvider)
	if redisClient != nil {
		antigravityAuthSvc = service.WithAntigravityDeviceStore(antigravityAuthSvc, service.NewRedisDeviceCodeStore(redisClient))
	}
	antigravityAuthSvc = service.WithAntigravityEventBus(antigravityAuthSvc, eventBus)

	// User-per-credential service (нужен резолверу аутентификации sandbox).
	llmCredRepo := repository.NewUserLlmCredentialRepository(db)
	llmCredSvc := service.NewUserLlmCredentialService(llmCredRepo, txManager, encryptor, slog.Default())

	llmHandler := handler.NewLLMHandler(llmService, llmCredSvc, claudeCodeAuthSvc, antigravityAuthSvc, cfg)

	// Phase 4 §4.3 — wire llmCredRepo for provider validation.
	agentSvcV2.WithLlmCredRepo(llmCredRepo)

	// Sprint 15.18 — динамический резолвер аутентификации sandbox (OAuth subscription / per-user creds / api key).
	sandboxAuthResolver := service.NewSandboxAuthEnvResolver(
		claudeCodeAuthSvc,
		antigravityAuthSvc,
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

	// V2 репозитории.
	taskEventRepoV2 := repository.NewTaskEventRepository(db)
	artifactRepoV2 := repository.NewArtifactRepository(db)
	routerDecisionRepoV2 := repository.NewRouterDecisionRepository(db)
	worktreeRepoV2 := repository.NewWorktreeRepository(db)

	// Sprint 15.M7 — функциональная опция вместо type-assertion-шима WithSandboxAuthResolver.
	orchestratorContextBuilder := service.NewContextBuilderFull(
		encryptor, pipelinePromptComposer, sandboxSecrets, taskMsgRepo,
		service.WithSandboxAuthResolverOption(sandboxAuthResolver),
		// Sprint 16.C — без этой опции AgentSettingsBundle никогда не доезжает
		// до sandbox-runner'а: hermes-config/skills/permission-mode становятся мёртвым кодом.
		service.WithAgentSettingsServiceOption(agentSettingsSvc),
		service.WithArtifactRepositoryOption(artifactRepoV2),
		service.WithGitIntegrationRepositoryOption(gitIntegrationRepo),
	)

	llmProviderRepo := repository.NewLLMProviderRepository(db)
	llmProviderSvc := service.NewLLMProviderService(llmProviderRepo, encryptor)
	llmProviderHandler := handler.NewLLMProviderHandler(llmProviderSvc)

	taskControlBus := service.NewUserTaskControlBus()
	_ = orchestratorContextBuilder // переиспользуется sandbox-резолвером через WithContextBuilderOption (если потребуется); основной путь — через sandboxAgentExecutor
	_ = codeIndexer                // legacy hooks — остаётся для backward-compat handlers
	_ = sandboxRunner              // используется через sandboxAgentExecutor
	// PR-publisher: открывает PR при завершении задачи и служит ground-truth-гейтом для done
	// (см. Orchestrator.SetPullRequestPublisher ниже). Без него done ставился по самоотчётам агентов.
	// gitIntegrationRepo → fallback на OAuth-токен владельца проекта, если project-level
	// git_credential не привязан (тот же путь, что у индексатора/sandbox).
	prPublisher := service.NewGitPRPublisher(gitFactory, encryptor, gitIntegrationRepo, slog.Default())

	// ─────────────────────────────────────────────────────────────────────────
	// Sprint 17 / Orchestration v2 — Stage 5g wiring.
	// ─────────────────────────────────────────────────────────────────────────

	// Redis-notifier для low-latency wakeup воркеров (polling ~500ms vs ~10ms). Клиент уже
	// построен выше; при nil воркеры работают в polling-only режиме.
	var v2Notifier *service.RedisNotifier
	if redisClient != nil {
		v2Notifier = service.NewRedisNotifier(redisClient)
	}

	// Кросс-нодовый WebSocket fan-out. WS-соединения держатся в памяти конкретного
	// инстанса, поэтому при N репликах за балансировщиком событие, опубликованное на
	// одной реплике, без моста не дойдёт до клиентов, подключённых к другой. ClusterBridge
	// ретранслирует project/user-scoped сообщения через Redis Pub/Sub. Без Redis —
	// одноинстансный режим (Hub.cluster == nil, поведение прежнее).
	if redisClient != nil {
		wsCluster := ws.NewClusterBridge(redisClient, hub, instanceID, slog.Default())
		hub.SetCluster(wsCluster)
		wg.Add(1)
		go func() {
			defer wg.Done()
			wsCluster.Run(rootCtx)
		}()
		log.Println("WS cluster bridge active: cross-instance WebSocket fan-out enabled")
	}

	// WorktreeManager — опциональный. Требует WORKTREES_ROOT и REPO_ROOT env.
	// Без них sandbox-агенты работают по старому пути (clone в контейнере).
	var v2WorktreeMgr *service.WorktreeManager
	if wtRoot, repoRoot := os.Getenv("WORKTREES_ROOT"), os.Getenv("REPO_ROOT"); wtRoot != "" && repoRoot != "" {
		mgr, err := service.NewWorktreeManager(
			service.WorktreeManagerConfig{RepoRoot: repoRoot, WorktreesRoot: wtRoot},
			worktreeRepoV2, v2Logger,
		)
		if err != nil {
			log.Printf("WorktreeManager init failed: %v — sandbox isolation via worktree disabled", err)
		} else {
			v2WorktreeMgr = mgr
			log.Printf("WorktreeManager active: repo=%s worktrees=%s", repoRoot, wtRoot)
		}
	} else {
		log.Println("WORKTREES_ROOT/REPO_ROOT not set; sandbox worktree-isolation disabled (legacy clone path)")
	}

	// AgentDispatcher — резолвит executor для агента по execution_kind.
	v2LLMResolver := service.NewSingletonLLMProviderResolver(llmService)
	v2SandboxFactory := service.NewSingletonSandboxExecutorFactory(sandboxAgentExecutor)
	v2Dispatcher := service.NewAgentDispatcher(v2LLMResolver, v2SandboxFactory)

	// RouterService — LLM-диспатчер с retry-pipeline на галлюцинации.
	v2AgentLoader := service.NewDBAgentLoader(db)
	v2RouterSvc := service.NewRouterService(v2AgentLoader, v2Dispatcher, v2Logger, service.DefaultRouterConfig())

	// Orchestrator (v2) — ядро. Реализует service.TaskOrchestrator интерфейс через
	// EnqueueInitialStep, плюс полноценный Step() для StepWorker'ов.
	orchestratorService := service.NewOrchestrator(
		db,
		artifactRepoV2,
		taskEventRepoV2,
		routerDecisionRepoV2,
		v2WorktreeMgr,
		v2RouterSvc,
		v2Notifier,
		eventBus,
		v2Logger,
		service.DefaultOrchestratorConfig(),
	)
	// Ground-truth-гейт завершения: done подтверждается открытым PR, иначе → needs_human.
	orchestratorService.SetPullRequestPublisher(prPublisher)

	// TaskLifecycleService — POST /tasks/:id/cancel handler использует.
	v2TaskLifecycle := service.NewTaskLifecycleService(db, v2Notifier, v2Logger)
	_ = v2TaskLifecycle  // подключается в task_handler через wiring ниже (Stage 5g.6)
	_ = llmAgentExecutor // ссылка остаётся для обратной совместимости conversation/handler инициализации

	conversationService := service.NewConversationService(
		conversationRepo,
		conversationMsgRepo,
		projectService,
		taskService,
		orchestratorService,
		conversationIndexer,
		txManager,
		eventBus,
	)
	conversationHandler := handler.NewConversationHandler(conversationService)

	ctxWorker, cancelWorker := context.WithCancel(context.Background())

	// Workflow Worker — задача-синглтон (только лидер): processNextTask забирает execution
	// обычным SELECT без SKIP LOCKED, поэтому на N репликах один execution выполнился бы
	// несколько раз. Отключается через WORKFLOW_WORKER_ENABLED=false.
	if cfg.WorkflowWorkerEnabled {
		leaderElector.OnLeader("workflow-worker", func(ctx context.Context) {
			workflowEngine.RunWorker(ctx)
		})
	} else {
		log.Println("Workflow worker is disabled (WORKFLOW_WORKER_ENABLED=false); pending executions will not run until re-enabled")
	}

	// Scheduler (cron) — задача-синглтон (только лидер): иначе cron-джобы (model sync,
	// project reindex, scheduled workflows) сработают на каждой реплике. Создаём свежий
	// планировщик на каждое получение лидерства — Start() регистрирует джобы, повторный
	// вызов на том же cron дал бы дубли.
	leaderElector.OnLeader("scheduler", func(ctx context.Context) {
		sched := service.NewScheduler(workflowRepo, workflowEngine, modelCatalogService, projectService, cfg.Git.ProjectSyncCron)
		if err := sched.Start(ctx); err != nil {
			log.Printf("Failed to start scheduler: %v", err)
			return
		}
		<-ctx.Done()
		sched.Stop()
	})

	// Handlers
	authHandler := handler.NewAuthHandler(authService, jwtManager)
	apiKeyHandler := handler.NewApiKeyHandler(apiKeyService, &cfg.MCP)
	promptHandler := handler.NewPromptHandler(promptService)
	projectHandler := handler.NewProjectHandler(projectService)
	teamHandler := handler.NewTeamHandler(teamService, projectService)
	toolDefinitionHandler := handler.NewToolDefinitionHandler(toolDefinitionService)
	taskHandler := handler.NewTaskHandler(taskService, orchestratorService, taskControlBus, v2TaskLifecycle)
	webhookPublicBase := fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	webhookHandler := handler.NewWebhookHandler(webhookRepo, workflowRepo, workflowEngine, webhookPublicBase)
	workflowHandler := handler.NewWorkflowHandler(workflowEngine)

	// PATCH /me/llm-credentials rate limit: shared через Redis при multi-instance (иначе
	// лимит 30/мин считался бы per-instance → обход 30×N), иначе in-memory.
	var llmCredRLOpts []middleware.RateLimiterOption
	if redisClient != nil {
		llmCredRLOpts = append(llmCredRLOpts, middleware.WithPatchRateLimitRedis(redisClient))
	}
	llmCredRL := middleware.NewLlmCredentialsPatchRateLimiter(30, time.Minute, llmCredRLOpts...)
	llmCredHandler := handler.NewUserLlmCredentialHandler(llmCredSvc)

	// UI Refactoring §4a.1 — callback-handler логирует через redact-обёрнутый logger,
	// чтобы случайные access_token / client_secret в сообщениях провайдера не утекали в stdout.
	claudeCodeAuthHandler := handler.WithClaudeCodeAuthLogger(
		handler.NewClaudeCodeAuthHandler(claudeCodeAuthSvc),
		v2Logger,
	)

	antigravityAuthHandler := handler.WithAntigravityAuthLogger(
		handler.NewAntigravityAuthHandler(antigravityAuthSvc),
		v2Logger,
	)

	// UI Refactoring Stage 3a — git-интеграции (GitHub / GitLab.com / BYO GitLab).
	// (gitIntegrationRepo инициализирован выше для ProjectService fallback.)
	githubOAuthClient := service.NewGitHubOAuthClient(service.GitHubOAuthConfig{
		ClientID:     cfg.GitHubOAuth.ClientID,
		ClientSecret: cfg.GitHubOAuth.ClientSecret,
		Scopes:       cfg.GitHubOAuth.Scopes,
	})
	gitlabOAuthClient := service.NewGitLabOAuthClient(service.GitLabOAuthConfig{
		ClientID:     cfg.GitLabOAuth.ClientID,
		ClientSecret: cfg.GitLabOAuth.ClientSecret,
		Scopes:       cfg.GitLabOAuth.Scopes,
	})
	// OAuth state store: Redis (shared) при multi-instance, иначе in-memory. Без shared-стора
	// OAuth-init и callback на разных репликах не сойдутся (ErrGitOAuthStateNotFound).
	var gitOAuthStateStore service.GitOAuthStateStore = service.NewInMemoryGitOAuthStateStore()
	if redisClient != nil {
		gitOAuthStateStore = service.NewRedisGitOAuthStateStore(redisClient)
	}
	gitIntegrationSvc := service.NewGitIntegrationService(service.GitIntegrationServiceDeps{
		Repo:       gitIntegrationRepo,
		Encryptor:  encryptor,
		GitHub:     githubOAuthClient,
		GitLab:     gitlabOAuthClient,
		Validator:  service.NewGitProviderHostValidator(service.DefaultHostResolver(), cfg.IsProd()),
		StateStore: gitOAuthStateStore,
		Bus:        eventBus,
		Logger:     v2Logger,
	})
	gitIntegrationHandler := handler.WithGitIntegrationLogger(
		handler.NewGitIntegrationHandler(gitIntegrationSvc),
		v2Logger,
	)
	if cfg.GitHubOAuth.ClientID == "" {
		log.Println("GitHub OAuth: disabled (set GITHUB_OAUTH_CLIENT_ID to enable)")
	}
	if cfg.GitLabOAuth.ClientID == "" {
		log.Println("GitLab.com OAuth: disabled (set GITLAB_OAUTH_CLIENT_ID to enable; self-hosted BYO остаётся доступен)")
	}
	if cfg.ClaudeCodeOAuth.ClientID != "" {
		// Токен-рефрешер — задача-синглтон (только лидер): два инстанса устроили бы гонку
		// на обновлении одного OAuth-токена.
		refresher := service.NewClaudeCodeTokenRefresher(claudeCodeSubRepo, claudeCodeAuthSvc, slog.Default())
		leaderElector.OnLeader("claude-code-token-refresher", refresher.Run)
		log.Println("Claude Code token refresher: registered (leader-only)")
	} else {
		log.Println("Claude Code OAuth: disabled (set CLAUDE_CODE_OAUTH_CLIENT_ID to enable)")
	}
	if cfg.AntigravityOAuth.ClientID != "" {
		refresher := service.NewAntigravityTokenRefresher(antigravitySubRepo, antigravityAuthSvc, slog.Default())
		leaderElector.OnLeader("antigravity-token-refresher", refresher.Run)
		log.Println("Antigravity token refresher: registered (leader-only)")
	} else {
		log.Println("Antigravity OAuth: disabled (set ANTIGRAVITY_OAUTH_CLIENT_ID to enable)")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Sprint 17 / Orchestration v2 — запуск пулов воркеров и retention.
	// Пулы: step=5, agent_llm=20, agent_sandbox=2 (план §2.7, дефолт для VPS 8-16GB).
	// Если в config будут per-env overrides — подключить через cfg.Orchestrator.{...}.
	//
	// ORCHESTRATOR_V2_WORKERS_ENABLED=false — тест-конфигурация для PR-gate
	// smoke-сьюита (см. docs/integration-tests-plan.md, backend/test/featuresmoke).
	// Это НЕ хак на SQLSTATE 40001 — для него есть retry в repository.TransactionManager.
	// Это изоляция: PR-gate смоук проверяет CRUD/API-контракт, а не реальный pipeline;
	// крутить воркеров впустую (они валятся на ненастроенный LLM) — лишний шум в логах
	// и постоянная гонка с пользовательскими операциями pause/cancel. Nightly
	// real-режим (feature-e2e-real.yml) флаг не выставляет — там воркеры нужны для
	// прогона полного pipeline.
	// ─────────────────────────────────────────────────────────────────────────
	v2WorkersEnabled := !strings.EqualFold(strings.TrimSpace(os.Getenv("ORCHESTRATOR_V2_WORKERS_ENABLED")), "false") &&
		!strings.EqualFold(strings.TrimSpace(os.Getenv("ORCHESTRATOR_V2_WORKERS_ENABLED")), "0")
	if !v2WorkersEnabled {
		log.Println("Orchestrator v2 workers DISABLED via ORCHESTRATOR_V2_WORKERS_ENABLED=false (smoke-test isolation)")
	} else {
		// Размер пулов конфигурируется через env — для дев-окружения разумно
		// держать их маленькими (каждый воркер опрашивает Yugabyte распределённым
		// SELECT ... FOR UPDATE SKIP LOCKED, и пул в полку — главный idle-CPU).
		// Дефолты ориентированы на прод (5 step + 22 agent = 20 llm + 2 sandbox;
		// пул общий, ClaimNext sequencing уже разводит).
		// Пулы воркеров — только на ролях, обслуживающих очередь (worker/all). Они claim-safe
		// (task_events + SKIP LOCKED), поэтому работают на всех worker-репликах параллельно.
		if role.RunsWorkers() {
			stepWorkersCount := orchestratorWorkerCount("ORCHESTRATOR_STEP_WORKERS", 5)
			agentWorkersCount := orchestratorWorkerCount("ORCHESTRATOR_AGENT_WORKERS", 22)
			for i := 0; i < stepWorkersCount; i++ {
				w := service.NewStepWorker(
					taskEventRepoV2,
					orchestratorService,
					v2Notifier,
					v2Logger,
					service.StepWorkerConfig{WorkerID: fmt.Sprintf("step-worker-%d", i), PollInterval: 500 * time.Millisecond},
				)
				go func() {
					if err := w.Run(ctxWorker); err != nil {
						log.Printf("step worker exited with error: %v", err)
					}
				}()
			}
			for i := 0; i < agentWorkersCount; i++ {
				w := service.NewAgentWorker(
					db,
					taskEventRepoV2,
					artifactRepoV2,
					v2Dispatcher,
					v2WorktreeMgr,
					v2Notifier,
					eventBus,
					v2Logger,
					service.AgentWorkerConfig{
						WorkerID:        fmt.Sprintf("agent-worker-%d", i),
						PollInterval:    500 * time.Millisecond,
						AgentJobTimeout: time.Hour,
					},
					orchestratorContextBuilder,
				)
				go func() {
					if err := w.Run(ctxWorker); err != nil {
						log.Printf("agent worker exited with error: %v", err)
					}
				}()
			}
			log.Printf("Orchestrator v2 workers started: %d step + %d agent", stepWorkersCount, agentWorkersCount)
		} else {
			log.Printf("role=%s: orchestrator worker pools not started (only worker/all run them)", role)
		}

		// Retention: 30 дней router_decisions + 1 сутки released worktrees. Раз в час.
		// Задача-синглтон (только лидер): глобальная чистка, одного исполнителя достаточно.
		v2Retention := service.NewRetentionService(routerDecisionRepoV2, taskEventRepoV2, v2WorktreeMgr, v2Logger, service.DefaultRetentionConfig())
		leaderElector.OnLeader("retention", func(ctx context.Context) {
			if err := v2Retention.Run(ctx); err != nil {
				log.Printf("retention service exited with error: %v", err)
			}
		})
		log.Println("Orchestrator v2 retention service registered (leader-only)")
	} // конец if v2WorkersEnabled

	// Sprint 21 — Global Assistant (правая боковая панель).
	// Собираем agentloop.Executor с публичными константами AssistantMax* (см. assistant_service.go),
	// AuthorizedExecutor (фиксированный каталог tools), AssistantService и AssistantHandler.
	// ConversationService не подключён к процессу — соответствующая группа conversation_*
	// tools просто выпадает из каталога (поведение AuthorizedExecutor задокументировано).
	assistantExecutor := agentloop.NewExecutor(agentloop.Config{
		MaxIterations:      service.AssistantMaxIterations,
		MaxToolResultBytes: service.AssistantMaxToolResultBytes,
		MaxHistoryBytes:    service.AssistantMaxHistoryBytes,
		HistoryTailKeep:    service.AssistantHistoryTailKeep,
		PerLLMCallTimeout:  60 * time.Second,
	}, v2Logger) // v2Logger обёрнут logging.NewHandler — маскирует секреты в промптах/raw_response/tool args (docs/rules/backend.mdc §2.3, review.md §1).
	assistantToolCatalog := mcpserver.NewAuthorizedExecutor(mcpserver.AuthorizedExecutorDeps{
		ProjectService:        projectService,
		TaskService:           taskService,
		ConversationService:   conversationService,
		TeamService:           teamService,
		AgentService:          agentSvcV2,
		GitIntegrationService: gitIntegrationSvc,
		QueryService: service.NewOrchestrationQueryService(
			artifactRepoV2,
			routerDecisionRepoV2,
			worktreeRepoV2,
		),
		OrchestratorService: orchestratorService,
	})
	assistantSessionRepo := repository.NewAssistantSessionRepository(db)
	assistantSvc, err := service.NewAssistantService(service.AssistantServiceDeps{
		Repo:         assistantSessionRepo,
		TaskRepo:     taskRepo,
		ProjectRepo:  projectRepo,
		TeamRepo:     teamRepo,
		AgentLoader:  service.NewDBAgentLoader(db),
		AgentCreator: agentSvcV2,
		LLMResolver:  service.NewAssistantLLMClientAdapter(llmCredSvc, llmFactory, cfg.LLM, llmRepo, llmModelRepo),
		UserCreds:    llmCredSvc,
		ToolCatalog:  assistantToolCatalog,
		Hub:          hub,
		Executor:     assistantExecutor,
		Logger:       v2Logger, // redact-обёрнутый; не пускает токены/ключи/пароли в stdout (см. comment выше).
	})
	if err != nil {
		log.Fatalf("Failed to construct AssistantService: %v", err)
	}
	// Stale-recovery ассистент-сессий — задача-синглтон (только лидер): чистка, одного хватает.
	leaderElector.OnLeader("assistant-stale-recovery", assistantSvc.StartStaleRecovery)

	// Все задачи-синглтоны зарегистрированы. Выбор лидера запускаем только на ролях
	// scheduler/all — иначе синглтоны (cron/refreshers/retention/workflow/model-sync) не
	// исполняются на этой ноде (api/worker лишь регистрируют их, но не выбирают лидера).
	// Привязка к ctxWorker: на shutdown cancelWorker() отменяет лиз и все leader-tasks.
	if role.RunsLeaderTasks() {
		go leaderElector.Run(ctxWorker)
	} else {
		log.Printf("role=%s: leader election not started (only scheduler/all run singletons)", role)
	}

	assistantHandler := handler.NewAssistantHandler(assistantSvc)

	// WebSocket Handler. Адаптер транслирует доменные ошибки ProjectService
	// (ErrProjectNotFound/Forbidden) в булевый контракт ws.ProjectAccessor —
	// это break import cycle (см. ws/handler.go), чтобы `service` мог
	// импортировать `ws` для assistant-marshalers (Sprint 21 §7).
	wsProjectAccess := wsProjectAccessAdapter{svc: projectService}
	wsHandler := ws.NewWebSocketHandler(hub, wsProjectAccess, ws.HandlerConfig{
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
		ConversationHandler:   conversationHandler,
		JWTManager:            jwtManager,
		ApiKeyService:         apiKeyService,
		WebSocketHandler:      wsHandler,

		UserLlmCredentialHandler: llmCredHandler,
		LlmCredentialsPatchRL:    llmCredRL,

		ClaudeCodeAuthHandler:  claudeCodeAuthHandler,
		AntigravityAuthHandler: antigravityAuthHandler,
		GitIntegrationHandler:  gitIntegrationHandler,
		AssistantHandler:       assistantHandler,
		AgentSettingsHandler:   handler.NewAgentSettingsHandler(teamService),
		LLMProviderHandler:     llmProviderHandler,
		HermesHandler:          handler.NewHermesHandler(),

		// Sprint 17 / Sprint 5F.3 — HTTP API для v2 admin (Frontend Agents Management).
		AgentV2Handler: handler.NewAgentV2Handler(agentSvcV2),

		// Phase 4 §4.2 — /me/agents — user-level агенты.
		AgentMyHandler: handler.NewAgentMyHandler(agentSvcV2),

		// Phase 1 §1.4 — admin API для дефолтных промптов ролей агентов.
		AgentRolePromptHandler: handler.NewAgentRolePromptHandler(rolePromptRepo),

		// Phase 5 — project/user secrets + MCP registry admin CRUD.
		ProjectSecretHandler:     handler.NewProjectSecretHandler(projectSecretSvc),
		UserSecretHandler:        handler.NewUserSecretHandler(userSecretSvc),
		MCPServerRegistryHandler: handler.NewMCPServerRegistryHandler(mcpRegistrySvc),

		// Sprint 17 / Orchestration v2 — read-only API + manual unstick (POST /worktrees/:id/release).
		// taskService нужен ListWorktrees'у для task-ownership check'а (см. Sprint 17 / 6.2).
		// v2WorktreeMgr опционален — без него ReleaseWorktree отвечает 503 (см. 6.3).
		OrchestrationV2Handler: handler.NewOrchestrationV2Handler(
			artifactRepoV2,
			routerDecisionRepoV2,
			worktreeRepoV2,
			taskService,
			v2WorktreeMgr,
		),
	})

	// HTTP API/WS — только на ролях api/all. worker/scheduler не принимают клиентский трафик
	// (LB маршрутизирует внешние запросы только на api-инстансы) и остаются живы на <-quit.
	if role.RunsHTTP() {
		go func() {
			if err := srv.Start(); err != nil {
				log.Fatalf("Failed to run server: %v", err)
			}
		}()
	} else {
		log.Printf("role=%s: HTTP server not started (only api/all serve API/WS)", role)
	}

	// --- MCP-сервер (условный запуск; только на ролях с HTTP) ---
	var mcpHTTPServer *http.Server

	if role.RunsHTTP() && cfg.MCP.Enabled {
		mcpSrv := mcpserver.NewMCPServer(mcpserver.Dependencies{
			Config:                 cfg.MCP,
			LLMService:             llmService,
			WorkflowEngine:         workflowEngine,
			PromptService:          promptService,
			ProjectService:         projectService,
			TeamService:            teamService,
			TaskService:            taskService,
			ToolDefinitionService:  toolDefinitionService,
			OrchestratorSvc:        orchestratorService,
			ApiKeyService:          apiKeyService,
			ClaudeCodeAuthService:  claudeCodeAuthSvc,
			AntigravityAuthService: antigravityAuthSvc,
			GitIntegrationService:  gitIntegrationSvc,
			MCPServerRegistryRepo:  repository.NewMCPServerRegistryRepository(db),
			AgentSkillRepo:         repository.NewAgentSkillRepository(db),

			// Sprint 17 / Sprint 5 — v2 orchestration MCP tools через SERVICE-слой.
			AgentSvcV2: agentSvcV2,
			OrchestrationQuerySvcV2: service.NewOrchestrationQueryService(
				artifactRepoV2,
				routerDecisionRepoV2,
				worktreeRepoV2,
			),
			TaskLifecycleV2: v2TaskLifecycle,

			// Sprint 17 / 6.3 — destructive worktree_release MCP tool. nil → tool не регистрируется
			// (legacy clone-path: WORKTREES_ROOT не задан).
			WorktreeMgrV2: v2WorktreeMgr,

			// Sprint 21 §5 — assistant-специфичные MCP tools.
			Hub:      hub,
			UserRepo: userRepo,

			// Phase 5 — project/user secret MCP tools.
			ProjectSecretSvc: projectSecretSvc,
			UserSecretSvc:    userSecretSvc,
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

	// Останавливаем фоновые воркеры и leader-tasks. cancelWorker() отменяет ctxWorker:
	// LeaderElector освобождает лиз и гасит cron/refreshers/retention/stale-recovery/
	// workflow-worker (все привязаны к этому контексту).
	cancelWorker()

	// Graceful shutdown MCP-сервера
	if mcpHTTPServer != nil {
		log.Println("Shutting down MCP server...")
		if err := mcpHTTPServer.Shutdown(ctx); err != nil {
			log.Printf("MCP server forced to shutdown: %v", err)
		}
	}

	// Graceful shutdown основного сервера (только если он стартовал — api/all).
	if role.RunsHTTP() {
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
	}

	// Останавливаем Hub и Bridge
	rootCancel()
	wg.Wait()

	log.Println("All servers exited")
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
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             gormSlowThresholdFromEnv(),
			LogLevel:                  gormLogLevelFromEnv(),
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormLogger,
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
	sqlDB.SetConnMaxIdleTime(time.Minute)

	// Проверяем подключение
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// gormLogLevelFromEnv читает GORM_LOG_LEVEL (silent|error|warn|info).
// По умолчанию warn — заглушает шумный поллинг очередей (task_events),
// но всё ещё логирует ошибки SQL и медленные запросы (>SlowThreshold).
// Для отладки запроса можно временно поднять до info.
func gormLogLevelFromEnv() logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GORM_LOG_LEVEL"))) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}

// gormSlowThresholdFromEnv читает GORM_SLOW_THRESHOLD (например, "1s", "500ms").
// По умолчанию 2s — на Yugabyte распределённые запросы (FOR UPDATE SKIP LOCKED
// в task_events поллинге) регулярно тратят 200-500ms и засоряют логи.
// orchestratorWorkerCount читает размер пула воркеров из env. Невалидное или
// отрицательное значение → дефолт; 0 допустим (полностью отключает пул данного
// типа — например, нода только под step-роутинг без исполнителей).
func orchestratorWorkerCount(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		log.Printf("%s=%q invalid, falling back to default %d", key, raw, def)
		return def
	}
	return n
}

func gormSlowThresholdFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GORM_SLOW_THRESHOLD"))
	if raw == "" {
		return 2 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 2 * time.Second
	}
	return d
}

// Sprint 17 / Orchestration v2 — stub удалён. Используется реальный
// service.Orchestrator(v2), сконструированный выше.

// wsProjectAccessAdapter мостит service.ProjectService.HasAccess в булевый
// контракт ws.ProjectAccessor (см. ws/handler.go). Без адаптера ws пришлось
// бы импортировать service для проверки sentinel'ов ErrProjectNotFound /
// ErrProjectForbidden — но service сам импортирует ws ради typed-Marshal'еров
// assistant.* событий (Sprint 21 §7).
type wsProjectAccessAdapter struct {
	svc service.ProjectService
}

func (a wsProjectAccessAdapter) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (allowed, denied bool, err error) {
	err = a.svc.HasAccess(ctx, userID, userRole, projectID)
	if err == nil {
		return true, false, nil
	}
	if errors.Is(err, service.ErrProjectNotFound) || errors.Is(err, service.ErrProjectForbidden) {
		return false, true, nil
	}
	return false, false, err
}
