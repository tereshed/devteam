package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/llm"
)

var (
	ErrEnhancerInvalidCron     = errors.New("invalid enhancer cron expression")
	ErrEnhancerInvalidAutonomy = errors.New("only 'propose' autonomy is supported")
	ErrEnhancerInvalidWindow   = errors.New("analysis_window_days must be between 1 and 90")
	ErrEnhancerInvalidLimit    = errors.New("max_changes_per_run must be between 1 and 20")
	ErrEnhancerRunInProgress   = errors.New("enhancer run is already in progress")
	ErrEnhancerRunNotFound     = errors.New("enhancer run not found")
)

const (
	// EnhancerRunTimeout — hard timeout на весь прогон (agentloop с tool-вызовами).
	// Прогоны 'running' старше EnhancerRunStaleAfter считаются упавшими и гасятся
	// при следующей проверке busy-guard (краш-восстановление).
	EnhancerRunTimeout    = 15 * time.Minute
	EnhancerRunStaleAfter = EnhancerRunTimeout + time.Minute

	// EnhancerMaxAddendumChars — гардрейл на размер prompt_addendum в предложении
	// agent_override: раздутые промпты сами по себе деградируют качество и стоимость
	// (урок prompt-induced OOM, миграции 065→068). Enforced в Go, не в промпте.
	EnhancerMaxAddendumChars = 4000
	// EnhancerMaxPayloadBytes — гардрейл на размер payload предложения целиком.
	EnhancerMaxPayloadBytes = 16 * 1024
	// EnhancerMaxTextChars — лимит reason / expected_effect / report от модели.
	EnhancerMaxTextChars = 8000

	enhancerDefaultWindowDays = 7
	enhancerDefaultMaxChanges = 5
	enhancerRunsListLimit     = 20
)

// enhancerReadToolNames — whitelist read-only инструментов из общего каталога
// ассистента (AuthorizedExecutor), доступных энхансеру. Изоляция по проекту
// enforced самими handler'ами через AuthContext.ProjectID.
var enhancerReadToolNames = map[string]bool{
	"project_get":          true,
	"task_list":            true,
	"task_get":             true,
	"artifact_list":        true,
	"artifact_get":         true,
	"router_decision_list": true,
	"agent_list":           true,
	"agent_get":            true,
	"team_list":            true,
	"team_get":             true,
}

// EnhancerService — бизнес-логика энхансера: per-project конфиг, прогоны
// анализа и предложения изменений (enhancer_changes).
type EnhancerService interface {
	GetConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.EnhancerConfig, error)
	UpdateConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateEnhancerConfigRequest) (*models.EnhancerConfig, error)
	// RunNow — ручной запуск прогона (работает и при is_active=false).
	RunNow(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.EnhancerRun, error)
	ListRuns(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.EnhancerRun, error)
	ListRunChanges(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, runID uuid.UUID) ([]models.EnhancerChange, error)
	// RunDue — вызывается leader-gated раннером: запускает прогоны по всем
	// созревшим конфигам и пересчитывает их next_run_at.
	RunDue(ctx context.Context, now time.Time) (int, error)
}

// EnhancerServiceDeps — DI-bag конструктора.
type EnhancerServiceDeps struct {
	Repo       repository.EnhancerRepository
	ProjectSvc ProjectService
	TeamRepo   repository.TeamRepository
	UserRepo   repository.UserRepository
	// TaskRepo / TaskMsgRepo — для локального read-инструмента task_message_list:
	// фидбек пользователя в задачах (message_type=feedback) — ключевой вход анализа.
	TaskRepo    repository.TaskRepository
	TaskMsgRepo repository.TaskMessageRepository
	AgentSvc    *AgentService
	// LLMResolver — тот же адаптер, что у ассистента: ключи берутся из
	// user_llm_credentials владельца (user keys over ENV).
	LLMResolver AssistantLLMClientResolver
	// ToolCatalog — общий каталог ассистента (AuthorizedExecutor); энхансер
	// видит только whitelist read-инструментов из него.
	ToolCatalog AssistantToolCatalogProvider
	Executor    *agentloop.Executor
	Logger      *slog.Logger
}

