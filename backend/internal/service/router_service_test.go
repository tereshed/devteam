package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test doubles
// ─────────────────────────────────────────────────────────────────────────────

// mockExecutor возвращает заранее запрограммированные строки в порядке вызова.
// Это позволяет тестировать retry-цикл: первая попытка вернёт сломанный JSON,
// вторая — валидный.
type mockExecutor struct {
	responses []string
	calls     int
	// onCall можно использовать для проверки промпта (включая corrective hint'ы).
	onCall func(in agent.ExecutionInput)
}

func (m *mockExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	if m.onCall != nil {
		m.onCall(in)
	}
	if m.calls >= len(m.responses) {
		return nil, errors.New("mockExecutor: no more responses programmed")
	}
	out := m.responses[m.calls]
	m.calls++
	return &agent.ExecutionResult{Success: true, Output: out}, nil
}

// mockDispatcher всегда возвращает заданного executor'а.
type mockDispatcher struct {
	executor agent.AgentExecutor
	err      error
}

func (m *mockDispatcher) BuildExecutor(ctx context.Context, a *models.Agent) (agent.AgentExecutor, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.executor, nil
}

// mockLoader возвращает router-agent. err имеет приоритет над agent.
type mockLoader struct {
	agent *models.Agent
	err   error
}

func (m *mockLoader) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.agent, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// makeRouterAgent — minimal router-agent record для тестов.
func makeRouterAgent() *models.Agent {
	model := "claude-sonnet-4-6"
	return &models.Agent{
		ID:            uuid.New(),
		Name:          "router",
		Role:          models.AgentRoleRouter,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
		IsActive:      true,
	}
}

func makeAgent(name string, kind models.AgentExecutionKind) *models.Agent {
	a := &models.Agent{ID: uuid.New(), Name: name, ExecutionKind: kind, IsActive: true}
	if kind == models.AgentExecutionKindLLM {
		m := "claude-sonnet-4-6"
		a.Model = &m
	} else {
		cb := models.CodeBackendClaudeCode
		a.CodeBackend = &cb
	}
	return a
}

func makeTask() *models.Task {
	return &models.Task{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		Title:       "Add JWT auth",
		Description: "Implement JWT authentication for the API.",
		State:       models.TaskStateActive,
	}
}

// newTestRouter создаёт RouterService с мок-зависимостями. Возвращает также
// указатель на executor чтобы тест мог проверять количество вызовов / промпты.
func newTestRouter(t *testing.T, responses []string, agents []*models.Agent) (*RouterService, *mockExecutor) {
	t.Helper()
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}

	// Используем redact-обёрнутый logger, чтобы случайно протекших raw-данных не было
	// в выводе теста (тестируем поведение, не логи).
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))

	// Реестр доступных агентов — router + те, что пришли в тест.
	registry := append([]*models.Agent{makeRouterAgent()}, agents...)
	_ = registry // используется через RouterState

	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())
	return svc, exec
}

func makeState(agents []*models.Agent, artifacts []models.Artifact) RouterState {
	return RouterState{
		Task:      makeTask(),
		Agents:    agents,
		Artifacts: artifacts,
		StepNo:    0,
		MaxSteps:  100,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Happy paths
// ─────────────────────────────────────────────────────────────────────────────

// TestDecide_Sequential_PlannerNext — пустая задача, Router выбирает Planner.
func TestDecide_Sequential_PlannerNext(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		`{"done": false, "agents": [{"agent": "planner", "input": {"instructions": "Build a plan"}}], "reason": "task just started, need a plan"}`,
	}
	svc, exec := newTestRouter(t, responses, []*models.Agent{planner})

	state := makeState([]*models.Agent{planner}, nil)
	d, err := svc.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Done {
		t.Fatalf("expected done=false, got Decision=%+v", d)
	}
	if len(d.Agents) != 1 || d.Agents[0].Name != "planner" {
		t.Fatalf("expected single planner, got %+v", d.Agents)
	}
	if exec.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", exec.calls)
	}
}

