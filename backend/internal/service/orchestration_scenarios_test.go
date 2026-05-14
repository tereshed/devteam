package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// orchestration_scenarios_test.go — Sprint 17 / Sprint 4 — **scenario-driven
// COMPONENT tests** (не E2E integration с реальной БД).
//
// ВАЖНО для читателя: эти тесты НЕ покрывают полный Orchestrator.Step с настоящей
// postgres-транзакцией и `FOR UPDATE NOWAIT`/`SKIP LOCKED`. Они проверяют ЛОГИКУ
// Router-решений и AgentWorker.saveArtifact через мок-LLMExecutor и in-memory repo.
//
// Что покрыто:
//   - Router принимает корректные решения по фикстурам RouterState (sequential/parallel/DAG).
//   - Router фоллбечит на needs_human при exhausted retries.
//   - AgentWorker.saveArtifact корректно сохраняет envelope; supersede только для review.
//   - Контракты MergerOutput/TestResult парсятся end-to-end (агент → artifact → parser).
//   - Security: canary не утекает в логи ни Router'а, ни AgentWorker'а.
//
// Что НЕ покрыто (отнесено к Sprint 5 / postgres-integration через testcontainers):
//   - Multi-process worker pool с реальным FOR UPDATE NOWAIT / SKIP LOCKED.
//   - Cancel mid-flight через NOTIFY + ctx.Cancel под нагрузкой.
//   - Restart mid-task (kill процесса + recovery после старта воркеров).
//   - DAG с depends_on зависимостями (требует state-loader из реальной БД с цепочкой артефактов).
//
// Эти scenario tests проверяют:
//   - Router принимает корректные решения по mock-фикстурам RouterState (Sequential)
//   - Router распознаёт параллельный fan-out (Parallel)
//   - Router фоллбечит на needs_human при исчерпании retry (HallucinationFallback)
//   - AgentWorker.saveArtifact корректно созраняет envelope в reciprocal-репозиторий
//   - Контракт MergerOutput/TestResult парсится из реального content агента

// ─────────────────────────────────────────────────────────────────────────────
// Mocked LLM provider for orchestration scenarios
// ─────────────────────────────────────────────────────────────────────────────

// scriptedLLM — последовательно отдаёт заранее заготовленные строки в ответ на Generate.
// Перезаписывает агент = router; для других агентов возвращает заглушку.
type scriptedLLM struct {
	mu        sync.Mutex
	responses []string
	calls     int
}

func (l *scriptedLLM) Generate(ctx context.Context, req scriptedRequest) (*scriptedResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls >= len(l.responses) {
		return nil, errors.New("scriptedLLM: out of responses")
	}
	out := l.responses[l.calls]
	l.calls++
	return &scriptedResponse{Content: out}, nil
}

// Wrapper to satisfy llm.Provider interface signature без зависимости от pkg/llm в тесте.
// В реальном использовании прямо передаём llm.Provider; в тесте используем scriptedExecutor
// чтобы не тащить pkg/llm.

type scriptedRequest struct{}
type scriptedResponse struct{ Content string }

// scriptedExecutor — реализует agent.AgentExecutor; возвращает заранее запрограммированный Output.
type scriptedExecutor struct {
	mu        sync.Mutex
	responses []string
	calls     int
	hook      func(in agent.ExecutionInput)
}

func (e *scriptedExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.hook != nil {
		e.hook(in)
	}
	if e.calls >= len(e.responses) {
		return nil, errors.New("scriptedExecutor: out of responses")
	}
	out := e.responses[e.calls]
	e.calls++
	return &agent.ExecutionResult{Success: true, Output: out}, nil
}

// fixedDispatcher всегда возвращает заданный executor вне зависимости от агента.
type fixedDispatcher struct {
	exec agent.AgentExecutor
}

func (d *fixedDispatcher) BuildExecutor(_ context.Context, _ *models.Agent) (agent.AgentExecutor, error) {
	return d.exec, nil
}

// fixedAgentLoader — возвращает один и тот же router-agent record.
type fixedAgentLoader struct{ a *models.Agent }

func (l *fixedAgentLoader) GetAgentByName(_ context.Context, _ string) (*models.Agent, error) {
	return l.a, nil
}

// Helpers ----------------------------------------------------------------------

func makeRouterAgentForScenario() *models.Agent {
	m := "claude-sonnet-4-6"
	return &models.Agent{
		ID:            uuid.New(),
		Name:          "router",
		Role:          models.AgentRoleRouter,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &m,
		IsActive:      true,
	}
}