// NewEnhancerService создаёт сервис энхансера.
func NewEnhancerService(deps EnhancerServiceDeps) (EnhancerService, error) {
	if deps.Repo == nil {
		return nil, errors.New("EnhancerService: Repo is required")
	}
	if deps.ProjectSvc == nil {
		return nil, errors.New("EnhancerService: ProjectSvc is required")
	}
	if deps.TeamRepo == nil {
		return nil, errors.New("EnhancerService: TeamRepo is required")
	}
	if deps.UserRepo == nil {
		return nil, errors.New("EnhancerService: UserRepo is required")
	}
	if deps.TaskRepo == nil {
		return nil, errors.New("EnhancerService: TaskRepo is required")
	}
	if deps.TaskMsgRepo == nil {
		return nil, errors.New("EnhancerService: TaskMsgRepo is required")
	}
	if deps.AgentSvc == nil {
		return nil, errors.New("EnhancerService: AgentSvc is required")
	}
	if deps.LLMResolver == nil {
		return nil, errors.New("EnhancerService: LLMResolver is required")
	}
	if deps.ToolCatalog == nil {
		return nil, errors.New("EnhancerService: ToolCatalog is required")
	}
	if deps.Executor == nil {
		return nil, errors.New("EnhancerService: Executor is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &enhancerService{deps: deps}, nil
}

type enhancerService struct {
	deps EnhancerServiceDeps
}

// ─────────────────────────────────────────────────────────────────────────────
// Конфиг.
// ─────────────────────────────────────────────────────────────────────────────

// defaultEnhancerConfig — синтетический дефолт для проектов без сохранённого
// конфига (GET до первого PUT). Не персистится.
func defaultEnhancerConfig(projectID uuid.UUID) *models.EnhancerConfig {
	return &models.EnhancerConfig{
		ProjectID:          projectID,
		IsActive:           false,
		Autonomy:           models.EnhancerAutonomyPropose,
		AnalysisWindowDays: enhancerDefaultWindowDays,
		MaxChangesPerRun:   enhancerDefaultMaxChanges,
	}
}

func (s *enhancerService) GetConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.EnhancerConfig, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrEnhancerConfigNotFound) {
			return defaultEnhancerConfig(projectID), nil
		}
		return nil, err
	}
	return cfg, nil
}

func (s *enhancerService) UpdateConfig(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateEnhancerConfigRequest) (*models.EnhancerConfig, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}

	cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID)
	created := false
	if err != nil {
		if !errors.Is(err, repository.ErrEnhancerConfigNotFound) {
			return nil, err
		}
		cfg = defaultEnhancerConfig(projectID)
		cfg.ID = uuid.New()
		cfg.CreatedBy = userID
		created = true
	}

	if req.Autonomy != nil {
		// Фаза 1: только propose. auto_apply появится вместе с замером эффекта.
		if models.EnhancerAutonomy(*req.Autonomy) != models.EnhancerAutonomyPropose {
			return nil, ErrEnhancerInvalidAutonomy
		}
		cfg.Autonomy = models.EnhancerAutonomyPropose
	}
	if req.AnalysisWindowDays != nil {
		if *req.AnalysisWindowDays < 1 || *req.AnalysisWindowDays > 90 {
			return nil, ErrEnhancerInvalidWindow
		}
		cfg.AnalysisWindowDays = *req.AnalysisWindowDays
	}
	if req.MaxChangesPerRun != nil {
		if *req.MaxChangesPerRun < 1 || *req.MaxChangesPerRun > 20 {
			return nil, ErrEnhancerInvalidLimit
		}
		cfg.MaxChangesPerRun = *req.MaxChangesPerRun
	}
	if req.CronExpression != nil {
		trimmed := strings.TrimSpace(*req.CronExpression)
		if trimmed == "" {
			cfg.CronExpression = nil
		} else {
			if _, err := parseCron(trimmed); err != nil {
				return nil, ErrEnhancerInvalidCron
			}
			cfg.CronExpression = &trimmed
		}
	}
	if req.IsActive != nil {
		cfg.IsActive = *req.IsActive
	}

	// next_run_at живёт только у активного конфига с расписанием.
	if cfg.IsActive && cfg.CronExpression != nil {
		sched, err := parseCron(*cfg.CronExpression)
		if err != nil {
			return nil, ErrEnhancerInvalidCron
		}
		next := sched.Next(time.Now())
		cfg.NextRunAt = &next
	} else {
		cfg.NextRunAt = nil
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

// ─────────────────────────────────────────────────────────────────────────────
// Прогоны: запуск.
// ─────────────────────────────────────────────────────────────────────────────

func (s *enhancerService) RunNow(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.EnhancerRun, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}

	windowDays, maxChanges := enhancerDefaultWindowDays, enhancerDefaultMaxChanges
	var configID *uuid.UUID
	if cfg, err := s.deps.Repo.GetConfigByProjectID(ctx, projectID); err == nil {
		windowDays, maxChanges = cfg.AnalysisWindowDays, cfg.MaxChangesPerRun
		configID = &cfg.ID
	} else if !errors.Is(err, repository.ErrEnhancerConfigNotFound) {
		return nil, err
	}

	busy, err := s.deps.Repo.HasRunningRun(ctx, projectID, EnhancerRunStaleAfter)
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, ErrEnhancerRunInProgress
	}

	run := &models.EnhancerRun{
		ID:          uuid.New(),
		ProjectID:   projectID,
		ConfigID:    configID,
		TriggerKind: models.EnhancerRunTriggerManual,
		Status:      models.EnhancerRunStatusRunning,
		StartedAt:   time.Now(),
	}
	if err := s.deps.Repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	// Владелец ручного прогона — сам запускающий: его enhancer-агент и LLM-ключи.
	go s.executeRunWithRecovery(run, userID, windowDays, maxChanges)
	return run, nil
}

