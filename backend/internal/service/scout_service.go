package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// scoutDefaultTimeoutSeconds — дефолтный потолок прогона разведчика.
const scoutDefaultTimeoutSeconds = 600

// scoutDossierBeginMarker / scoutDossierEndMarker — маркеры, между которыми
// разведчик обязан вывести досье. Извлекаем его из stdout агента (agent.log →
// ExecutionResult.Output): отдельного канала для произвольного файла из контейнера
// нет (сбор артефактов берёт фиксированные status.json/full.diff/agent.log).
const (
	scoutDossierBeginMarker = "<<<DOSSIER>>>"
	scoutDossierEndMarker   = "<<<END_DOSSIER>>>"
)

// scoutDefaultSystemPrompt — встроенный промпт разведчика (используется когда
// scout_configs.prompt пуст). Редактируемый промпт ЗАМЕНЯЕТ его целиком, но
// контракт вывода (scoutOutputContract) добавляется всегда — иначе извлечь
// досье невозможно.
const scoutDefaultSystemPrompt = `Ты — агент-разведчик команды разработки. Твоя задача — НЕ писать и НЕ менять код, а собрать контекст по проблеме пользователя и подготовить досье для постановки задачи.

На диске у тебя весь код проекта (см. каталог репозиториев ниже): основной репозиторий в /workspace/repo, соседние — read-only в /workspace/siblings/<slug>. Изучи релевантные места:
- где в коде скорее всего лежит проблема и как сейчас устроено это поведение;
- какие есть ограничения, зависимости и риски;
- 1–3 реалистичных подхода к решению с трейд-оффами;
- что остаётся неясным и требует уточнения у пользователя.

НИЧЕГО не коммить, не меняй файлы, не создавай ветки. Работай только на чтение.`

// scoutOutputContract — обязательный хвост системного промпта: формат досье.
const scoutOutputContract = `

В САМОМ КОНЦЕ работы выведи досье строго между маркерами ` + scoutDossierBeginMarker + ` и ` + scoutDossierEndMarker + ` (markdown), по разделам:
## Проблема
## Релевантные файлы (путь — почему важен)
## Как устроено сейчас
## Подходы (1–3, с трейд-оффами)
## Открытые вопросы
## Предлагаемые критерии приёмки

Ничего не выводи после ` + scoutDossierEndMarker + `.`

var (
	// ErrScoutInvalidBackend — неизвестный code_backend.
	ErrScoutInvalidBackend = errors.New("invalid scout code_backend")
	// ErrScoutInvalidTimeout — timeout_seconds вне диапазона 60..3600.
	ErrScoutInvalidTimeout = errors.New("timeout_seconds must be between 60 and 3600")
	// ErrScoutInvalidSubscriptionID — subscription_id не парсится как UUID.
	ErrScoutInvalidSubscriptionID = errors.New("invalid subscription_id format")
	// ErrScoutSubscriptionNotFound — выбранная подписка не найдена у владельца проекта.
	ErrScoutSubscriptionNotFound = errors.New("selected subscription is not connected for the project owner")
	// ErrScoutInvalidProviderKind — неизвестный provider_kind.
	ErrScoutInvalidProviderKind = errors.New("invalid scout provider_kind")
	// ErrScoutProviderBackendMismatch — provider_kind несовместим с code_backend.
	ErrScoutProviderBackendMismatch = errors.New("provider_kind is not allowed for the selected code_backend")
	// ErrScoutInvalidTemperature — temperature вне диапазона 0..2.
	ErrScoutInvalidTemperature = errors.New("temperature must be between 0 and 2")
	// ErrScoutInvalidSettings — невалидный code_backend_settings/sandbox_permissions.
	ErrScoutInvalidSettings = errors.New("invalid scout settings")

	// ErrScoutEmptyProblem — пустая постановка проблемы.
	ErrScoutEmptyProblem = errors.New("problem statement is required")
	// ErrScoutNoRepositories — у проекта нет репозитория для разведки.
	ErrScoutNoRepositories = errors.New("project has no repository to scout")
	// ErrScoutNoSubscription — у владельца проекта не подключена Claude-подписка.
	ErrScoutNoSubscription = errors.New("claude subscription is not connected for the project owner")
	// ErrScoutBackendUnsupported — фаза 1 диспатчит только claude-code (подписка).
	ErrScoutBackendUnsupported = errors.New("scout dispatch currently supports only the claude-code backend")
	// ErrScoutRunNotFound — прогон разведчика не найден.
	ErrScoutRunNotFound = errors.New("scout run not found")
)