// TestDecide_Parallel_TwoDevelopers — Router возвращает массив из 2 developer'ов
// для независимых подзадач (DAG с depends_on=[]).
func TestDecide_Parallel_TwoDevelopers(t *testing.T) {
	dev := makeAgent("developer", models.AgentExecutionKindSandbox)
	artA, artB := uuid.New(), uuid.New()
	responses := []string{
		`{
			"done": false,
			"agents": [
				{"agent": "developer", "input": {"target_artifact_id": "` + artA.String() + `", "instructions": "subtask A"}},
				{"agent": "developer", "input": {"target_artifact_id": "` + artB.String() + `", "instructions": "subtask B"}}
			],
			"reason": "subtasks A and B have empty depends_on, running in parallel"
		}`,
	}
	svc, _ := newTestRouter(t, responses, []*models.Agent{dev})

	state := makeState([]*models.Agent{dev}, nil)
	d, err := svc.Decide(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Agents) != 2 {
		t.Fatalf("expected 2 parallel agents, got %d: %+v", len(d.Agents), d.Agents)
	}
	got0, ok0 := d.Agents[0].TargetArtifactID()
	got1, ok1 := d.Agents[1].TargetArtifactID()
	if !ok0 || !ok1 || got0 == got1 {
		t.Errorf("expected two distinct target_artifact_id, got %v / %v", got0, got1)
	}
}

// TestDecide_Done_Completed — Router считает что задача выполнена.
func TestDecide_Done_Completed(t *testing.T) {
	responses := []string{
		`{"done": true, "outcome": "done", "agents": [], "reason": "all subtasks passed tests"}`,
	}
	svc, _ := newTestRouter(t, responses, nil)

	d, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeDone {
		t.Fatalf("expected done=true outcome=done, got %+v", d)
	}
}

// TestDecide_Done_Failed — Router сдался и пометил задачу failed.
// Это покрывает фикстуру "blocked" из плана (п.8 Sprint 2): Router не может
// продвинуться (например, артефакт ревьюится 6+ раз) и завершает терминально.
func TestDecide_Done_Failed(t *testing.T) {
	responses := []string{
		`{"done": true, "outcome": "failed", "agents": [], "reason": "plan rejected 6 times by reviewer — blocked"}`,
	}
	svc, _ := newTestRouter(t, responses, nil)

	d, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeFailed {
		t.Fatalf("expected done=true outcome=failed, got %+v", d)
	}
}

// TestDecide_Done_Cancelled — Router распознал что пользователь отменил задачу
// (cancel_requested=true), и финализирует её как cancelled.
// В реальности cancel-check делает Orchestrator.Step до вызова Router, но если
// flag всё же прокинут и Router увидел его в state — допустим валидный outcome.
func TestDecide_Done_Cancelled(t *testing.T) {
	responses := []string{
		`{"done": true, "outcome": "cancelled", "agents": [], "reason": "user requested cancellation"}`,
	}
	svc, _ := newTestRouter(t, responses, nil)

	d, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeCancelled {
		t.Fatalf("expected done=true outcome=cancelled, got %+v", d)
	}
}

// TestDecide_Done_NeedsHuman — Router решил эскалировать на оператора.
// Часто бывает при detected loop ("artifact reviewed >5 times") или при
// неоднозначных требованиях.
func TestDecide_Done_NeedsHuman(t *testing.T) {
	responses := []string{
		`{"done": true, "outcome": "needs_human", "agents": [], "reason": "task requirements ambiguous, escalating to operator"}`,
	}
	svc, _ := newTestRouter(t, responses, nil)

	d, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeNeedsHuman {
		t.Fatalf("expected done=true outcome=needs_human, got %+v", d)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Hallucination recovery — главный security-кейс плана
// ─────────────────────────────────────────────────────────────────────────────

// TestDecide_RecoversFromMarkdownFences — LLM обернул ответ в ```json ... ```.
func TestDecide_RecoversFromMarkdownFences(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		"```json\n" + `{"done": false, "agents": [{"agent": "planner"}], "reason": "fenced response"}` + "\n```",
	}
	svc, _ := newTestRouter(t, responses, []*models.Agent{planner})

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("expected fences to be stripped, got error: %v", err)
	}
	if d.Done || len(d.Agents) != 1 || d.Agents[0].Name != "planner" {
		t.Fatalf("expected parsed planner decision, got %+v", d)
	}
}