func (s *enhancerService) RunDue(ctx context.Context, now time.Time) (int, error) {
	due, err := s.deps.Repo.ListDueConfigs(ctx, now, 0)
	if err != nil {
		return 0, err
	}
	fired := 0
	for i := range due {
		cfg := due[i]
		// next_run_at пересчитываем всегда (даже при busy-skip и ошибках),
		// чтобы не зациклить раннер на «сбойном» конфиге.
		cfg.LastRunAt = &now
		s.advanceNextRun(ctx, &cfg, now)

		busy, err := s.deps.Repo.HasRunningRun(ctx, cfg.ProjectID, EnhancerRunStaleAfter)
		if err != nil {
			s.deps.Logger.Error("enhancer: busy check failed", "project_id", cfg.ProjectID, "error", err)
			continue
		}
		if busy {
			s.deps.Logger.Info("enhancer: run skipped, already in progress", "project_id", cfg.ProjectID)
			continue
		}

		run := &models.EnhancerRun{
			ID:          uuid.New(),
			ProjectID:   cfg.ProjectID,
			ConfigID:    &cfg.ID,
			TriggerKind: models.EnhancerRunTriggerCron,
			Status:      models.EnhancerRunStatusRunning,
			StartedAt:   now,
		}
		if err := s.deps.Repo.CreateRun(ctx, run); err != nil {
			s.deps.Logger.Error("enhancer: create run failed", "project_id", cfg.ProjectID, "error", err)
			continue
		}
		go s.executeRunWithRecovery(run, cfg.CreatedBy, cfg.AnalysisWindowDays, cfg.MaxChangesPerRun)
		fired++
	}
	return fired, nil
}