// ScoutResumer — поднимает распарканную сессию ассистента результатом разведки.
// Реализуется assistantService; вызывается ScoutService при завершении (или
// сбое) прогона, привязанного к сессии. Разрывает цикл DI scout↔assistant.
type ScoutResumer interface {
	ResumeFromScout(ctx context.Context, sessionID, userID uuid.UUID, toolCallID, dossier string, runErr error)
}

// ScoutGitTokenResolver — git-креды (GIT_TOKEN/...) для клонирования репозиториев
// проекта в sandbox. Реализуется ContextBuilder — переиспользуем его дешифровку
// кредов и OAuth-рефреш, а не дублируем.
type ScoutGitTokenResolver interface {
	ResolveGitTokenSecrets(ctx context.Context, project *models.Project, targetRepo *models.ProjectRepository) map[string]string
}

// ScoutDispatcher — то, что ассистенту нужно от разведчика: диспатч из
// распарканной сессии и гейтинг tool'а. Реализуется ScoutService.
type ScoutDispatcher interface {
	DispatchForSession(ctx context.Context, userID, projectID, sessionID uuid.UUID, toolCallID, problem string) error
	ScoutEnabled(ctx context.Context, projectID uuid.UUID) bool
}

// ScoutService — агент-разведчик проекта: конфиг (фаза 0), диспатч прогонов
// сбора контекста в sandbox на подписке (фаза 1), wake-up ассистента (фаза 2).
type ScoutService interface {
	GetConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.ScoutConfig, error)
	UpdateConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateScoutConfigRequest) (*models.ScoutConfig, error)
	// Dispatch запускает прогон разведчика по постановке проблемы (асинхронно).
	Dispatch(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, problem string) (*models.ScoutRun, error)
	GetRun(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, runID uuid.UUID) (*models.ScoutRun, error)
	ListRuns(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ScoutRun, error)

	// ScoutDispatcher: диспатч из распарканной сессии ассистента + гейтинг.
	DispatchForSession(ctx context.Context, userID, projectID, sessionID uuid.UUID, toolCallID, problem string) error
	ScoutEnabled(ctx context.Context, projectID uuid.UUID) bool
	// SetResumer — пост-конструкторная привязка ассистента (разрыв цикла DI).
	SetResumer(r ScoutResumer)
}

// ScoutServiceDeps — DI-bag конструктора.
type ScoutServiceDeps struct {
	Repo       repository.ScoutRepository
	ProjectSvc ProjectService
	// SubscriptionRepo — для валидации выбранной подписки (одна на владельца).
	SubscriptionRepo repository.ClaudeCodeSubscriptionRepository
	// RepoRepo — git-репозитории проекта (мульти-репо): primary + siblings.
	RepoRepo repository.ProjectRepoRepository
	// SandboxExecutor — тот же sandbox-исполнитель, что у dev-агентов.
	SandboxExecutor agent.AgentExecutor
	// AuthResolver — резолвер auth-env по (project, agent): тот же, что у dev-агентов
	// (claude oauth/api-key, hermes/antigravity). Разведчик строит временный Agent.
	AuthResolver SandboxAuthEnvResolver
	// AgentSettingsSvc — сборка sandbox-bundle (MCP/скиллы/permissions) по агенту.
	AgentSettingsSvc AgentSettingsService
	// GitTokenResolver — git-креды для клонирования репо проекта в sandbox (как у dev-агентов).
	GitTokenResolver ScoutGitTokenResolver
	Logger           *slog.Logger
}