// TestDecide_RecoversFromProseWrappedFences — LLM добавил болтливый префикс/суффикс
// вокруг fence-блока. Это типичное поведение Sonnet/Opus при недостаточно жёстком
// system prompt'е. Регулярка БЕЗ якорей ^/$ должна вытащить блок и не сжечь retry.
func TestDecide_RecoversFromProseWrappedFences(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		"Here is my decision for this task:\n\n```json\n" +
			`{"done": false, "agents": [{"agent": "planner"}], "reason": "wrapped in prose"}` +
			"\n```\n\nLet me know if you need adjustments!",
	}
	svc, exec := newTestRouter(t, responses, []*models.Agent{planner})

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("expected fences to be stripped from prose, got error: %v", err)
	}
	if d.Done || len(d.Agents) != 1 || d.Agents[0].Name != "planner" {
		t.Fatalf("expected planner decision, got %+v", d)
	}
	if exec.calls != 1 {
		t.Errorf("prose-wrapped fences should be parsed on first try (no retry), got %d calls", exec.calls)
	}
}

// TestDecide_RetriesOnInvalidJSON — первая попытка ломаный JSON, вторая валидна.
// Проверяем что промпт второй попытки СОДЕРЖИТ correction-hint.
func TestDecide_RetriesOnInvalidJSON(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		`this is not JSON at all`,
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "recovered after retry"}`,
	}

	var prompts []string
	exec := &mockExecutor{
		responses: responses,
		onCall:    func(in agent.ExecutionInput) { prompts = append(prompts, in.PromptUser) },
	}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("expected recovery, got error: %v", err)
	}
	if d.Done || len(d.Agents) != 1 {
		t.Fatalf("expected planner decision after retry, got %+v", d)
	}
	if exec.calls != 2 {
		t.Errorf("expected 2 LLM calls (retry), got %d", exec.calls)
	}
	if len(prompts) < 2 {
		t.Fatal("expected to capture at least 2 prompts")
	}
	if !strings.Contains(prompts[1], "Correction") {
		t.Errorf("second prompt must contain Correction section, got: %s", prompts[1])
	}
	if !strings.Contains(prompts[1], "not valid JSON") {
		t.Errorf("second prompt must explain the JSON parse error, got: %s", prompts[1])
	}
}

// TestDecide_RetriesOnUnknownAgent — Router придумал агента, которого нет.
func TestDecide_RetriesOnUnknownAgent(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		`{"done": false, "agents": [{"agent": "super_hacker"}], "reason": "hallucinated agent"}`,
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "corrected"}`,
	}
	var prompts []string
	exec := &mockExecutor{responses: responses, onCall: func(in agent.ExecutionInput) { prompts = append(prompts, in.PromptUser) }}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Agents[0].Name != "planner" {
		t.Fatalf("expected corrected planner, got %+v", d.Agents)
	}
	if !strings.Contains(prompts[1], "super_hacker") {
		t.Errorf("correction must reference the bad agent name, got: %s", prompts[1])
	}
	if !strings.Contains(prompts[1], "planner") {
		t.Errorf("correction must list valid choices, got: %s", prompts[1])
	}
}

// TestDecide_RetriesOnDuplicateArtifactID — параллельный fan-out на ОДИН артефакт запрещён.
func TestDecide_RetriesOnDuplicateArtifactID(t *testing.T) {
	dev := makeAgent("developer", models.AgentExecutionKindSandbox)
	dup := uuid.New().String()
	responses := []string{
		`{"done": false, "agents": [
			{"agent": "developer", "input": {"target_artifact_id": "` + dup + `"}},
			{"agent": "developer", "input": {"target_artifact_id": "` + dup + `"}}
		], "reason": "duplicate target"}`,
		`{"done": false, "agents": [{"agent": "developer", "input": {"target_artifact_id": "` + dup + `"}}], "reason": "fixed"}`,
	}
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{dev}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Agents) != 1 {
		t.Fatalf("expected single agent after correction, got %d", len(d.Agents))
	}
}