// advanceNextRun пересчитывает next_run_at (или гасит конфиг при невалидном cron).
func (s *enhancerService) advanceNextRun(ctx context.Context, cfg *models.EnhancerConfig, now time.Time) {
	if cfg.CronExpression == nil {
		cfg.NextRunAt = nil
	} else if sched, err := parseCron(*cfg.CronExpression); err != nil {
		s.deps.Logger.Error("enhancer: invalid cron in db, deactivating", "config_id", cfg.ID)
		cfg.IsActive = false
		cfg.NextRunAt = nil
	} else {
		next := sched.Next(now)
		cfg.NextRunAt = &next
	}
	if err := s.deps.Repo.UpdateConfig(ctx, cfg); err != nil {
		s.deps.Logger.Error("enhancer: update next_run failed", "config_id", cfg.ID, "error", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Прогоны: чтение.
// ─────────────────────────────────────────────────────────────────────────────

func (s *enhancerService) ListRuns(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.EnhancerRun, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	return s.deps.Repo.ListRunsByProjectID(ctx, projectID, enhancerRunsListLimit)
}

func (s *enhancerService) ListRunChanges(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, runID uuid.UUID) ([]models.EnhancerChange, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	run, err := s.deps.Repo.GetRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrEnhancerRunNotFound) {
			return nil, ErrEnhancerRunNotFound
		}
		return nil, err
	}
	if run.ProjectID != projectID {
		return nil, ErrEnhancerRunNotFound
	}
	return s.deps.Repo.ListChangesByRunID(ctx, runID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Исполнение прогона: agentloop с read-каталогом + propose/finish инструментами.
// ─────────────────────────────────────────────────────────────────────────────

// enhancerRunSink — приёмник результатов прогона из tool-замыканий.
type enhancerRunSink struct {
	mu        sync.Mutex
	report    string
	proposals int
}

func (s *enhancerService) executeRunWithRecovery(run *models.EnhancerRun, ownerID uuid.UUID, windowDays, maxChanges int) {
	ctx, cancel := context.WithTimeout(context.Background(), EnhancerRunTimeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			s.deps.Logger.Error("enhancer: panic in run",
				"run_id", run.ID, "panic", fmt.Sprintf("%v", r), "stack", string(debug.Stack()))
			s.finishRun(run, models.EnhancerRunStatusFailed, "", "внутренняя ошибка прогона")
		}
	}()
	s.executeRun(ctx, run, ownerID, windowDays, maxChanges)
}

func (s *enhancerService) executeRun(ctx context.Context, run *models.EnhancerRun, ownerID uuid.UUID, windowDays, maxChanges int) {
	owner, err := s.deps.UserRepo.GetByID(ctx, ownerID)
	if err != nil {
		s.failRun(run, fmt.Errorf("owner lookup: %w", err), "владелец конфига не найден")
		return
	}
	project, err := s.deps.ProjectSvc.GetByID(ctx, ownerID, owner.Role, run.ProjectID)
	if err != nil {
		s.failRun(run, fmt.Errorf("load project: %w", err), "проект недоступен владельцу конфига")
		return
	}

	agent, err := s.deps.AgentSvc.EnsureEnhancerAgent(ctx, ownerID)
	if err != nil {
		s.failRun(run, fmt.Errorf("ensure enhancer agent: %w", err), "не удалось получить enhancer-агента")
		return
	}
	if !agent.IsActive {
		s.failRun(run, errors.New("enhancer agent inactive"), "enhancer-агент отключён в настройках агентов")
		return
	}

	// LLM-настройки: свои у enhancer-агента; если не заданы — фолбэк на
	// ассистента владельца (тот обычно уже сконфигурирован). Fail-loud, если
	// нет ни того, ни другого.
	llmAgent := agent
	if agent.ProviderKind == nil || !agent.ProviderKind.IsValid() || agent.Model == nil || strings.TrimSpace(derefStringEmpty(agent.Model)) == "" {
		if assistant, aerr := s.deps.AgentSvc.EnsureAssistantAgent(ctx, ownerID); aerr == nil &&
			assistant.ProviderKind != nil && assistant.ProviderKind.IsValid() &&
			assistant.Model != nil && strings.TrimSpace(*assistant.Model) != "" {
			llmAgent = assistant
		}
	}
	client, err := s.deps.LLMResolver.ResolveAssistantClient(ctx, llmAgent, ownerID)
	if err != nil {
		s.failRun(run, fmt.Errorf("resolve llm client: %w", err), "LLM-провайдер не настроен: добавьте ключ провайдера в настройках и/или настройте агента enhancer")
		return
	}

	tools, sink := s.buildRunTools(run, maxChanges)
	sysPrompt := s.buildSystemPrompt(agent, project, windowDays, maxChanges)

	model, provider := "", ""
	if llmAgent.Model != nil {
		model = *llmAgent.Model
	}
	if llmAgent.ProviderKind != nil {
		provider = string(*llmAgent.ProviderKind)
	}

	result, runErr := s.deps.Executor.Run(ctx, agentloop.RunRequest{
		Client:       client,
		Provider:     provider,
		Model:        model,
		SystemPrompt: sysPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		History: []agentloop.Message{{
			Role: llm.RoleUser,
			Content: "Проведи аудит выполнения задач проекта за окно анализа и сформируй предложения улучшений. " +
				"Начни с task_list по проекту, выдели проблемные задачи (failed/needs_human, много шагов), " +
				"исследуй их через router_decision_list, artifact_list и task_get. " +
				"Прочитай текущие промпты агентов проекта (team_list, agent_get) прежде чем предлагать правки. " +
				"Каждое улучшение оформи отдельным вызовом enhancer_propose_change. " +
				"В конце ОБЯЗАТЕЛЬНО вызови enhancer_finish_run с итоговым отчётом.",
		}},
		Tools: tools,
		Auth: agentloop.AuthContext{
			UserID:    ownerID.String(),
			ProjectID: run.ProjectID.String(),
			Scope:     "enhancer",
		},
		Hooks: agentloop.Hooks{},
	})
	if runErr != nil {
		s.failRun(run, fmt.Errorf("executor config error: %w", runErr), "внутренняя ошибка конфигурации прогона")
		return
	}

	sink.mu.Lock()
	report := sink.report
	proposals := sink.proposals
	sink.mu.Unlock()

	switch result.Status {
	case agentloop.StatusCompleted:
		if report == "" {
			// Агент не вызвал enhancer_finish_run — берём финальный текст,
			// чтобы наблюдения не потерялись.
			report = strings.TrimSpace(result.LastAssistantText)
		}
		s.finishRun(run, models.EnhancerRunStatusDone, report, "")
		s.deps.Logger.Info("enhancer: run completed",
			"run_id", run.ID, "project_id", run.ProjectID,
			"iterations", result.Iterations, "proposals", proposals)
	case agentloop.StatusLimitExceeded:
		if report != "" {
			// Отчёт успели зафиксировать до лимита — прогон содержательный.
			s.finishRun(run, models.EnhancerRunStatusDone, report, "")
			return
		}
		s.failRun(run, errors.New("loop limit exceeded"), "превышен лимит шагов анализа")
	case agentloop.StatusParked:
		// Невозможен: в каталоге энхансера нет confirm-инструментов.
		s.failRun(run, errors.New("unexpected parked status"), "внутренняя ошибка прогона")
	case agentloop.StatusFailed:
		cause := "ошибка LLM-вызова"
		if result.Cause != nil && isCtxTimeoutErr(result.Cause) {
			cause = "прогон не уложился в таймаут"
		}
		s.failRun(run, result.Cause, cause)
	}
}

func (s *enhancerService) failRun(run *models.EnhancerRun, cause error, userMsg string) {
	s.deps.Logger.Error("enhancer: run failed", "run_id", run.ID, "project_id", run.ProjectID, "error", cause)
	s.finishRun(run, models.EnhancerRunStatusFailed, "", userMsg)
}

// finishRun фиксирует терминальное состояние прогона. Пишем через
// context.Background: ctx прогона к этому моменту может быть уже отменён.
func (s *enhancerService) finishRun(run *models.EnhancerRun, status models.EnhancerRunStatus, report, errMsg string) {
	now := time.Now()
	run.Status = status
	run.Report = truncateRunes(report, EnhancerMaxTextChars)
	run.Error = errMsg
	run.FinishedAt = &now
	if err := s.deps.Repo.UpdateRun(context.Background(), run); err != nil {
		s.deps.Logger.Error("enhancer: persist run result failed", "run_id", run.ID, "error", err)
	}
}

func (s *enhancerService) buildSystemPrompt(agent *models.Agent, project *models.Project, windowDays, maxChanges int) string {
	var sb strings.Builder
	if agent.SystemPrompt != nil {
		sb.WriteString(strings.TrimSpace(*agent.SystemPrompt))
	}
	sb.WriteString("\n\n=== КОНТЕКСТ ПРОГОНА ===\n")
	fmt.Fprintf(&sb, "Проект: %q (ID: %s)\n", project.Name, project.ID.String())
	if strings.TrimSpace(project.Description) != "" {
		fmt.Fprintf(&sb, "Текущее описание проекта:\n%s\n", project.Description)
	}
	now := time.Now()
	from := now.AddDate(0, 0, -windowDays)
	fmt.Fprintf(&sb, "Окно анализа: последние %d дней (%s — %s). Задачи старше окна игнорируй.\n",
		windowDays, from.Format("2006-01-02"), now.Format("2006-01-02"))
	fmt.Fprintf(&sb, "Лимит предложений за прогон: %d (enforced системой).\n", maxChanges)
	sb.WriteString("Режим: propose — каждое предложение попадёт на ревью человеку, ничего не применяется автоматически.\n")
	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Tool-каталог прогона.
// ─────────────────────────────────────────────────────────────────────────────

var schemaEnhancerProposeChange = json.RawMessage(`{
	"type": "object",
	"properties": {
		"target_kind": {
			"type": "string",
			"enum": ["agent_override", "project_description", "project_settings"],
			"description": "Тип цели: agent_override — проектный оверрайд промпта/настроек агента; project_description — новая редакция описания проекта; project_settings — правка настроек проекта."
		},
		"target_agent_id": {
			"type": "string",
			"description": "UUID агента проекта (обязателен для agent_override; агент обязан принадлежать командам этого проекта)."
		},
		"payload": {
			"type": "object",
			"description": "Самодостаточный дифф. agent_override: {\"prompt_addendum\": \"...\"} и/или {\"settings\": {...}}. project_description / project_settings: {\"old\": ..., \"new\": ...}."
		},
		"reason": {"type": "string", "description": "На каких наблюдениях основано (конкретные task_id и что в них пошло не так)."},
		"expected_effect": {"type": "string", "description": "Ожидаемый измеримый эффект (меньше итераций ревью / needs_human / шагов роутера)."}
	},
	"required": ["target_kind", "payload", "reason", "expected_effect"]
}`)

var schemaEnhancerTaskMessages = json.RawMessage(`{
	"type": "object",
	"properties": {
		"task_id": {"type": "string", "description": "UUID задачи этого проекта."},
		"message_type": {"type": "string", "description": "Опциональный фильтр: instruction | result | question | feedback | error | comment | summary."},
		"limit": {"type": "integer", "description": "Максимум сообщений (по умолчанию 20, максимум 50)."}
	},
	"required": ["task_id"]
}`)

var schemaEnhancerFinishRun = json.RawMessage(`{
	"type": "object",
	"properties": {
		"report": {"type": "string", "description": "Итоговый отчёт прогона (markdown): что проанализировано, какие проблемы найдены (с task_id), какие предложения сделаны и почему."}
	},
	"required": ["report"]
}`)

func (s *enhancerService) buildRunTools(run *models.EnhancerRun, maxChanges int) ([]agentloop.Tool, *enhancerRunSink) {
	sink := &enhancerRunSink{}
	tools := make([]agentloop.Tool, 0, len(enhancerReadToolNames)+2)
	for _, t := range s.deps.ToolCatalog.Catalog() {
		// Только whitelist и только не-destructive: защита от расширения
		// общего каталога ассистента write-инструментами в будущем.
		if enhancerReadToolNames[t.Name] && !t.RequiresConfirmation {
			tools = append(tools, t)
		}
	}
	tools = append(tools,
		agentloop.Tool{
			Name: "task_message_list",
			Description: "Сообщения задачи (общение агентов и пользователя). Особенно ценны message_type=feedback — " +
				"прямой фидбек пользователя на работу агентов.",
			InputSchema: schemaEnhancerTaskMessages,
			Handler:     s.makeTaskMessageListHandler(run),
		},
		agentloop.Tool{
			Name: "enhancer_propose_change",
			Description: "Зафиксировать предложение улучшения проекта. Предложение попадёт на ревью человеку. " +
				"Каждое предложение обязано опираться на проверенные инструментами наблюдения (reason) и иметь ожидаемый эффект (expected_effect).",
			InputSchema: schemaEnhancerProposeChange,
			Handler:     s.makeProposeChangeHandler(run, maxChanges, sink),
		},
		agentloop.Tool{
			Name:        "enhancer_finish_run",
			Description: "Завершить прогон итоговым отчётом. Вызывается один раз в самом конце анализа.",
			InputSchema: schemaEnhancerFinishRun,
			Handler:     makeFinishRunHandler(sink),
		},
	)
	return tools, sink
}

// enhancerToolErr — payload бизнес-ошибки для LLM (петлю не прерывает).
func enhancerToolErr(status, message string) (json.RawMessage, error) {
	b, _ := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{status, message})
	return b, nil
}

func enhancerToolOK(data any) (json.RawMessage, error) {
	b, err := json.Marshal(struct {
		Status string `json:"status"`
		Data   any    `json:"data,omitempty"`
	}{"ok", data})
	if err != nil {
		return nil, err
	}
	return b, nil
}

type enhancerProposeArgs struct {
	TargetKind     string          `json:"target_kind"`
	TargetAgentID  string          `json:"target_agent_id,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Reason         string          `json:"reason"`
	ExpectedEffect string          `json:"expected_effect"`
}

func (s *enhancerService) makeProposeChangeHandler(run *models.EnhancerRun, maxChanges int, sink *enhancerRunSink) agentloop.ToolHandler {
	return func(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
		var a enhancerProposeArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return enhancerToolErr("validation", "невалидные аргументы: "+err.Error())
		}

		kind := models.EnhancerChangeKind(a.TargetKind)
		if !kind.IsValid() {
			return enhancerToolErr("validation", "target_kind должен быть agent_override | project_description | project_settings")
		}
		if strings.TrimSpace(a.Reason) == "" || strings.TrimSpace(a.ExpectedEffect) == "" {
			return enhancerToolErr("validation", "reason и expected_effect обязательны")
		}
		if len(a.Payload) == 0 || string(a.Payload) == "null" {
			return enhancerToolErr("validation", "payload обязателен")
		}
		if len(a.Payload) > EnhancerMaxPayloadBytes {
			return enhancerToolErr("validation", fmt.Sprintf("payload превышает лимит %d байт — сократи предложение", EnhancerMaxPayloadBytes))
		}

		var targetAgentID *uuid.UUID
		if kind == models.EnhancerChangeKindAgentOverride {
			if strings.TrimSpace(a.TargetAgentID) == "" {
				return enhancerToolErr("validation", "target_agent_id обязателен для agent_override")
			}
			id, err := uuid.Parse(a.TargetAgentID)
			if err != nil {
				return enhancerToolErr("validation", "target_agent_id должен быть UUID")
			}
			ok, err := s.agentBelongsToProject(ctx, run.ProjectID, id)
			if err != nil {
				return nil, fmt.Errorf("validate agent membership: %w", err)
			}
			if !ok {
				return enhancerToolErr("forbidden", "агент не принадлежит командам этого проекта — глобальные агенты менять запрещено")
			}
			// Гардрейл размера аддендума — enforced в Go, не в промпте.
			var payloadFields struct {
				PromptAddendum string `json:"prompt_addendum"`
			}
			if err := json.Unmarshal(a.Payload, &payloadFields); err == nil {
				if len([]rune(payloadFields.PromptAddendum)) > EnhancerMaxAddendumChars {
					return enhancerToolErr("validation", fmt.Sprintf("prompt_addendum превышает лимит %d символов — добавка к промпту должна быть точечной", EnhancerMaxAddendumChars))
				}
			}
			targetAgentID = &id
		}

		count, err := s.deps.Repo.CountChangesByRunID(ctx, run.ID)
		if err != nil {
			return nil, fmt.Errorf("count changes: %w", err)
		}
		if count >= int64(maxChanges) {
			return enhancerToolErr("limit_exceeded", fmt.Sprintf("лимит предложений за прогон (%d) исчерпан — заверши анализ через enhancer_finish_run", maxChanges))
		}

		change := &models.EnhancerChange{
			ID:             uuid.New(),
			RunID:          run.ID,
			ProjectID:      run.ProjectID,
			TargetKind:     kind,
			TargetAgentID:  targetAgentID,
			Payload:        datatypes.JSON(a.Payload),
			Reason:         truncateRunes(a.Reason, EnhancerMaxTextChars),
			ExpectedEffect: truncateRunes(a.ExpectedEffect, EnhancerMaxTextChars),
			Status:         models.EnhancerChangeStatusProposed,
		}
		if err := s.deps.Repo.CreateChange(ctx, change); err != nil {
			return nil, fmt.Errorf("persist change: %w", err)
		}

		sink.mu.Lock()
		sink.proposals++
		sink.mu.Unlock()

		return enhancerToolOK(map[string]any{
			"change_id": change.ID.String(),
			"remaining": maxChanges - int(count) - 1,
		})
	}
}

func (s *enhancerService) makeTaskMessageListHandler(run *models.EnhancerRun) agentloop.ToolHandler {
	return func(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
		var a struct {
			TaskID      string `json:"task_id"`
			MessageType string `json:"message_type,omitempty"`
			Limit       int    `json:"limit,omitempty"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return enhancerToolErr("validation", "невалидные аргументы: "+err.Error())
		}
		taskID, err := uuid.Parse(a.TaskID)
		if err != nil {
			return enhancerToolErr("validation", "task_id должен быть UUID")
		}
		task, err := s.deps.TaskRepo.GetByID(ctx, taskID)
		if err != nil {
			if errors.Is(err, repository.ErrTaskNotFound) {
				return enhancerToolErr("not_found", "задача не найдена")
			}
			return nil, fmt.Errorf("load task: %w", err)
		}
		// Граница изоляции: только задачи проекта прогона.
		if task.ProjectID != run.ProjectID {
			return enhancerToolErr("forbidden", "задача принадлежит другому проекту")
		}

		limit := a.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}
		filter := repository.TaskMessageFilter{Limit: limit}
		if strings.TrimSpace(a.MessageType) != "" {
			mt := models.MessageType(a.MessageType)
			filter.MessageType = &mt
		}
		msgs, total, err := s.deps.TaskMsgRepo.ListByTaskID(ctx, taskID, filter)
		if err != nil {
			return nil, fmt.Errorf("list task messages: %w", err)
		}
		items := make([]map[string]any, 0, len(msgs))
		for i := range msgs {
			m := &msgs[i]
			items = append(items, map[string]any{
				"sender_type":  m.SenderType,
				"message_type": m.MessageType,
				"content":      m.Content,
				"created_at":   m.CreatedAt,
			})
		}
		return enhancerToolOK(map[string]any{"items": items, "total": total})
	}
}