// NewScoutService создаёт сервис разведчика.
func NewScoutService(deps ScoutServiceDeps) (ScoutService, error) {
	if deps.Repo == nil {
		return nil, errors.New("ScoutService: Repo is required")
	}
	if deps.ProjectSvc == nil {
		return nil, errors.New("ScoutService: ProjectSvc is required")
	}
	if deps.SubscriptionRepo == nil {
		return nil, errors.New("ScoutService: SubscriptionRepo is required")
	}
	if deps.RepoRepo == nil {
		return nil, errors.New("ScoutService: RepoRepo is required")
	}
	if deps.SandboxExecutor == nil {
		return nil, errors.New("ScoutService: SandboxExecutor is required")
	}
	if deps.AuthResolver == nil {
		return nil, errors.New("ScoutService: AuthResolver is required")
	}
	if deps.AgentSettingsSvc == nil {
		return nil, errors.New("ScoutService: AgentSettingsSvc is required")
	}
	if deps.GitTokenResolver == nil {
		return nil, errors.New("ScoutService: GitTokenResolver is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &scoutService{deps: deps}, nil
}

type scoutService struct {
	deps ScoutServiceDeps
	// resumer — привязка ассистента (ставится сеттером на старте, до диспатчей).
	resumer ScoutResumer
}

// SetResumer привязывает ассистента для wake-up (вызывается один раз при wiring).
func (s *scoutService) SetResumer(r ScoutResumer) { s.resumer = r }

// ScoutEnabled — включён ли разведчик у проекта (для гейтинга tool'а ассистента).
func (s *scoutService) ScoutEnabled(ctx context.Context, projectID uuid.UUID) bool {
	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	if err != nil {
		return false
	}
	return cfg.IsEnabled
}

// ─────────────────────────────────────────────────────────────────────────────
// Конфиг.
// ─────────────────────────────────────────────────────────────────────────────

// defaultScoutConfig — синтетический дефолт для проектов без сохранённого конфига
// (GET до первого PUT). Не персистится.
func defaultScoutConfig(projectID uuid.UUID) *models.ScoutConfig {
	return &models.ScoutConfig{
		ProjectID:           projectID,
		IsEnabled:           false,
		Prompt:              "",
		CodeBackend:         models.CodeBackendClaudeCode,
		CodeBackendSettings: datatypes.JSON([]byte("{}")),
		SandboxPermissions:  datatypes.JSON([]byte("{}")),
		TimeoutSeconds:      scoutDefaultTimeoutSeconds,
	}
}

func (s *scoutService) GetConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.ScoutConfig, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrScoutConfigNotFound) {
			return defaultScoutConfig(projectID), nil
		}
		return nil, err
	}
	return cfg, nil
}