// TestDecide_RetriesOnEmptyAgents — done=false с пустым массивом agents — невалидно.
func TestDecide_RetriesOnEmptyAgents(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		`{"done": false, "agents": [], "reason": "forgot to pick anyone"}`,
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "fixed"}`,
	}
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Agents) != 1 || d.Agents[0].Name != "planner" {
		t.Fatalf("expected recovery to single planner, got %+v", d.Agents)
	}
}

// TestDecide_ExhaustedRetries_FallsBackToNeedsHuman — LLM так и не дал валидный JSON.
// Router должен ВЕРНУТЬ Decision{Done:true, Outcome:needs_human}, а НЕ error.
func TestDecide_ExhaustedRetries_FallsBackToNeedsHuman(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	responses := []string{
		`bad json 1`,
		`still bad`,
		`also bad`,
	}
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig()) // MaxRetries=2 → 3 attempts total

	d, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("exhausted retries must NOT return error; got: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeNeedsHuman {
		t.Fatalf("expected done=true outcome=needs_human, got %+v", d)
	}
	if exec.calls != 3 {
		t.Errorf("expected 3 attempts (initial + 2 retries), got %d", exec.calls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant errors — НЕ молчим, возвращаем error
// ─────────────────────────────────────────────────────────────────────────────

// TestDecide_FailsIfRouterAgentMissing — нет router-агента в БД.
func TestDecide_FailsIfRouterAgentMissing(t *testing.T) {
	disp := &mockDispatcher{executor: &mockExecutor{}}
	loader := &mockLoader{err: errors.New("not found")}
	svc := NewRouterService(loader, disp, slog.New(slog.NewTextHandler(io.Discard, nil)), DefaultRouterConfig())

	_, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err == nil {
		t.Fatal("expected error when router agent is missing")
	}
}

// TestDecide_FailsIfRouterAgentInactive — router выключен — это инвариантная ошибка.
func TestDecide_FailsIfRouterAgentInactive(t *testing.T) {
	a := makeRouterAgent()
	a.IsActive = false
	loader := &mockLoader{agent: a}
	disp := &mockDispatcher{executor: &mockExecutor{}}
	svc := NewRouterService(loader, disp, slog.New(slog.NewTextHandler(io.Discard, nil)), DefaultRouterConfig())

	_, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err == nil {
		t.Fatal("expected error when router agent is disabled")
	}
}

// TestDecide_FailsIfRouterAgentNotLLM — execution_kind подделан в БД.
func TestDecide_FailsIfRouterAgentNotLLM(t *testing.T) {
	a := makeRouterAgent()
	a.ExecutionKind = models.AgentExecutionKindSandbox
	cb := models.CodeBackendClaudeCode
	a.CodeBackend = &cb
	a.Model = nil
	loader := &mockLoader{agent: a}
	disp := &mockDispatcher{executor: &mockExecutor{}}
	svc := NewRouterService(loader, disp, slog.New(slog.NewTextHandler(io.Discard, nil)), DefaultRouterConfig())

	_, err := svc.Decide(context.Background(), makeState(nil, nil))
	if err == nil {
		t.Fatal("expected error when router agent has execution_kind != llm")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Раздельные unit-тесты на парсер (без RouterService)
// ─────────────────────────────────────────────────────────────────────────────

func TestParseAndValidate_RejectsDoneWithAgents(t *testing.T) {
	enabled := []*models.Agent{makeAgent("planner", models.AgentExecutionKindLLM)}
	raw := []byte(`{"done": true, "outcome": "done", "agents": [{"agent":"planner"}], "reason": "x"}`)
	_, correction := parseAndValidateDecision(raw, enabled)
	if correction == nil {
		t.Fatal("expected validation error for done=true with non-empty agents")
	}
	if correction.LogCode != correctionCodeDoneWithAgents {
		t.Errorf("expected LogCode=%q, got %q", correctionCodeDoneWithAgents, correction.LogCode)
	}
	if !strings.Contains(correction.PromptText, "agents") {
		t.Errorf("correction PromptText must mention agents field, got: %s", correction.PromptText)
	}
}

func TestParseAndValidate_RejectsInvalidOutcome(t *testing.T) {
	raw := []byte(`{"done": true, "outcome": "completed", "agents": [], "reason": "x"}`)
	_, correction := parseAndValidateDecision(raw, nil)
	if correction == nil {
		t.Fatal("expected validation error for invalid outcome value")
	}
	if correction.LogCode != correctionCodeInvalidOutcome {
		t.Errorf("expected LogCode=%q, got %q", correctionCodeInvalidOutcome, correction.LogCode)
	}
}

func TestParseAndValidate_RejectsMissingReason(t *testing.T) {
	enabled := []*models.Agent{makeAgent("planner", models.AgentExecutionKindLLM)}
	raw := []byte(`{"done": false, "agents": [{"agent": "planner"}], "reason": ""}`)
	_, correction := parseAndValidateDecision(raw, enabled)
	if correction == nil {
		t.Fatal("expected validation error for empty reason")
	}
	if correction.LogCode != correctionCodeMissingReason {
		t.Errorf("expected LogCode=%q, got %q", correctionCodeMissingReason, correction.LogCode)
	}
}

// TestOutcomesAsStrings_MatchesModelsSource — DRY-guard: outcomesAsStrings источник —
// models.AllRouterDecisionOutcomes. Если в models добавится новое значение, здесь оно
// автоматически появится. Тест зафиксирует контракт.
func TestOutcomesAsStrings_MatchesModelsSource(t *testing.T) {
	got := outcomesAsStrings()
	want := make([]string, 0, len(models.AllRouterDecisionOutcomes()))
	for _, o := range models.AllRouterDecisionOutcomes() {
		want = append(want, string(o))
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("outcomesAsStrings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStripMarkdownFences(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain JSON", "plain JSON"},
		{"```json\n{}\n```", "{}"},
		{"```\n{}\n```", "{}"},
		{"   ```json\n{\"a\":1}\n```   ", `{"a":1}`},
	}
	for _, c := range cases {
		got := string(stripMarkdownFences([]byte(c.in)))
		if got != c.want {
			t.Errorf("stripMarkdownFences(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestDecide_DoesNotLeakRawToLogs — главный security-guard плана.
// Прокидываем canary LEAK_CANARY_PAYLOAD в LLM-ответе с поломанным JSON, проверяем
// что в stderr/буфере логов он НЕ появляется (только хэш+длина через SafeRawAttr).
func TestDecide_DoesNotLeakRawToLogs(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	canary := "LEAK_CANARY_PAYLOAD_orchestrator_v2"
	responses := []string{
		// Невалидный JSON с canary — он должен попасть в WarnContext через SafeRawAttr,
		// что МАСКИРУЕТ его содержимое.
		`not json: ` + canary,
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "recovered"}`,
	}
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	_, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), canary) {
		t.Fatalf("CANARY LEAKED into logs: %s", buf.String())
	}
}

// TestDecide_DoesNotLeakErrErrorToLogs — узкий security-guard для замечания ревью #3.
// json.Unmarshal может включать фрагмент сломанного JSON в err.Error()
// ("invalid character 'L' looking for ... 'LEAK_CANARY_'"). Этот текст идёт в
// correctionHint.PromptText (→ следующий промпт LLM), но НЕ в лог: в лог пишется
// только correctionHint.LogCode (статичная категория, например "json_parse_error").
func TestDecide_DoesNotLeakErrErrorToLogs(t *testing.T) {
	planner := makeAgent("planner", models.AgentExecutionKindLLM)
	// Канарейка ВНУТРИ невалидного JSON. json.Unmarshal на этой строке вернёт
	// ошибку, текст которой содержит цитату из исходной строки.
	canary := "ERR_CANARY_payload_visible_in_error_message"
	responses := []string{
		// Невалидный JSON: открывающая фигурная скобка + текст без кавычек.
		// json.Unmarshal обычно цитирует точку остановки парсинга в ошибке.
		`{` + canary + `}`,
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "recovered"}`,
	}
	exec := &mockExecutor{responses: responses}
	disp := &mockDispatcher{executor: exec}
	loader := &mockLoader{agent: makeRouterAgent()}

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	svc := NewRouterService(loader, disp, logger, DefaultRouterConfig())

	_, err := svc.Decide(context.Background(), makeState([]*models.Agent{planner}, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logged := buf.String()
	if strings.Contains(logged, canary) {
		t.Fatalf("ERR_CANARY leaked into logs (likely via err.Error()): %s", logged)
	}
	// Зато статичный LogCode ОБЯЗАН быть в логе (для аналитики).
	if !strings.Contains(logged, correctionCodeJSONParseError) {
		t.Errorf("log must contain static correction code %q for analytics, got: %s", correctionCodeJSONParseError, logged)
	}
}