func makeFinishRunHandler(sink *enhancerRunSink) agentloop.ToolHandler {
	return func(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
		var a struct {
			Report string `json:"report"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return enhancerToolErr("validation", "невалидные аргументы: "+err.Error())
		}
		if strings.TrimSpace(a.Report) == "" {
			return enhancerToolErr("validation", "report обязателен")
		}
		sink.mu.Lock()
		sink.report = strings.TrimSpace(a.Report)
		sink.mu.Unlock()
		return enhancerToolOK(map[string]string{"message": "отчёт зафиксирован, можно завершать"})
	}
}

// agentBelongsToProject — true, если агент входит в одну из команд проекта.
// Это граница изоляции энхансера: глобальные/чужие агенты недоступны.
func (s *enhancerService) agentBelongsToProject(ctx context.Context, projectID, agentID uuid.UUID) (bool, error) {
	_, err := s.deps.TeamRepo.GetAgentInProject(ctx, projectID, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Раннер (leader-gated тикер).
// ─────────────────────────────────────────────────────────────────────────────

// enhancerRunner — leader-gated тикер, периодически вызывающий RunDue.
type enhancerRunner struct {
	svc      EnhancerService
	interval time.Duration
	logger   *slog.Logger
}

// NewEnhancerRunner создаёт раннер энхансера. interval<=0 → 1 минута.
func NewEnhancerRunner(svc EnhancerService, interval time.Duration, logger *slog.Logger) *enhancerRunner {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &enhancerRunner{svc: svc, interval: interval, logger: logger}
}

// Run блокируется до отмены ctx, периодически запуская созревшие конфиги.
func (r *enhancerRunner) Run(ctx context.Context) {
	r.logger.Info("enhancer runner started", "interval", r.interval.String())
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	r.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("enhancer runner stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *enhancerRunner) tick(ctx context.Context) {
	fired, err := r.svc.RunDue(ctx, time.Now())
	if err != nil {
		r.logger.Error("enhancer runner tick failed", "error", err)
		return
	}
	if fired > 0 {
		r.logger.Info("enhancer runner tick", "fired", fired)
	}
}
