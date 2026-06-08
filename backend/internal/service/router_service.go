package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// router_service.go — Sprint 17 / Orchestration v2 — Router-агент.
//
// Цикл вызова на одном Orchestrator.Step:
//   1. Загружается RouterState (task + agents registry + artifacts metadata + in-flight jobs).
//   2. RouterService.Decide:
//      a) Строит user-prompt из state (метаданные only, никаких content — context budget).
//      b) Вызывает router-агента через AgentDispatcher → LLM.
//      c) Снимает markdown fences с ответа.
//      d) json.Unmarshal в Decision.
//      e) Валидирует: agent-имена существуют, нет дублей target_artifact_id.
//      f) На ошибке парсинга/валидации — retry с corrective prompt (max RouterMaxRetries).
//      g) После исчерпания retry — возвращает Decision{Done:true, Outcome:needs_human}.
//
// Безопасность:
//   - В стандартный logger пишется ТОЛЬКО метаданные (task_id, step_no, error_type).
//     Сырой LLM-output идёт через redact.SafeRawAttr (хэш+длина, без содержимого).
//   - Полный raw_response сохраняется отдельно в router_decisions.encrypted_raw_response
//     (это делает Orchestrator.Step после Decide, не RouterService).

// Decision — то, что Router решил на этом Step'е.
//
// Семантика:
//   - Done=true + Outcome — задача терминальна, Orchestrator финализирует tasks.state.
//   - Done=false + Agents (>=1) — следующий шаг: запустить перечисленных агентов
//     (>1 = параллельный fan-out).
//   - Done=false + Agents пустой — НЕВАЛИДНО, парсер вернёт ошибку.
type Decision struct {
	Done    bool                         `json:"done"`
	Outcome models.RouterDecisionOutcome `json:"outcome,omitempty"`
	Agents  []AgentRequest               `json:"agents,omitempty"`
	Reason  string                       `json:"reason"`
}

// AgentRequest — один элемент Decision.Agents: какого агента и с каким input вызвать.
type AgentRequest struct {
	Name  string         `json:"agent"` // имя из реестра agents (agents.name)
	Input map[string]any `json:"input,omitempty"`
}

// TargetArtifactID извлекает первый target_artifact_id из Input (если есть).
// Оставлен для совместимости с тестами и старым кодом.
func (a AgentRequest) TargetArtifactID() (uuid.UUID, bool) {
	ids, ok := a.RawTargetArtifactIDs()
	if !ok || len(ids) == 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(ids[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// RawTargetArtifactIDs возвращает сырые строковые значения input.target_artifact_ids
// (или target_artifact_id если это строка) и признак их НАЛИЧИЯ.
func (a AgentRequest) RawTargetArtifactIDs() ([]string, bool) {
	if a.Input == nil {
		return nil, false
	}
	
	// Support both singular and plural forms for backward compatibility
	var rawIDs []string
	
	if rawSingular, ok := a.Input["target_artifact_id"]; ok {
		if s, ok := rawSingular.(string); ok && s != "" {
			rawIDs = append(rawIDs, s)
		}
	}
	
	if rawPlural, ok := a.Input["target_artifact_ids"]; ok {
		if arr, ok := rawPlural.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					rawIDs = append(rawIDs, s)
				}
			}
		}
	}
	
	if len(rawIDs) == 0 {
		return nil, false
	}
	
	return rawIDs, true
}