func (s *scoutService) UpdateConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateScoutConfigRequest) (*models.ScoutConfig, error) {
	project, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID)
	if err != nil {
		return nil, err
	}

	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	created := false
	if err != nil {
		if !errors.Is(err, repository.ErrScoutConfigNotFound) {
			return nil, err
		}
		cfg = defaultScoutConfig(projectID)
		cfg.ID = uuid.New()
		cfg.CreatedBy = userID
		created = true
	}

	if req.Prompt != nil {
		cfg.Prompt = *req.Prompt
	}
	if req.CodeBackend != nil {
		backend := models.CodeBackend(*req.CodeBackend)
		if !backend.IsValid() {
			return nil, ErrScoutInvalidBackend
		}
		cfg.CodeBackend = backend
	}
	if req.ProviderKind != nil {
		trimmed := strings.TrimSpace(*req.ProviderKind)
		if trimmed == "" {
			cfg.ProviderKind = nil
		} else {
			pk := models.AgentProviderKind(trimmed)
			if !pk.IsValid() {
				return nil, ErrScoutInvalidProviderKind
			}
			// Ограничение backend→провайдер (как agent_provider_rules на фронте).
			if !scoutProviderAllowedForBackend(cfg.CodeBackend, pk) {
				return nil, ErrScoutProviderBackendMismatch
			}
			cfg.ProviderKind = &pk
		}
	}
	if req.Temperature != nil {
		if *req.Temperature < 0 || *req.Temperature > 2 {
			return nil, ErrScoutInvalidTemperature
		}
		t := *req.Temperature
		cfg.Temperature = &t
	}
	if len(req.CodeBackendSettings) > 0 {
		if !isJSONObject(req.CodeBackendSettings) {
			return nil, fmt.Errorf("%w: code_backend_settings must be a JSON object", ErrScoutInvalidSettings)
		}
		// Та же строгая валидация, что у агента (DisallowUnknownFields + regex'ы).
		if err := validateCodeBackendSettingsStrict(req.CodeBackendSettings); err != nil {
			return nil, fmt.Errorf("%w: code_backend_settings: %v", ErrScoutInvalidSettings, err)
		}
		cfg.CodeBackendSettings = datatypes.JSON(append([]byte(nil), req.CodeBackendSettings...))
	}
	if len(req.SandboxPermissions) > 0 {
		var perms SandboxPermissions
		if err := json.Unmarshal(req.SandboxPermissions, &perms); err != nil {
			return nil, fmt.Errorf("%w: sandbox_permissions: invalid JSON", ErrScoutInvalidSettings)
		}
		if err := ValidateSandboxPermissions(perms); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrScoutInvalidSettings, err)
		}
		cfg.SandboxPermissions = datatypes.JSON(append([]byte(nil), req.SandboxPermissions...))
	}
	if req.TimeoutSeconds != nil {
		if *req.TimeoutSeconds < 60 || *req.TimeoutSeconds > 3600 {
			return nil, ErrScoutInvalidTimeout
		}
		cfg.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.SubscriptionID != nil {
		if err := s.applySubscription(ctx, cfg, project.UserID, *req.SubscriptionID); err != nil {
			return nil, err
		}
	}
	if req.IsEnabled != nil {
		cfg.IsEnabled = *req.IsEnabled
	}

	if created {
		if err := s.deps.Repo.CreateConfig(ctx, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err := s.deps.Repo.UpdateConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applySubscription валидирует и проставляет выбор подписки. Пустая строка —
// сброс на дефолтную подписку владельца (NULL). Иначе подписка должна
// принадлежать владельцу проекта (сейчас одна на пользователя).
func (s *scoutService) applySubscription(ctx context.Context, cfg *models.ScoutConfig, ownerID uuid.UUID, raw string) error {
	if raw == "" {
		cfg.SubscriptionID = nil
		return nil
	}
	subID, err := uuid.Parse(raw)
	if err != nil {
		return ErrScoutInvalidSubscriptionID
	}
	sub, err := s.deps.SubscriptionRepo.GetByUserID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, repository.ErrClaudeCodeSubscriptionNotFound) {
			return ErrScoutSubscriptionNotFound
		}
		return err
	}
	if sub.ID != subID {
		return ErrScoutSubscriptionNotFound
	}
	cfg.SubscriptionID = &subID
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Прогоны: диспатч.
// ─────────────────────────────────────────────────────────────────────────────

func (s *scoutService) Dispatch(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, problem string) (*models.ScoutRun, error) {
	return s.dispatch(ctx, userID, userRole, projectID, nil, "", problem)
}

// DispatchForSession — диспатч из распарканной сессии ассистента: прогон
// привязывается к (session_id, tool_call_id) и по завершении закрывает tool_call
// досье через ScoutResumer. Ассистент уже проверил владение сессией → RoleUser.
func (s *scoutService) DispatchForSession(ctx context.Context, userID, projectID, sessionID uuid.UUID, toolCallID, problem string) error {
	_, err := s.dispatch(ctx, userID, models.RoleUser, projectID, &sessionID, toolCallID, problem)
	return err
}

func (s *scoutService) dispatch(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, sessionID *uuid.UUID, toolCallID string, problem string) (*models.ScoutRun, error) {
	project, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID)
	if err != nil {
		return nil, err
	}

	problem = strings.TrimSpace(problem)
	if problem == "" {
		return nil, ErrScoutEmptyProblem
	}

	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	if err != nil {
		if !errors.Is(err, repository.ErrScoutConfigNotFound) {
			return nil, err
		}
		cfg = defaultScoutConfig(projectID)
	}

	repos, err := s.deps.RepoRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	primary := primaryScoutRepo(repos)
	if primary == nil {
		return nil, ErrScoutNoRepositories
	}

	// Pre-flight: собираем временный Agent из конфига и проверяем, что auth для
	// sandbox резолвится (claude oauth/api-key, hermes/antigravity — как у dev-агентов).
	// Пусто → нет подписки/ключа: fail-loud немедленно, не создавая прогон.
	scoutAgent := buildScoutAgent(cfg)
	if len(s.deps.AuthResolver.Resolve(ctx, project, scoutAgent).ToEnv()) == 0 {
		s.deps.Logger.Warn("scout: no sandbox auth resolved", "project_id", projectID, "owner_id", project.UserID, "provider_kind", providerKindStr(cfg.ProviderKind))
		return nil, ErrScoutNoSubscription
	}

	run := &models.ScoutRun{
		ID:          uuid.New(),
		ProjectID:   projectID,
		CreatedBy:   &userID,
		SessionID:   sessionID,
		Status:      models.ScoutRunStatusRunning,
		CodeBackend: cfg.CodeBackend,
		Problem:     problem,
		StartedAt:   time.Now(),
	}
	if toolCallID != "" {
		run.ToolCallID = &toolCallID
	}
	if err := s.deps.Repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	go s.executeRunWithRecovery(run, project, repos, *primary, cfg)
	return run, nil
}

// executeRunWithRecovery гоняет прогон в фоне и фиксирует исход (done/failed).
func (s *scoutService) executeRunWithRecovery(run *models.ScoutRun, project *models.Project, repos []models.ProjectRepository, primary models.ProjectRepository, cfg *models.ScoutConfig) {
	defer func() {
		if r := recover(); r != nil {
			s.deps.Logger.Error("scout: panic in run", "run_id", run.ID, "recover", r)
			s.finishRun(run, "", "", fmt.Errorf("internal panic during scout run"))
		}
	}()

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = scoutDefaultTimeoutSeconds * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dossier, sandboxID, err := s.runScout(ctx, run, project, repos, primary, cfg)
	s.finishRun(run, dossier, sandboxID, err)
}

// finishRun проставляет терминальный статус прогона и персистит его.
func (s *scoutService) finishRun(run *models.ScoutRun, dossier, sandboxID string, runErr error) {
	now := time.Now()
	run.FinishedAt = &now
	if sandboxID != "" {
		run.SandboxInstanceID = sandboxID
	}
	if runErr != nil {
		run.Status = models.ScoutRunStatusFailed
		run.Error = runErr.Error()
	} else {
		run.Status = models.ScoutRunStatusDone
		run.Dossier = dossier
	}
	if err := s.deps.Repo.UpdateRun(context.Background(), run); err != nil {
		s.deps.Logger.Error("scout: update run failed", "run_id", run.ID, "error", err)
	}

	// Wake-up: прогон привязан к распарканной сессии ассистента — закрыть
	// pending tool_call досье (или ошибкой) и поднять луп.
	if run.SessionID != nil && run.ToolCallID != nil && s.resumer != nil {
		owner := uuid.Nil
		if run.CreatedBy != nil {
			owner = *run.CreatedBy
		}
		s.resumer.ResumeFromScout(context.Background(), *run.SessionID, owner, *run.ToolCallID, run.Dossier, runErr)
	}
}

// runScout собирает ExecutionInput, гоняет sandbox-исполнитель и извлекает досье.
func (s *scoutService) runScout(ctx context.Context, run *models.ScoutRun, project *models.Project, repos []models.ProjectRepository, primary models.ProjectRepository, cfg *models.ScoutConfig) (string, string, error) {
	systemPrompt := s.scoutSystemPrompt(cfg)
	userPrompt := renderRepoCatalog(repos) + "\nПроблема пользователя:\n" + run.Problem

	// Временный Agent из конфига → тот же auth-резолвер и sandbox-bundle, что у dev-агентов.
	scoutAgent := buildScoutAgent(cfg)
	authEnv := s.deps.AuthResolver.Resolve(ctx, project, scoutAgent).ToEnv()
	if authEnv == nil {
		authEnv = map[string]string{}
	}
	// Git-креды для клонирования репо (primary + siblings клонятся тем же токеном).
	// Без этого приватный репо падает на clone (could not read Username).
	for k, v := range s.deps.GitTokenResolver.ResolveGitTokenSecrets(ctx, project, &primary) {
		authEnv[k] = v
	}
	bundle, err := s.deps.AgentSettingsSvc.BuildSandboxBundle(ctx, scoutAgent, project)
	if err != nil {
		return "", "", fmt.Errorf("build sandbox bundle: %w", err)
	}

	in := agent.ExecutionInput{
		TaskID:           run.ID.String(),
		ProjectID:        run.ProjectID.String(),
		OwnerUserID:      project.UserID.String(),
		GitURL:           primary.GitURL,
		GitDefaultBranch: primary.GitDefaultBranch,
		// Скаут читает дефолтную ветку; коммитов нет → push в entrypoint скипается.
		// Ветка-черновик нужна только как имя локального чекаута.
		BranchName:    "scout-" + run.ID.String()[:8],
		CodeBackend:   string(cfg.CodeBackend),
		Provider:      providerKindStr(cfg.ProviderKind),
		Model:         scoutModelFromSettings(cfg.CodeBackendSettings),
		Temperature:   cfg.Temperature,
		PromptSystem:  systemPrompt,
		PromptUser:    userPrompt,
		SiblingRepos:  scoutSiblings(repos, primary.Slug),
		AgentSettings: bundle,
		EnvSecrets:    authEnv,
	}

	res, err := s.deps.SandboxExecutor.Execute(ctx, in)
	if err != nil {
		return "", "", err
	}
	sandboxID := res.SandboxInstanceID

	// Успех меряем НЕ res.Success (его контракт заточен под коммит/дифф dev-агента),
	// а наличием досье в выводе: для read-only прогона диффа нет.
	dossier := extractDossier(res.Output)
	if strings.TrimSpace(dossier) == "" {
		return "", sandboxID, fmt.Errorf("scout produced no dossier (sandbox summary: %s)", res.Summary)
	}
	return dossier, sandboxID, nil
}

// scoutSystemPrompt — редактируемый промпт (или дефолт) + обязательный контракт вывода.
func (s *scoutService) scoutSystemPrompt(cfg *models.ScoutConfig) string {
	base := strings.TrimSpace(cfg.Prompt)
	if base == "" {
		base = scoutDefaultSystemPrompt
	}
	return base + scoutOutputContract
}

// ─────────────────────────────────────────────────────────────────────────────
// Прогоны: чтение.
// ─────────────────────────────────────────────────────────────────────────────

func (s *scoutService) GetRun(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, runID uuid.UUID) (*models.ScoutRun, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	run, err := s.deps.Repo.GetRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrScoutRunNotFound) {
			return nil, ErrScoutRunNotFound
		}
		return nil, err
	}
	// Прогон обязан принадлежать проекту (защита от cross-project доступа по UUID).
	if run.ProjectID != projectID {
		return nil, ErrScoutRunNotFound
	}
	return run, nil
}