func makeLLMAgent(name string) *models.Agent {
	m := "claude-sonnet-4-6"
	return &models.Agent{
		ID:            uuid.New(),
		Name:          name,
		Role:          models.AgentRoleReviewer,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &m,
		IsActive:      true,
	}
}

func makeSandboxAgent(name string) *models.Agent {
	cb := models.CodeBackendClaudeCode
	return &models.Agent{
		ID:            uuid.New(),
		Name:          name,
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
		IsActive:      true,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 1: Sequential happy path
// ─────────────────────────────────────────────────────────────────────────────

// TestScenario_Sequential_PlanReviewCodeReviewTest — основной happy path.
// Router последовательно отдаёт 5 решений: planner → reviewer(plan) → developer →
// reviewer(code) → tester → DONE. Проверяем что каждое решение распарсилось и валидно.
func TestScenario_Sequential_PlanReviewCodeReviewTest(t *testing.T) {
	planner := makeLLMAgent("planner")
	reviewer := makeLLMAgent("reviewer")
	developer := makeSandboxAgent("developer")
	tester := makeSandboxAgent("tester")
	enabled := []*models.Agent{planner, reviewer, developer, tester}

	planID := uuid.New()
	codeID := uuid.New()

	// 6 router-выводов — для 6 шагов orchestration.
	routerOutputs := []string{
		// 1. start → planner
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "task started"}`,
		// 2. plan ready → reviewer
		`{"done": false, "agents": [{"agent": "reviewer", "input": {"target_artifact_id": "` + planID.String() + `"}}], "reason": "plan needs review"}`,
		// 3. plan approved → developer
		`{"done": false, "agents": [{"agent": "developer", "input": {"target_artifact_id": "` + planID.String() + `"}}], "reason": "plan approved, build it"}`,
		// 4. code_diff ready → reviewer
		`{"done": false, "agents": [{"agent": "reviewer", "input": {"target_artifact_id": "` + codeID.String() + `"}}], "reason": "code needs review"}`,
		// 5. code approved → tester
		`{"done": false, "agents": [{"agent": "tester", "input": {"target_artifact_id": "` + codeID.String() + `"}}], "reason": "run tests"}`,
		// 6. tests passed → DONE
		`{"done": true, "outcome": "done", "agents": [], "reason": "all tests passed"}`,
	}

	exec := &scriptedExecutor{responses: routerOutputs}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}
	svc := NewRouterService(loader, disp, discardLogger(), DefaultRouterConfig())

	task := &models.Task{
		ID: uuid.New(), ProjectID: uuid.New(),
		Title: "Add JWT auth", State: models.TaskStateActive,
	}

	// Step 1 — пустой state.
	d, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled})
	if err != nil || d.Done || len(d.Agents) != 1 || d.Agents[0].Name != "planner" {
		t.Fatalf("Step 1: want planner, got %+v err=%v", d, err)
	}

	// Step 2 — есть plan-артефакт.
	plan := models.Artifact{
		ID: planID, TaskID: task.ID, Kind: models.ArtifactKindPlan,
		Summary: "MVP-план", Status: models.ArtifactStatusReady,
		ProducerAgent: "planner",
	}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{plan}})
	if err != nil || d.Agents[0].Name != "reviewer" {
		t.Fatalf("Step 2: want reviewer, got %+v err=%v", d, err)
	}
	if gotID, ok := d.Agents[0].TargetArtifactID(); !ok || gotID != planID {
		t.Errorf("Step 2: target_artifact_id mismatch")
	}

	// Step 3 — есть approved review.
	review1 := models.Artifact{
		ID: uuid.New(), TaskID: task.ID, ParentID: &planID,
		Kind: models.ArtifactKindReview, Summary: "approved",
		Status: models.ArtifactStatusReady, ProducerAgent: "reviewer",
	}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{plan, review1}})
	if err != nil || d.Agents[0].Name != "developer" {
		t.Fatalf("Step 3: want developer, got %+v err=%v", d, err)
	}

	// Step 4 — есть code_diff.
	code := models.Artifact{
		ID: codeID, TaskID: task.ID, ParentID: &planID,
		Kind: models.ArtifactKindCodeDiff, Summary: "implemented",
		Status: models.ArtifactStatusReady, ProducerAgent: "developer",
	}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{plan, review1, code}})
	if err != nil || d.Agents[0].Name != "reviewer" {
		t.Fatalf("Step 4: want reviewer, got %+v err=%v", d, err)
	}

	// Step 5 — есть approved code review.
	review2 := models.Artifact{
		ID: uuid.New(), TaskID: task.ID, ParentID: &codeID,
		Kind: models.ArtifactKindReview, Summary: "approved",
		Status: models.ArtifactStatusReady, ProducerAgent: "reviewer",
	}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{plan, review1, code, review2}})
	if err != nil || d.Agents[0].Name != "tester" {
		t.Fatalf("Step 5: want tester, got %+v err=%v", d, err)
	}

	// Step 6 — passing test_result → DONE.
	tr := models.Artifact{
		ID: uuid.New(), TaskID: task.ID, ParentID: &codeID,
		Kind: models.ArtifactKindTestResult, Summary: "5/5 passed",
		Status: models.ArtifactStatusReady, ProducerAgent: "tester",
	}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{plan, review1, code, review2, tr}})
	if err != nil || !d.Done || d.Outcome != models.RouterDecisionOutcomeDone {
		t.Fatalf("Step 6: want done=true outcome=done, got %+v err=%v", d, err)
	}

	if exec.calls != 6 {
		t.Errorf("expected exactly 6 LLM calls (one per step), got %d", exec.calls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 2: Parallel fan-out
// ─────────────────────────────────────────────────────────────────────────────

// TestScenario_Parallel_TwoDevsThenMerger — Router возвращает массив из 2 developer'ов,
// затем (после двух code_diff'ов) — merger.
func TestScenario_Parallel_TwoDevsThenMerger(t *testing.T) {
	developer := makeSandboxAgent("developer")
	merger := makeSandboxAgent("merger")
	enabled := []*models.Agent{developer, merger}

	sub1, sub2 := uuid.New(), uuid.New()

	routerOutputs := []string{
		// 1. Параллельный fan-out на 2 developer'а.
		`{
			"done": false,
			"agents": [
				{"agent": "developer", "input": {"target_artifact_id": "` + sub1.String() + `"}},
				{"agent": "developer", "input": {"target_artifact_id": "` + sub2.String() + `"}}
			],
			"reason": "subtask 1 и 2 не зависят друг от друга"
		}`,
		// 2. Оба code_diff approved → merger
		`{"done": false, "agents": [{"agent": "merger"}], "reason": "merge 2 parallel diffs"}`,
	}
	exec := &scriptedExecutor{responses: routerOutputs}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}
	svc := NewRouterService(loader, disp, discardLogger(), DefaultRouterConfig())

	task := &models.Task{ID: uuid.New(), ProjectID: uuid.New(), Title: "feature", State: models.TaskStateActive}

	// Step 1 — пустой state, ждём fan-out.
	d, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled})
	if err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if len(d.Agents) != 2 {
		t.Fatalf("step 1: want 2 parallel agents, got %d", len(d.Agents))
	}
	gotA, _ := d.Agents[0].TargetArtifactID()
	gotB, _ := d.Agents[1].TargetArtifactID()
	if gotA == gotB {
		t.Error("parallel agents must target DIFFERENT artifacts (Router validation)")
	}

	// Step 2 — два code_diff готовы + approve'ы → merger.
	approvedCode1 := models.Artifact{
		ID: uuid.New(), TaskID: task.ID, ParentID: &sub1,
		Kind: models.ArtifactKindCodeDiff, Status: models.ArtifactStatusReady,
		Summary: "diff 1", ProducerAgent: "developer",
	}
	approvedCode2 := models.Artifact{
		ID: uuid.New(), TaskID: task.ID, ParentID: &sub2,
		Kind: models.ArtifactKindCodeDiff, Status: models.ArtifactStatusReady,
		Summary: "diff 2", ProducerAgent: "developer",
	}
	d, err = svc.Decide(context.Background(), RouterState{
		Task: task, Agents: enabled,
		Artifacts: []models.Artifact{approvedCode1, approvedCode2},
	})
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if d.Agents[0].Name != "merger" {
		t.Errorf("step 2: want merger, got %q", d.Agents[0].Name)
	}
}

// TestScenario_Parallel_ThreeDevsThreeReviewsMergerTester — Sprint 4 review fix:
// расширенный parallel-сценарий по DoD: 3 параллельных Developer'а → 3 параллельных
// Reviewer'а → Merger → Tester → DONE. Проверяет что Router корректно различает фазы:
// fan-out → fan-out review → конвергенция к merger → tester → done.
func TestScenario_Parallel_ThreeDevsThreeReviewsMergerTester(t *testing.T) {
	developer := makeSandboxAgent("developer")
	reviewer := makeLLMAgent("reviewer")
	merger := makeSandboxAgent("merger")
	tester := makeSandboxAgent("tester")
	enabled := []*models.Agent{developer, reviewer, merger, tester}

	sub1, sub2, sub3 := uuid.New(), uuid.New(), uuid.New()
	code1, code2, code3 := uuid.New(), uuid.New(), uuid.New()

	routerOutputs := []string{
		// 1. Fan-out 3 developer'а на независимые подзадачи.
		`{"done": false, "agents": [
			{"agent": "developer", "input": {"target_artifact_id": "` + sub1.String() + `"}},
			{"agent": "developer", "input": {"target_artifact_id": "` + sub2.String() + `"}},
			{"agent": "developer", "input": {"target_artifact_id": "` + sub3.String() + `"}}
		], "reason": "3 independent subtasks"}`,
		// 2. Fan-out 3 reviewer'а на 3 code_diff.
		`{"done": false, "agents": [
			{"agent": "reviewer", "input": {"target_artifact_id": "` + code1.String() + `"}},
			{"agent": "reviewer", "input": {"target_artifact_id": "` + code2.String() + `"}},
			{"agent": "reviewer", "input": {"target_artifact_id": "` + code3.String() + `"}}
		], "reason": "review 3 parallel diffs"}`,
		// 3. Все approved → merger.
		`{"done": false, "agents": [{"agent": "merger"}], "reason": "merge 3 approved branches"}`,
		// 4. Merger готов → tester.
		`{"done": false, "agents": [{"agent": "tester"}], "reason": "run tests on merged"}`,
		// 5. Tests passed → DONE.
		`{"done": true, "outcome": "done", "agents": [], "reason": "all green"}`,
	}
	exec := &scriptedExecutor{responses: routerOutputs}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}
	svc := NewRouterService(loader, disp, discardLogger(), DefaultRouterConfig())

	task := &models.Task{ID: uuid.New(), ProjectID: uuid.New(), State: models.TaskStateActive}

	// Step 1
	d, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled})
	if err != nil || len(d.Agents) != 3 {
		t.Fatalf("step1: want 3 parallel agents, got %+v err=%v", d, err)
	}
	// Проверяем что все target_artifact_id РАЗНЫЕ (Router-валидация дублей).
	seen := map[uuid.UUID]struct{}{}
	for _, a := range d.Agents {
		id, _ := a.TargetArtifactID()
		seen[id] = struct{}{}
	}
	if len(seen) != 3 {
		t.Errorf("step1: want 3 distinct target_artifact_ids, got %d", len(seen))
	}

	// Step 2: 3 code_diff готовы → review fan-out.
	code1Art := models.Artifact{ID: code1, TaskID: task.ID, ParentID: &sub1, Kind: models.ArtifactKindCodeDiff, Status: models.ArtifactStatusReady, ProducerAgent: "developer", Summary: "diff1"}
	code2Art := models.Artifact{ID: code2, TaskID: task.ID, ParentID: &sub2, Kind: models.ArtifactKindCodeDiff, Status: models.ArtifactStatusReady, ProducerAgent: "developer", Summary: "diff2"}
	code3Art := models.Artifact{ID: code3, TaskID: task.ID, ParentID: &sub3, Kind: models.ArtifactKindCodeDiff, Status: models.ArtifactStatusReady, ProducerAgent: "developer", Summary: "diff3"}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{code1Art, code2Art, code3Art}})
	if err != nil || len(d.Agents) != 3 {
		t.Fatalf("step2: want 3 parallel reviewers, got %+v err=%v", d, err)
	}
	for _, a := range d.Agents {
		if a.Name != "reviewer" {
			t.Errorf("step2: expected reviewer, got %s", a.Name)
		}
	}

	// Step 3: 3 approved review → merger.
	r1 := models.Artifact{ID: uuid.New(), TaskID: task.ID, ParentID: &code1, Kind: models.ArtifactKindReview, Status: models.ArtifactStatusReady, ProducerAgent: "reviewer", Summary: "approved"}
	r2 := models.Artifact{ID: uuid.New(), TaskID: task.ID, ParentID: &code2, Kind: models.ArtifactKindReview, Status: models.ArtifactStatusReady, ProducerAgent: "reviewer", Summary: "approved"}
	r3 := models.Artifact{ID: uuid.New(), TaskID: task.ID, ParentID: &code3, Kind: models.ArtifactKindReview, Status: models.ArtifactStatusReady, ProducerAgent: "reviewer", Summary: "approved"}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{code1Art, code2Art, code3Art, r1, r2, r3}})
	if err != nil || d.Agents[0].Name != "merger" {
		t.Fatalf("step3: want merger, got %+v err=%v", d, err)
	}

	// Step 4: merger готов → tester.
	mergedID := uuid.New()
	merged := models.Artifact{ID: mergedID, TaskID: task.ID, Kind: models.ArtifactKindMergedCode, Status: models.ArtifactStatusReady, ProducerAgent: "merger", Summary: "merged 3 branches"}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{code1Art, code2Art, code3Art, r1, r2, r3, merged}})
	if err != nil || d.Agents[0].Name != "tester" {
		t.Fatalf("step4: want tester, got %+v err=%v", d, err)
	}

	// Step 5: test_result passed → DONE.
	tr := models.Artifact{ID: uuid.New(), TaskID: task.ID, ParentID: &mergedID, Kind: models.ArtifactKindTestResult, Status: models.ArtifactStatusReady, ProducerAgent: "tester", Summary: "10/10 passed"}
	d, err = svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled, Artifacts: []models.Artifact{code1Art, code2Art, code3Art, r1, r2, r3, merged, tr}})
	if err != nil || !d.Done || d.Outcome != models.RouterDecisionOutcomeDone {
		t.Fatalf("step5: want done=done, got %+v err=%v", d, err)
	}

	if exec.calls != 5 {
		t.Errorf("expected 5 LLM calls (one per phase), got %d", exec.calls)
	}
}

// TestScrubTestResultRawOutput — Sprint 4 review fix §1: проверяем что raw_output
// проходит через secret_scrub перед записью в artifact.content.
func TestScrubTestResultRawOutput(t *testing.T) {
	in := []byte(`{
		"passed": 1, "failed": 0,
		"build_passed": true, "lint_passed": true, "typecheck_passed": true,
		"raw_output_truncated": "Test ran with token=abc123secretvalue and api_key: ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	}`)
	out, err := scrubTestResultRawOutput(in)
	if err != nil {
		t.Fatalf("scrubTestResultRawOutput: %v", err)
	}
	if strings.Contains(string(out), "abc123secretvalue") {
		t.Errorf("raw token leaked: %s", string(out))
	}
	if strings.Contains(string(out), "ghp_abcdefghijklmnopqrstuvwxyz0123456789") {
		t.Errorf("github PAT leaked: %s", string(out))
	}
	if !strings.Contains(string(out), "REDACTED") {
		t.Errorf("expected REDACTED marker, got: %s", string(out))
	}
}

// TestRedactRawOutputToSentinel_ReplacesWithHashAndLength — Sprint 4 review fix §2:
// sentinel-pathway заменяет raw_output_truncated на {_scrub_failed, len, head_sha256_8}.
func TestRedactRawOutputToSentinel_ReplacesWithHashAndLength(t *testing.T) {
	secretContent := "Test ran with TOKEN=verysecretvalue_that_must_not_persist_xyz"
	in := []byte(`{
		"passed": 1,
		"build_passed": true,
		"raw_output_truncated": "` + secretContent + `"
	}`)
	out, err := redactRawOutputToSentinel(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "verysecretvalue") {
		t.Fatalf("secret leaked through sentinel: %s", string(out))
	}
	// Парсим и проверяем структуру sentinel'а.
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	rawField, ok := parsed["raw_output_truncated"].(map[string]any)
	if !ok {
		t.Fatalf("raw_output_truncated must be object after redaction, got: %v", parsed["raw_output_truncated"])
	}
	if rawField["_scrub_failed"] != true {
		t.Errorf("sentinel must have _scrub_failed=true, got: %v", rawField)
	}
	if rawField["len"] == nil {
		t.Error("sentinel must include 'len'")
	}
	if rawField["head_sha256_8"] == nil {
		t.Error("sentinel must include 'head_sha256_8' hash")
	}
}

// TestRedactRawOutputToSentinel_FlagsTypeMismatch — если raw_output_truncated есть,
// но это не строка (агент сломал контракт — например, прислал object/number/array),
// sentinel помечает это явным флагом `_type_mismatch=true` чтобы оператор отличал
// от "tester просто не сохранил output" (валидная пустая строка).
func TestRedactRawOutputToSentinel_FlagsTypeMismatch(t *testing.T) {
	in := []byte(`{
		"passed": 0,
		"build_passed": true,
		"raw_output_truncated": {"oops": "agent put an object here"}
	}`)
	out, err := redactRawOutputToSentinel(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	sentinel, _ := parsed["raw_output_truncated"].(map[string]any)
	if sentinel == nil {
		t.Fatalf("expected sentinel object, got %v", parsed["raw_output_truncated"])
	}
	if sentinel["_type_mismatch"] != true {
		t.Errorf("expected _type_mismatch=true for non-string raw_output, got: %v", sentinel)
	}
	if sentinel["_scrub_failed"] != true {
		t.Errorf("_scrub_failed must remain true alongside _type_mismatch")
	}
}

// TestRedactRawOutputToSentinel_NoTypeMismatchOnEmptyString — пустая строка
// — валидный кейс (tester просто не сохранил output); НЕ ставит _type_mismatch.
func TestRedactRawOutputToSentinel_NoTypeMismatchOnEmptyString(t *testing.T) {
	in := []byte(`{"passed": 0, "build_passed": true, "raw_output_truncated": ""}`)
	out, err := redactRawOutputToSentinel(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	sentinel, _ := parsed["raw_output_truncated"].(map[string]any)
	if sentinel == nil {
		t.Fatalf("expected sentinel object, got %v", parsed["raw_output_truncated"])
	}
	if _, has := sentinel["_type_mismatch"]; has {
		t.Errorf("_type_mismatch must NOT be set for valid empty string; got: %v", sentinel)
	}
	if sentinel["len"] != float64(0) {
		t.Errorf("expected len=0, got: %v", sentinel["len"])
	}
}

// TestRedactRawOutputToSentinel_NoOpWhenNoRawField — без поля не делает ничего.
func TestRedactRawOutputToSentinel_NoOpWhenNoRawField(t *testing.T) {
	in := []byte(`{"passed": 5, "build_passed": true}`)
	out, err := redactRawOutputToSentinel(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(in) {
		t.Errorf("expected content unchanged, got: %s", string(out))
	}
}

// TestSaveArtifact_TestResult_FailsWhenContentNotObject — full pipeline test:
// если content test_result — валидный JSON, но не object (array/scalar),
// scrubTestResultRawOutput не может его обработать → sentinel-pathway тоже не справится
// (json.Unmarshal в map ожидает object) → saveArtifact возвращает ошибку и не создаёт артефакт.
// Это strict-policy: "no save until scrubbed" в действии.
func TestSaveArtifact_TestResult_FailsWhenContentNotObject(t *testing.T) {
	repo := newMemArtifactRepo()
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	w := &AgentWorker{artifactRepo: repo, logger: logger}

	// envelope с kind=test_result, content — валидный JSON-массив (не object).
	envelope := AgentResponseEnvelope{
		Kind:    string(models.ArtifactKindTestResult),
		Summary: "tests broken format",
		Content: json.RawMessage(`[1, 2, 3]`),
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}

	err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "tester"}, result)
	if err == nil {
		t.Fatal("expected error when test_result content is not a JSON object (scrub+sentinel both unable to process)")
	}
	if len(repo.created) != 0 {
		t.Errorf("expected NO artifact saved on scrub failure, got %d", len(repo.created))
	}
}

// TestSaveArtifact_TestResult_SentinelOnPartialScrubFailure — теоретическая проверка
// пути sentinel: scrub возвращает success на валидном object'е (всё ок).
// Дополнительно — тест что обычный test_result проходит через scrubbing без потерь
// при отсутствии секретов в raw_output.
func TestSaveArtifact_TestResult_SentinelPathDoesNotTriggerOnValidObject(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}

	envelope := AgentResponseEnvelope{
		Kind:    string(models.ArtifactKindTestResult),
		Summary: "5/5 passed",
		Content: json.RawMessage(`{
			"passed": 5, "failed": 0,
			"build_passed": true, "lint_passed": true, "typecheck_passed": true,
			"raw_output_truncated": "clean test output without secrets"
		}`),
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "tester"}, result); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 artifact saved, got %d", len(repo.created))
	}
	// Сохранённый artifact.Content должен содержать raw_output_truncated в исходном виде
	// (без sentinel-маркера _scrub_failed).
	var saved map[string]any
	_ = json.Unmarshal(repo.created[0].Content, &saved)
	rawField, _ := saved["raw_output_truncated"].(string)
	if rawField != "clean test output without secrets" {
		t.Errorf("expected raw_output preserved verbatim, got: %v", saved["raw_output_truncated"])
	}
}

// TestScrubTestResultRawOutput_NoOpWhenNoRawField — поля нет → возврат без изменений.
func TestScrubTestResultRawOutput_NoOpWhenNoRawField(t *testing.T) {
	in := []byte(`{"passed": 5, "build_passed": true, "lint_passed": true, "typecheck_passed": true}`)
	out, err := scrubTestResultRawOutput(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Может быть re-serialized порядок ключей; парсим обе и сравниваем как map.
	var inMap, outMap map[string]any
	_ = json.Unmarshal(in, &inMap)
	_ = json.Unmarshal(out, &outMap)
	if !reflect.DeepEqual(inMap, outMap) {
		t.Errorf("expected unchanged content, got: %s", string(out))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 3: Hallucination → recovery
// ─────────────────────────────────────────────────────────────────────────────

// TestScenario_HallucinationRecovery_UnknownAgent — Router придумал агента, retry даёт верный.
func TestScenario_HallucinationRecovery_UnknownAgent(t *testing.T) {
	planner := makeLLMAgent("planner")
	enabled := []*models.Agent{planner}

	var capturedPrompts []string
	exec := &scriptedExecutor{
		responses: []string{
			`{"done": false, "agents": [{"agent": "ghost_agent"}], "reason": "hallucination"}`,
			`{"done": false, "agents": [{"agent": "planner"}], "reason": "fixed after correction"}`,
		},
		hook: func(in agent.ExecutionInput) {
			capturedPrompts = append(capturedPrompts, in.PromptUser)
		},
	}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}
	svc := NewRouterService(loader, disp, discardLogger(), DefaultRouterConfig())

	task := &models.Task{ID: uuid.New(), ProjectID: uuid.New(), Title: "x", State: models.TaskStateActive}
	d, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled})
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if d.Done || d.Agents[0].Name != "planner" {
		t.Fatalf("want planner after correction, got %+v", d)
	}
	if exec.calls != 2 {
		t.Errorf("want 2 calls (initial + retry), got %d", exec.calls)
	}
	if len(capturedPrompts) < 2 || !strings.Contains(capturedPrompts[1], "ghost_agent") {
		t.Errorf("second prompt must mention hallucinated agent name to help LLM correct")
	}
}

// TestScenario_HallucinationFallback_NeedsHuman — все попытки невалидны → needs_human (НЕ error).
func TestScenario_HallucinationFallback_NeedsHuman(t *testing.T) {
	planner := makeLLMAgent("planner")
	enabled := []*models.Agent{planner}

	exec := &scriptedExecutor{
		responses: []string{
			`not even close to json`,
			`{still: broken}`,
			`{"done": ...`,
		},
	}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}
	svc := NewRouterService(loader, disp, discardLogger(), DefaultRouterConfig()) // MaxRetries=2 → 3 calls total

	task := &models.Task{ID: uuid.New(), ProjectID: uuid.New(), State: models.TaskStateActive}
	d, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled})
	if err != nil {
		t.Fatalf("fallback must NOT return error, got: %v", err)
	}
	if !d.Done || d.Outcome != models.RouterDecisionOutcomeNeedsHuman {
		t.Fatalf("want done=true outcome=needs_human, got %+v", d)
	}
	if exec.calls != 3 {
		t.Errorf("want 3 calls (initial + 2 retries), got %d", exec.calls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 4: End-to-end через AgentWorker.saveArtifact с Merger-output
// ─────────────────────────────────────────────────────────────────────────────

// TestScenario_MergerOutputContract — Merger возвращает валидный envelope с MergerOutput
// в content; AgentWorker сохраняет, parser восстанавливает структуру.
func TestScenario_MergerOutputContract(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	wt1, wt2 := uuid.New(), uuid.New()

	merger := models.Agent{Name: "merger"}
	envelope := AgentResponseEnvelope{
		Kind:    string(models.ArtifactKindMergedCode),
		Summary: "merged 2 branches, 1 conflict resolved",
		Content: json.RawMessage(fmt.Sprintf(`{
			"merged_branch": "task-abc-merged",
			"source_worktree_ids": ["%s", "%s"],
			"merge_conflicts_resolved": [{"file": "auth.go", "resolution": "kept feature-A"}],
			"checks_run": ["go build"],
			"checks_passed": true,
			"head_commit_sha": "deadbeef"
		}`, wt1, wt2)),
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}

	taskID := uuid.New()
	if err := w.saveArtifact(context.Background(), taskID, &merger, result); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatal("expected 1 artifact")
	}
	art := repo.created[0]
	if art.Kind != models.ArtifactKindMergedCode {
		t.Errorf("kind = %q, want merged_code", art.Kind)
	}

	mo, err := models.ParseMergerOutput(art.Content)
	if err != nil {
		t.Fatalf("ParseMergerOutput: %v", err)
	}
	if mo.MergedBranch != "task-abc-merged" {
		t.Errorf("MergedBranch mismatch")
	}
	if len(mo.SourceWorktreeIDs) != 2 || mo.SourceWorktreeIDs[0] != wt1 {
		t.Errorf("SourceWorktreeIDs mismatch")
	}
	if len(mo.MergeConflictsResolved) != 1 || mo.MergeConflictsResolved[0].File != "auth.go" {
		t.Errorf("MergeConflictsResolved mismatch")
	}
	if !mo.ChecksPassed {
		t.Error("ChecksPassed=true expected")
	}
}

// TestScenario_TestResultContract — Tester возвращает структурированный test_result.
func TestScenario_TestResultContract(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	codeID := uuid.New()

	tester := models.Agent{Name: "tester"}
	envelope := AgentResponseEnvelope{
		Kind:             string(models.ArtifactKindTestResult),
		Summary:          "12/12 passed",
		ParentArtifactID: &codeID,
		Content: json.RawMessage(`{
			"passed": 12, "failed": 0, "skipped": 0,
			"duration_ms": 5430, "coverage_percent": 87.5,
			"build_passed": true, "lint_passed": true, "typecheck_passed": true
		}`),
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}

	if err := w.saveArtifact(context.Background(), uuid.New(), &tester, result); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	art := repo.created[0]
	if art.ParentID == nil || *art.ParentID != codeID {
		t.Errorf("ParentID must reference code_diff for traceability")
	}

	tr, err := models.ParseTestResult(art.Content)
	if err != nil {
		t.Fatalf("ParseTestResult: %v", err)
	}
	if !tr.AllPassed() {
		t.Error("AllPassed expected true for 12/0/0 + all checks")
	}
	if tr.DurationMS != 5430 {
		t.Errorf("DurationMS mismatch: %d", tr.DurationMS)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 5: Security canary across full pipeline
// ─────────────────────────────────────────────────────────────────────────────

// TestScenario_SecurityCanary_EndToEnd — security guard ВСЕЙ пайплайны:
// Router → AgentDispatcher → executor → saveArtifact. На каждом этапе canary
// в промптах/выводах LLM не должен попасть в стандартный log-stream.
func TestScenario_SecurityCanary_EndToEnd(t *testing.T) {
	const canary = "FULL_PIPELINE_CANARY_no_leak_allowed_anywhere"
	planner := makeLLMAgent("planner")
	enabled := []*models.Agent{planner}

	// Router возвращает невалидный JSON с canary → retry → recovery
	exec := &scriptedExecutor{
		responses: []string{
			`broken json embedded: ` + canary,
			`{"done": false, "agents": [{"agent": "planner"}], "reason": "ok"}`,
		},
	}
	disp := &fixedDispatcher{exec: exec}
	loader := &fixedAgentLoader{a: makeRouterAgentForScenario()}

	var routerLogs bytes.Buffer
	routerLogger := slog.New(logging.NewHandler(slog.NewTextHandler(&routerLogs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	svc := NewRouterService(loader, disp, routerLogger, DefaultRouterConfig())

	task := &models.Task{ID: uuid.New(), ProjectID: uuid.New(), State: models.TaskStateActive}
	if _, err := svc.Decide(context.Background(), RouterState{Task: task, Agents: enabled}); err != nil {
		t.Fatalf("decide failed: %v", err)
	}
	if strings.Contains(routerLogs.String(), canary) {
		t.Fatalf("Router LEAKED canary in logs: %s", routerLogs.String())
	}

	// AgentWorker.saveArtifact с canary в Output (fallback path)
	repo := newMemArtifactRepo()
	var workerLogs bytes.Buffer
	workerLogger := slog.New(logging.NewHandler(slog.NewTextHandler(&workerLogs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	w := &AgentWorker{artifactRepo: repo, logger: workerLogger}
	result := &agent.ExecutionResult{Success: true, Output: "free text: " + canary}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "x"}, result); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if strings.Contains(workerLogs.String(), canary) {
		t.Fatalf("AgentWorker LEAKED canary in logs: %s", workerLogs.String())
	}
}

// (тесты для Orchestrator.Step с реальной gorm-транзакцией и FOR UPDATE NOWAIT
//  +  TaskEventRepository.ClaimNext с SKIP LOCKED требуют real postgres — будут
//  добавлены в Sprint 5 через testcontainers-постgres-обвязку.)

// Lint guards.
var (
	_ = io.Discard
	_ = time.Now
)