// RouterState — снимок состояния задачи для одного вызова Router'а.
// Заполняется Orchestrator.Step'ом перед Decide. Все артефакты — БЕЗ content
// (поле Content в Artifact оставляется пустым/обрезанным; Router его не видит).
type RouterState struct {
	Task      *models.Task
	TeamID    uuid.UUID
	Agents    []*models.Agent    // только enabled (is_active=true)
	Artifacts []models.Artifact  // только metadata (без content); только status=ready
	InFlight  []models.TaskEvent // незавершённые agent_job для этой задачи
	DeadJobs  []models.TaskEvent // agent_job, исчерпавшие retry (OOM/timeout/crash) — НЕ переназначать вслепую
	StepNo    int                // текущий step number (для max-steps трекинга в промпте)
	MaxSteps  int                // конфиг max_steps_per_task
	// Repositories — git-репозитории проекта (мульти-репо). Рендерятся в промпт секцией
	// `# Repositories`, чтобы decomposer раскладывал подзадачи по нужному репо (repo_slug).
	Repositories []models.ProjectRepository
	// RecentDecisions — последние Router-решения по ЭТОЙ задаче в хронологическом порядке
	// (ASC по step_no). Даёт Router'у память о том, кого он уже запускал и почему — без неё
	// LLM на каждом шаге видит только title+artifacts и может счесть задачу «только созданной»
	// и заново вызвать того же агента (инцидент SupportAggent). Рендерятся секцией
	// `# Recent routing history`. Пусто на первом шаге.
	RecentDecisions []models.RouterDecision
}

// RouterConfig — настройки сервиса.
type RouterConfig struct {
	RouterAgentName string // имя router-агента в БД (default "router")
	MaxRetries      int    // макс корректирующих повторов на одном Step (default 2)
}

// DefaultRouterConfig возвращает разумные дефолты.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		RouterAgentName: "router",
		MaxRetries:      2,
	}
}

// AgentLoader — мини-интерфейс для загрузки агентов. Введён для тестируемости
// (мокаем без gorm.DB). В реальной DI — поверх *gorm.DB.
type AgentLoader interface {
	GetAgentByName(ctx context.Context, name string) (*models.Agent, error)
	GetAgentByTeamAndName(ctx context.Context, teamID uuid.UUID, name string) (*models.Agent, error)
}

// RouterService — оркестрационный сервис Router'а.
type RouterService struct {
	loader     AgentLoader
	dispatcher AgentDispatcher
	logger     *slog.Logger
	cfg        RouterConfig
}