func (s *scoutService) ListRuns(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ScoutRun, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	return s.deps.Repo.ListRunsByProjectID(ctx, projectID, 50)
}

// ─────────────────────────────────────────────────────────────────────────────
// Хелперы.
// ─────────────────────────────────────────────────────────────────────────────

// primaryScoutRepo выбирает целевой (writable) репозиторий: IsPrimary, иначе
// первый с непустым GitURL.
func primaryScoutRepo(repos []models.ProjectRepository) *models.ProjectRepository {
	for i := range repos {
		if repos[i].IsPrimary && repos[i].GitURL != "" {
			return &repos[i]
		}
	}
	for i := range repos {
		if repos[i].GitURL != "" {
			return &repos[i]
		}
	}
	return nil
}

// scoutSiblings — все репозитории проекта кроме primary (read-only в sandbox).
func scoutSiblings(repos []models.ProjectRepository, primarySlug string) []agent.SiblingRepo {
	var out []agent.SiblingRepo
	for i := range repos {
		r := &repos[i]
		if r.Slug == primarySlug || r.GitURL == "" {
			continue
		}
		out = append(out, agent.SiblingRepo{Slug: r.Slug, GitURL: r.GitURL, Branch: r.GitDefaultBranch})
	}
	return out
}

// extractDossier вырезает текст между маркерами; при их отсутствии возвращает
// весь вывод (fallback — лучше шумное досье, чем пустое).
func extractDossier(output string) string {
	begin := strings.Index(output, scoutDossierBeginMarker)
	if begin == -1 {
		return strings.TrimSpace(output)
	}
	rest := output[begin+len(scoutDossierBeginMarker):]
	if end := strings.Index(rest, scoutDossierEndMarker); end != -1 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}