// NewRouterService — конструктор. logger ОБЯЗАН быть с redact-обёрткой
// (logging.NewHandler), иначе sensitive поля могут утечь.
func NewRouterService(loader AgentLoader, dispatcher AgentDispatcher, logger *slog.Logger, cfg RouterConfig) *RouterService {
	if cfg.RouterAgentName == "" {
		cfg.RouterAgentName = "router"
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &RouterService{
		loader:     loader,
		dispatcher: dispatcher,
		logger:     logger,
		cfg:        cfg,
	}
}

// Decide — главный публичный метод. Возвращает решение Router'а.
//
// Контракт ошибок:
//   - error != nil — сбой инфраструктуры (БД, dispatcher, ctx). НЕ возвращаем "needs_human";
//     caller (Orchestrator.Step) должен пометить task_event как failed и попробовать
//     повторить через retry-механизм очереди.
//   - error == nil — Decision валиден и готов к исполнению, ЛИБО Decision.Done=true
//     с Outcome=needs_human если LLM не смог за все retries дать валидный ответ
//     (это штатное завершение, не ошибка инфраструктуры).
func (r *RouterService) Decide(ctx context.Context, state RouterState) (Decision, error) {
	if state.Task == nil {
		return Decision{}, fmt.Errorf("router: state.Task is required")
	}

	a, err := r.loader.GetAgentByTeamAndName(ctx, state.TeamID, r.cfg.RouterAgentName)
	if err != nil {
		return Decision{}, fmt.Errorf("router: load %q agent: %w", r.cfg.RouterAgentName, err)
	}
	if a == nil {
		return Decision{}, fmt.Errorf("router: agent %q not found", r.cfg.RouterAgentName)
	}
	if !a.IsActive {
		return Decision{}, fmt.Errorf("router: agent %q is disabled (is_active=false)", a.Name)
	}
	if a.ExecutionKind != models.AgentExecutionKindLLM {
		// Router — концептуально llm-агент. Если кто-то в БД поменял execution_kind —
		// это инвариантная ошибка, не молчим.
		return Decision{}, fmt.Errorf("router: agent %q must have execution_kind=llm, got %q", a.Name, a.ExecutionKind)
	}

	executor, err := r.dispatcher.BuildExecutor(ctx, a)
	if err != nil {
		return Decision{}, fmt.Errorf("router: build executor: %w", err)
	}

	userPrompt := r.buildUserPrompt(state, "")

	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		in := agent.ExecutionInput{
			TaskID:       state.Task.ID.String(),
			ProjectID:    state.Task.ProjectID.String(),
			Title:        state.Task.Title,
			Description:  state.Task.Description,
			AgentID:      a.ID.String(),
			AgentName:    a.Name,
			Role:         string(a.Role),
			Model:        derefString(a.Model),
			Provider:     derefProviderKind(a.ProviderKind),
			PromptSystem: derefString(a.SystemPrompt),
			PromptUser:   userPrompt,
			Temperature:  a.Temperature,
			MaxTokens:    a.MaxTokens,
		}

		result, execErr := executor.Execute(ctx, in)
		if execErr != nil {
			return Decision{}, fmt.Errorf("router: executor failed on attempt %d: %w", attempt, execErr)
		}
		if result == nil || !result.Success {
			return Decision{}, fmt.Errorf("router: executor returned unsuccessful result on attempt %d", attempt)
		}

		raw := []byte(result.Output)
		decision, correction := parseAndValidateDecision(raw, state.Agents, state.Artifacts, len(state.InFlight) > 0)
		if correction == nil {
			r.logger.DebugContext(ctx, "router decision accepted",
				"task_id", state.Task.ID,
				"step_no", state.StepNo,
				"attempt", attempt,
				"done", decision.Done,
				"agents_count", len(decision.Agents),
			)
			return decision, nil
		}

		// Парсинг или валидация не прошли — следующая попытка с corrective hint'ом.
		// В лог идёт ТОЛЬКО статический Code (например, "json_parse_error") — этого
		// достаточно для аналитики. correction.PromptText, который может содержать
		// фрагмент сломанного JSON через err.Error(), отправляется ИСКЛЮЧИТЕЛЬНО в
		// промпт следующей попытки LLM, но не в логи. raw — через SafeRawAttr (хэш+длина).
		r.logger.WarnContext(ctx, "router decision invalid, retrying",
			"task_id", state.Task.ID,
			"step_no", state.StepNo,
			"attempt", attempt,
			"correction_code", correction.LogCode,
			logging.SafeRawAttr(raw),
		)

		userPrompt = r.buildUserPrompt(state, correction.PromptText)
	}

	// Исчерпан retry-бюджет.
	r.logger.ErrorContext(ctx, "router exceeded retry budget, falling back to needs_human",
		"task_id", state.Task.ID,
		"step_no", state.StepNo,
		"max_retries", r.cfg.MaxRetries,
	)
	return Decision{
		Done:    true,
		Outcome: models.RouterDecisionOutcomeNeedsHuman,
		Reason:  fmt.Sprintf("router exhausted retry budget (max=%d) without valid Decision", r.cfg.MaxRetries),
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Парсинг + валидация
// ─────────────────────────────────────────────────────────────────────────────

// markdownCodeFenceRe ловит ПЕРВЫЙ блок ```json\n{...}\n``` или ```\n{...}\n```
// в произвольном месте ответа. Якорей ^/$ нет специально — LLM может добавить
// сопроводительный текст до или после блока ("Here is the plan: ... Good luck!"),
// и мы не должны из-за этого терять одну попытку retry.
// Lazy-квантификатор (.*?) гарантирует что мы возьмём содержимое ПЕРВОГО fence-блока.
var markdownCodeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

// stripMarkdownFences — снимает обёртку ```json ... ``` если LLM её добавил.
// Возвращает inner content из первого найденного fence-блока или исходную строку
// (после TrimSpace) если fences не найдены.
func stripMarkdownFences(raw []byte) []byte {
	if m := markdownCodeFenceRe.FindSubmatch(raw); m != nil {
		return bytes.TrimSpace(m[1])
	}
	return bytes.TrimSpace(raw)
}

// correctionHint — результат неуспешного parse/validate, который оркестратор отправляет
// LLM-у на следующей попытке. Разделение полей — security-инвариант:
//
//   - LogCode — короткий категориальный идентификатор, БЕЗОПАСНЫЙ для логов. Не содержит
//     фрагментов LLM-ответа, имён неизвестных агентов, sensitive данных. Используется
//     для аналитики ("как часто Router галлюцинирует?") и trace-метрик.
//   - PromptText — полное человекочитаемое сообщение, может включать `err.Error()`
//     (а тот, в свою очередь, может содержать фрагмент невалидного JSON, включая
//     случайно сгенерированные секреты). Этот текст идёт ТОЛЬКО в следующий промпт LLM,
//     никогда в стандартный logger / file-логи / Sentry.
type correctionHint struct {
	LogCode    string
	PromptText string
}

// Коды коррекции — список фиксирован, безопасно логировать в открытом виде.
const (
	correctionCodeEmptyResponse     = "empty_response"
	correctionCodeJSONParseError    = "json_parse_error"
	correctionCodeMissingReason     = "missing_reason"
	correctionCodeInvalidOutcome    = "invalid_outcome"
	correctionCodeDoneWithAgents    = "done_with_agents"
	correctionCodeEmptyAgents       = "empty_agents"
	correctionCodeAgentNameEmpty    = "agent_name_empty"
	correctionCodeUnknownAgent      = "unknown_agent"
	correctionCodeDuplicateArtifact = "duplicate_artifact_id"
	correctionCodeUnknownArtifact   = "unknown_artifact"
	correctionCodeInvalidArtifactID = "invalid_artifact_id"
)

// parseAndValidateDecision выполняет: strip fences → unmarshal → validate.
// Возвращает (Decision, nil) при успехе или (zero, *correctionHint) при ошибке.
func parseAndValidateDecision(raw []byte, enabledAgents []*models.Agent, existingArtifacts []models.Artifact, hasInFlight bool) (Decision, *correctionHint) {
	stripped := stripMarkdownFences(raw)
	if len(stripped) == 0 {
		return Decision{}, &correctionHint{
			LogCode:    correctionCodeEmptyResponse,
			PromptText: "previous response was empty; respond with raw JSON object only.",
		}
	}

	var d Decision
	if err := json.Unmarshal(stripped, &d); err != nil {
		// ВАЖНО: err.Error() от json.Unmarshal может содержать фрагмент сломанного
		// JSON (например, "invalid character 'x' looking for ... near 'leak_canary_payload'").
		// Поэтому полная ошибка идёт ТОЛЬКО в PromptText (LLM → encrypted_raw_response),
		// а LogCode — статичный.
		return Decision{}, &correctionHint{
			LogCode:    correctionCodeJSONParseError,
			PromptText: fmt.Sprintf("previous response was not valid JSON: %s — respond with raw JSON object only, no markdown fences, no prose.", err.Error()),
		}
	}

	if d.Reason == "" {
		return Decision{}, &correctionHint{
			LogCode:    correctionCodeMissingReason,
			PromptText: `"reason" field is required (non-empty short explanation of your decision).`,
		}
	}

	if d.Done {
		if !d.Outcome.IsValid() {
			return Decision{}, &correctionHint{
				LogCode:    correctionCodeInvalidOutcome,
				PromptText: fmt.Sprintf(`when done=true, "outcome" must be one of: %s.`, strings.Join(outcomesAsStrings(), ", ")),
			}
		}
		if len(d.Agents) != 0 {
			return Decision{}, &correctionHint{
				LogCode:    correctionCodeDoneWithAgents,
				PromptText: `when done=true, "agents" must be empty.`,
			}
		}
		return d, nil
	}

	// done=false — должен быть хотя бы один агент, если только нет активных in-flight задач (в этом случае допустимо ожидание).
	if len(d.Agents) == 0 && !hasInFlight {
		return Decision{}, &correctionHint{
			LogCode:    correctionCodeEmptyAgents,
			PromptText: `when done=false, "agents" must contain at least one agent.`,
		}
	}

	// Валидация имён агентов: каждый должен быть в реестре enabled.
	enabledSet := make(map[string]struct{}, len(enabledAgents))
	for _, a := range enabledAgents {
		enabledSet[a.Name] = struct{}{}
	}
	enabledNames := agentNamesList(enabledAgents)
	for i, req := range d.Agents {
		if req.Name == "" {
			return Decision{}, &correctionHint{
				LogCode:    correctionCodeAgentNameEmpty,
				PromptText: fmt.Sprintf(`agents[%d].agent is empty; choose one from: %s.`, i, enabledNames),
			}
		}
		if _, ok := enabledSet[req.Name]; !ok {
			// req.Name — потенциально галлюцинированное LLM имя. В лог его не пишем
			// (через correctionHint.LogCode), а в PromptText включаем — LLM должен
			// видеть свою же ошибку чтобы исправить.
			return Decision{}, &correctionHint{
				LogCode:    correctionCodeUnknownAgent,
				PromptText: fmt.Sprintf(`agents[%d].agent=%q not found in registry; choose one from: %s.`, i, req.Name, enabledNames),
			}
		}
	}

	// Защита от дублей target_artifact_id в одном Decision (Router не должен запускать
	// двух воркеров на один артефакт параллельно) и валидация существования артефакта.
	seen := make(map[uuid.UUID]int)
	artifactsMap := make(map[uuid.UUID]models.Artifact, len(existingArtifacts))
	for _, art := range existingArtifacts {
		artifactsMap[art.ID] = art
	}

	for i, req := range d.Agents {
		rawIDs, hasTarget := req.RawTargetArtifactIDs()
		if !hasTarget {
			continue
		}
		
		for _, raw := range rawIDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				var validIDs []string
				for _, art := range existingArtifacts {
					validIDs = append(validIDs, fmt.Sprintf("%s (%s)", art.ID, art.Kind))
				}
				validList := strings.Join(validIDs, ", ")
				if validList == "" {
					validList = "(no artifacts exist)"
				}
				return Decision{}, &correctionHint{
					LogCode:    correctionCodeInvalidArtifactID,
					PromptText: fmt.Sprintf(`agents[%d] target_artifact_id=%q is not a valid full UUID. Copy the EXACT full id from the Artifacts list (e.g. "550e8400-e29b-41d4-a716-446655440000"), do not abbreviate. Available: %s.`, i, raw, validList),
				}
			}
			if _, exists := artifactsMap[id]; !exists {
				var validIDs []string
				for _, art := range existingArtifacts {
					validIDs = append(validIDs, fmt.Sprintf("%s (%s)", art.ID, art.Kind))
				}
				validList := strings.Join(validIDs, ", ")
				if validList == "" {
					validList = "(no artifacts exist)"
				}
				return Decision{}, &correctionHint{
					LogCode:    correctionCodeUnknownArtifact,
					PromptText: fmt.Sprintf(`agents[%d] targets artifact %s which does not exist in this task. Choose from the available artifacts: %s.`, i, id, validList),
				}
			}
			if prev, dup := seen[id]; dup {
				return Decision{}, &correctionHint{
					LogCode:    correctionCodeDuplicateArtifact,
					PromptText: fmt.Sprintf(`agents[%d] and agents[%d] both target the same artifact %s; one job per artifact per step.`, prev, i, id),
				}
			}
			seen[id] = i
		}
	}

	return d, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Промпт-билдер
// ─────────────────────────────────────────────────────────────────────────────

// buildUserPrompt строит user-prompt из RouterState. correction — необязательное
// сообщение об ошибке предыдущей попытки, дописывается в конец промпта чтобы
// LLM учёл его при повторе.
//
// ВАЖНО: НЕ включаем artifact.Content (это огромные дифы / тексты планов). Только
// summary, kind, producer, iteration, status. Бюджет контекста соблюдается.
func (r *RouterService) buildUserPrompt(state RouterState, correction string) string {
	var b strings.Builder

	b.WriteString("# Task\n")
	b.WriteString(state.Task.Title)
	b.WriteString("\n\n")
	if state.Task.Description != "" {
		b.WriteString(state.Task.Description)
		b.WriteString("\n\n")
	}

	// Мульти-репо: показываем каталог репозиториев проекта, только если их больше одного.
	// Для одно-репо проекта секция избыточна (всё пишется в единственный репо).
	if len(state.Repositories) > 1 {
		b.WriteString("# Repositories\n")
		b.WriteString("Проект состоит из нескольких git-репозиториев. Каждая подзадача относится РОВНО к одному репо (см. repo_slug). Кросс-репо фичу декомпозируй на подзадачи с depends_on.\n")
		for _, repo := range state.Repositories {
			marker := ""
			if repo.IsPrimary {
				marker = " (primary)"
			}
			desc := strings.TrimSpace(repo.RoleDescription)
			if desc == "" {
				desc = repo.DisplayName
			}
			fmt.Fprintf(&b, "- slug=%s%s: %s\n", repo.Slug, marker, desc)
		}
		b.WriteString("\n")
	}

	b.WriteString("# Available Agents\n")
	for _, a := range state.Agents {
		desc := derefString(a.RoleDescription)
		// Defence-in-depth: даже если loadRouterState отдал агента без описания
		// (например, через тестовый stub), не пихаем его в каталог — LLM не может
		// с ним работать. См. cost-leak инцидент Phase 2: 289 пустых записей
		// раздувают prompt до 7k+ токенов на каждый Router-вызов.
		if desc == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s (kind=%s): %s\n",
			a.Name,
			a.ExecutionKind,
			desc,
		)
	}
	b.WriteString("\n")

	if len(state.Artifacts) == 0 {
		b.WriteString("# Artifacts\n(no artifacts yet — task just started)\n\n")
	} else {
		b.WriteString("# Artifacts (metadata only, no content)\n")
		for i, art := range state.Artifacts {
			parent := "-"
			if art.ParentID != nil {
				parent = art.ParentID.String()
			}
			fmt.Fprintf(&b, "%d. id=%s kind=%s producer=%s iter=%d status=%s parent=%s\n   summary: %s\n",
				i+1, art.ID, art.Kind, art.ProducerAgent, art.Iteration, art.Status, parent, art.Summary,
			)
		}
		b.WriteString("\n")
	}

	// Память Router'а о собственных прошлых решениях по этой задаче. Без неё LLM видит только
	// title+artifacts и склонен счесть задачу «только созданной», заново вызвав того же агента
	// (инцидент SupportAggent). Показываем, кого уже запускали — артефакты этих агентов выше.
	if len(state.RecentDecisions) > 0 {
		b.WriteString("# Recent routing history (YOUR previous decisions on this task)\n")
		for _, d := range state.RecentDecisions {
			agentsStr := "(waiting — no agents dispatched)"
			if len(d.ChosenAgents) > 0 {
				agentsStr = "dispatched [" + strings.Join([]string(d.ChosenAgents), ", ") + "]"
			}
			outcome := ""
			if d.Outcome != nil {
				outcome = fmt.Sprintf(" outcome=%s", *d.Outcome)
			}
			reason := strings.TrimSpace(d.Reason)
			if reason != "" {
				reason = " — " + reason
			}
			fmt.Fprintf(&b, "- step %d: %s%s%s\n", d.StepNo, agentsStr, outcome, reason)
		}
		b.WriteString("These agents ALREADY RAN; their results are in the Artifacts section above. " +
			"Do NOT re-dispatch an agent that has already produced a ready artifact unless you have a NEW, " +
			"concrete instruction that will change its output. If the deliverable already exists, finish the " +
			"task (DONE outcome=done), or escalate (outcome=needs_human) if you are blocked. Repeating the same " +
			"dispatch is a loop and will be force-stopped.\n\n")
	}

	if len(state.InFlight) > 0 {
		b.WriteString("# In-flight jobs (NOT yet completed — DO NOT duplicate)\n")
		now := time.Now()
		for _, ev := range state.InFlight {
			startedAt := "scheduled"
			if ev.LockedAt != nil {
				startedAt = fmt.Sprintf("running for %s", now.Sub(*ev.LockedAt).Round(time.Second))
			}
			fmt.Fprintf(&b, "- task_event #%d kind=%s attempts=%d/%d %s\n   payload: %s\n",
				ev.ID, ev.Kind, ev.Attempts, ev.MaxAttempts, startedAt, string(ev.Payload),
			)
		}
		b.WriteString("\n")
	}

	if len(state.DeadJobs) > 0 {
		b.WriteString("# Failed jobs (exhausted all retries — DO NOT blindly re-dispatch)\n")
		b.WriteString("These agent jobs failed repeatedly (likely sandbox OOM / timeout / crash, not a logic problem you can fix by retrying the same agent on the same target):\n")
		for _, ev := range state.DeadJobs {
			fmt.Fprintf(&b, "- task_event #%d attempts=%d/%d payload: %s\n",
				ev.ID, ev.Attempts, ev.MaxAttempts, string(ev.Payload))
			if ev.LastError != nil && *ev.LastError != "" {
				fmt.Fprintf(&b, "   last_error: %s\n", *ev.LastError)
			}
		}
		b.WriteString("If the same subtask keeps failing this way, prefer DONE with outcome=needs_human (a human must inspect the sandbox/environment) over re-dispatching the same agent.\n\n")
	}

	if state.MaxSteps > 0 {
		fmt.Fprintf(&b, "# Progress\nStep %d of %d (hard cap). If you've been looping without progress, consider DONE outcome=failed.\n\n",
			state.StepNo, state.MaxSteps,
		)
	}

	b.WriteString(`# Guidelines
- Right-size the process. For a small, single-layer task (a skeleton, one file, one fix) go straight to a developer, then a reviewer. Only run planner/decomposer when the task genuinely spans several independent sub-deliverables.
- Decomposition auto-expands: once a 'decomposition' artifact is approved it is split into 'subtask_description' artifacts automatically — you do NOT need to re-run the decomposer for that.
- Gate the merger: do NOT dispatch the merger while ANY in-flight job is running or while ANY subtask still lacks an approved code_diff. Merge only once every subtask is approved.
- Review of a fix is scoped: when a reviewer (or developer) targets an artifact that already has a parent review with changes_requested, judge ONLY whether the previous review's points are resolved. Do not expand the scope and invent new unrelated requirements round after round.
- Don't loop. Check "Recent routing history": if an agent you already dispatched has produced a ready artifact and you have no new concrete instruction that would change the result, do NOT dispatch it again — finish the task (DONE outcome=done if the deliverable exists) or escalate (outcome=needs_human) if you are stuck. A task is not "just created" if it already has artifacts or routing history.
- Don't fight the environment. If a job is in "Failed jobs" above, retrying the identical agent on the identical target will fail the same way — escalate to needs_human instead.

`)

	b.WriteString(`# Response Format
Respond with ONE valid JSON object, no markdown fences, no prose around it:
{
  "done": false,
  "outcome": null,
  "agents": [{"agent": "<name>", "input": {"target_artifact_id": "<uuid or omit>", "target_artifact_ids": ["<uuid1>", "<uuid2>"], "instructions": "..."}}],
  "reason": "1-2 sentences explaining your choice"
}
Note: For merger, use target_artifact_ids array to pass ALL approved code_diff artifacts you want to merge at once.
If task is complete/failed/needs-human: set done=true, outcome accordingly, agents=[].
`)

	if correction != "" {
		b.WriteString("\n# Correction (your previous response had a problem)\n")
		b.WriteString(correction)
		b.WriteString("\nPlease respond again, fixing this issue.\n")
	}

	return b.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefProviderKind(p *models.AgentProviderKind) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

// outcomesAsStrings — обёртка над models.AllRouterDecisionOutcomes для подстановки
// в текстовые сообщения. Источник правды — models, чтобы при добавлении нового
// outcome не было дублирующего списка в этом файле (DRY).
func outcomesAsStrings() []string {
	all := models.AllRouterDecisionOutcomes()
	out := make([]string, len(all))
	for i, o := range all {
		out[i] = string(o)
	}
	return out
}

func agentNamesList(agents []*models.Agent) string {
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name)
	}
	return "[" + strings.Join(names, ", ") + "]"
}