// buildScoutAgent собирает временный Agent из конфига разведчика (всегда sandbox):
// его жуёт тот же auth-резолвер и AgentSettingsService, что dev-агентов.
func buildScoutAgent(cfg *models.ScoutConfig) *models.Agent {
	cb := cfg.CodeBackend
	return &models.Agent{
		Name:                "scout",
		ExecutionKind:       models.AgentExecutionKindSandbox,
		CodeBackend:         &cb,
		ProviderKind:        cfg.ProviderKind,
		Temperature:         cfg.Temperature,
		CodeBackendSettings: cfg.CodeBackendSettings,
		SandboxPermissions:  cfg.SandboxPermissions,
	}
}

// scoutProviderAllowedForBackend — те же правила, что agent_provider_rules на фронте.
func scoutProviderAllowedForBackend(backend models.CodeBackend, pk models.AgentProviderKind) bool {
	switch backend {
	case models.CodeBackendHermes:
		return pk == models.AgentProviderKindAnthropic ||
			pk == models.AgentProviderKindOpenRouter ||
			pk == models.AgentProviderKindHermes
	case models.CodeBackendAntigravity:
		return pk == models.AgentProviderKindAntigravity ||
			pk == models.AgentProviderKindAntigravityOAuth
	default:
		return true
	}
}

func providerKindStr(pk *models.AgentProviderKind) string {
	if pk == nil {
		return ""
	}
	return string(*pk)
}

// scoutModelFromSettings достаёт model из code_backend_settings (как у sandbox-агента).
func scoutModelFromSettings(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var s AgentCodeBackendSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s.Model
}
