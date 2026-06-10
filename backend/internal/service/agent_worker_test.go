package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory ArtifactRepository (для теста без gorm/sqlite)
// ─────────────────────────────────────────────────────────────────────────────

type memArtifactRepo struct {
	created    []models.Artifact
	superseded []supersedeCall
}

type supersedeCall struct {
	taskID   uuid.UUID
	parentID *uuid.UUID
	kind     models.ArtifactKind
}

func newMemArtifactRepo() *memArtifactRepo { return &memArtifactRepo{} }

func (r *memArtifactRepo) Create(_ context.Context, a *models.Artifact) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.created = append(r.created, *a)
	return nil
}
func (r *memArtifactRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Artifact, error) {
	for i := range r.created {
		if r.created[i].ID == id {
			cp := r.created[i]
			return &cp, nil
		}
	}
	return nil, repository.ErrArtifactNotFound
}
func (r *memArtifactRepo) ListByTaskID(_ context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error) {
	out := make([]models.Artifact, 0)
	for _, a := range r.created {
		if a.TaskID == taskID && (!onlyReady || a.Status == models.ArtifactStatusReady) {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *memArtifactRepo) ListMetadataByTaskID(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error) {
	return r.ListByTaskID(ctx, taskID, onlyReady)
}
func (r *memArtifactRepo) SupersedePrevious(_ context.Context, taskID uuid.UUID, parentID *uuid.UUID, kind models.ArtifactKind) (int64, error) {
	r.superseded = append(r.superseded, supersedeCall{taskID, parentID, kind})
	return 0, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

// TestParseAgentEnvelope_ValidEnvelope — стандартный happy path.
func TestParseAgentEnvelope_ValidEnvelope(t *testing.T) {
	result := &agent.ExecutionResult{
		Success: true,
		Output: `{
			"kind": "plan",
			"summary": "MVP-план из 3 пунктов",
			"content": {"steps": [{"id": "1"}]}
		}`,
	}
	env, ok := parseAgentEnvelope(result, "planner")
	if !ok {
		t.Fatal("expected successful envelope parse")
	}
	if env.Kind != "plan" {
		t.Errorf("Kind = %q, want plan", env.Kind)
	}
	if env.Summary != "MVP-план из 3 пунктов" {
		t.Errorf("Summary mismatch: %q", env.Summary)
	}
}

// TestParseAgentEnvelope_FromArtifactsJSON — envelope извлекается из ArtifactsJSON
// (когда LLMAgentExecutor нашёл ```json ... ``` блок).
func TestParseAgentEnvelope_FromArtifactsJSON(t *testing.T) {
	result := &agent.ExecutionResult{
		Success:       true,
		ArtifactsJSON: json.RawMessage(`{"kind": "review", "summary": "approved"}`),
		Output:        "Verbose human-readable analysis text that's NOT JSON",
	}
	env, ok := parseAgentEnvelope(result, "reviewer")
	if !ok {
		t.Fatal("expected successful parse from ArtifactsJSON")
	}
	if env.Kind != "review" {
		t.Errorf("Kind = %q, want review", env.Kind)
	}
}

// TestParseAgentEnvelope_MarkdownFenced — envelope извлекается из Output, когда он обернут в markdown fences.
func TestParseAgentEnvelope_MarkdownFenced(t *testing.T) {
	result := &agent.ExecutionResult{
		Success: true,
		Output: `Some verbose logs before
` + "```json" + `
{
	"kind": "review",
	"summary": "fenced review",
	"content": {"approved": true}
}
` + "```" + `
Some verbose logs after`,
	}
	env, ok := parseAgentEnvelope(result, "reviewer")
	if !ok {
		t.Fatal("expected successful parse from fenced markdown in Output")
	}
	if env.Kind != "review" {
		t.Errorf("Kind = %q, want review", env.Kind)
	}
	if env.Summary != "fenced review" {
		t.Errorf("Summary = %q, want fenced review", env.Summary)
	}
}

// TestParseAgentEnvelope_FallbackOnNonJSON — агент вернул свободный текст, не JSON.
func TestParseAgentEnvelope_FallbackOnNonJSON(t *testing.T) {
	result := &agent.ExecutionResult{
		Success: true,
		Output:  "Sorry, I couldn't format as JSON. Here's my answer: ...",
	}
	_, ok := parseAgentEnvelope(result, "developer")
	if ok {
		t.Error("expected parse to FAIL for non-JSON, so caller uses fallback path")
	}
}

// TestParseAgentEnvelope_FallbackOnEmptyKind — JSON без kind = невалидный envelope.
func TestParseAgentEnvelope_FallbackOnEmptyKind(t *testing.T) {
	result := &agent.ExecutionResult{
		Success: true,
		Output:  `{"summary": "no kind here", "content": {}}`,
	}
	_, ok := parseAgentEnvelope(result, "developer")
	if ok {
		t.Error("expected parse to FAIL when kind is empty")
	}
}

// TestParseAgentEnvelope_DirectReview — парсинг прямого JSON ревью с оборачиванием в envelope.
func TestParseAgentEnvelope_DirectReview(t *testing.T) {
	target := &models.Artifact{
		ID:            uuid.New(),
		ProducerAgent: "planner",
		Kind:          models.ArtifactKindPlan,
	}

	result := &agent.ExecutionResult{
		Success: true,
		Output: `Some logs
` + "```json" + `
{
	"decision": "approve",
	"comments": [{"message": "all good"}]
}
` + "```" + `
More logs`,
	}

	env, ok := parseAgentEnvelope(result, "reviewer", target)
	if !ok {
		t.Fatal("expected successful parse of direct review JSON")
	}
	if env.Kind != "review" {
		t.Errorf("Kind = %q, want review", env.Kind)
	}
	if env.Summary != "Review decision: approved" {
		t.Errorf("Summary = %q, want Review decision: approved", env.Summary)
	}
	if env.ParentArtifactID == nil || *env.ParentArtifactID != target.ID {
		t.Errorf("ParentArtifactID mismatch")
	}
}

// TestParseAgentEnvelope_DirectTestResult — парсинг прямого JSON тестов с оборачиванием в envelope.
func TestParseAgentEnvelope_DirectTestResult(t *testing.T) {
	target := &models.Artifact{
		ID:            uuid.New(),
		ProducerAgent: "developer",
		Kind:          models.ArtifactKindCodeDiff,
	}

	result := &agent.ExecutionResult{
		Success: true,
		Output:  `{"decision": "passed", "test_result": "pass", "summary": "tests passed successfully"}`,
	}

	env, ok := parseAgentEnvelope(result, "tester", target)
	if !ok {
		t.Fatal("expected successful parse of direct test result JSON")
	}
	if env.Kind != "test_result" {
		t.Errorf("Kind = %q, want test_result", env.Kind)
	}
	if env.Summary != "tests passed successfully" {
		t.Errorf("Summary = %q, want tests passed successfully", env.Summary)
	}
	if env.ParentArtifactID == nil || *env.ParentArtifactID != target.ID {
		t.Errorf("ParentArtifactID mismatch")
	}
}

// TestParseAgentEnvelope_DirectTestResultBlocked — вердикт blocked (миграция 082:
// суть задачи непроверяема в sandbox) распознаётся как test_result наравне с passed/failed.
func TestParseAgentEnvelope_DirectTestResultBlocked(t *testing.T) {
	target := &models.Artifact{
		ID:            uuid.New(),
		ProducerAgent: "developer",
		Kind:          models.ArtifactKindCodeDiff,
	}

	result := &agent.ExecutionResult{
		Success: true,
		Output:  `{"decision": "blocked", "summary": "blocked: integration layer requires Postgres, not available in sandbox"}`,
	}

	env, ok := parseAgentEnvelope(result, "tester", target)
	if !ok {
		t.Fatal("expected successful parse of blocked test result JSON")
	}
	if env.Kind != "test_result" {
		t.Errorf("Kind = %q, want test_result", env.Kind)
	}
	if env.Summary != "blocked: integration layer requires Postgres, not available in sandbox" {
		t.Errorf("Summary = %q, want blocked summary", env.Summary)
	}
	if env.ParentArtifactID == nil || *env.ParentArtifactID != target.ID {
		t.Errorf("ParentArtifactID mismatch")
	}
}

// TestParseAgentEnvelope_WithPreambleAndDirectReview — парсинг прямого JSON ревью с не-JSON преамбулой без markdown fences.
func TestParseAgentEnvelope_WithPreambleAndDirectReview(t *testing.T) {
	target := &models.Artifact{
		ID:            uuid.New(),
		ProducerAgent: "decomposer",
		Kind:          models.ArtifactKindSubtaskDescription,
	}

	result := &agent.ExecutionResult{
		Success: true,
		Output: `Cloning into '/workspace/repo'...
From https://github.com/tereshed/spanish-tutor-app
 * branch            main       -> FETCH_HEAD
Reset branch 'main'
branch 'main' set up to track 'origin/main'.
Your branch is up to date with 'origin/main'.
Warning: Unknown toolsets: file_ops, shell

session_id: 20260525_183715_59b273
{
  "kind": "review",
  "summary": "changes_requested: Subtask descriptions lack details",
  "parent_artifact_id": "50d891b8-98af-4fde-b94b-a134f996f923",
  "content": {
    "decision": "changes_requested",
    "issues": [{"severity": "major", "comment": "Subtask 4 requires PR"}]
  }
}
`,
	}

	env, ok := parseAgentEnvelope(result, "reviewer", target)
	if !ok {
		t.Fatal("expected successful parse of review JSON even with preamble")
	}
	if env.Kind != "review" {
		t.Errorf("Kind = %q, want review", env.Kind)
	}
	if env.Summary != "changes_requested: Subtask descriptions lack details" {
		t.Errorf("Summary = %q", env.Summary)
	}
	if env.ParentArtifactID == nil || env.ParentArtifactID.String() != "50d891b8-98af-4fde-b94b-a134f996f923" {
		t.Errorf("ParentArtifactID mismatch")
	}
}

// TestParseAgentEnvelope_WithShortParentArtifactID — парсинг JSON с некорректным UUID в parent_artifact_id (например, короткий хэш).
func TestParseAgentEnvelope_WithShortParentArtifactID(t *testing.T) {
	target := &models.Artifact{
		ID:            uuid.New(),
		ProducerAgent: "decomposer",
		Kind:          models.ArtifactKindSubtaskDescription,
	}

	result := &agent.ExecutionResult{
		Success: true,
		Output: `{
  "kind": "review",
  "summary": "changes_requested: Subtask descriptions lack details",
  "parent_artifact_id": "50d891b8",
  "content": {
    "decision": "changes_requested",
    "issues": [{"severity": "major", "comment": "Subtask 4 requires PR"}]
  }
}
`,
	}

	env, ok := parseAgentEnvelope(result, "reviewer", target)
	if !ok {
		t.Fatal("expected successful parse even with short parent_artifact_id")
	}
	if env.Kind != "review" {
		t.Errorf("Kind = %q, want review", env.Kind)
	}
	// Должен подставиться target.ID, т.к. "50d891b8" не распарсился как UUID
	if env.ParentArtifactID == nil || *env.ParentArtifactID != target.ID {
		t.Errorf("ParentArtifactID mismatch: got %v, want %s", env.ParentArtifactID, target.ID)
	}
}

// TestSaveArtifact_HappyPath — заваливаем валидный envelope, ожидаем созданный артефакт
// в repo с теми же kind/summary, status=ready.
func TestSaveArtifact_HappyPath(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{
		artifactRepo: repo,
		logger:       discardLogger(),
	}
	agentRec := &models.Agent{Name: "planner"}
	result := &agent.ExecutionResult{
		Success: true,
		Output:  `{"kind": "plan", "summary": "test plan", "content": {"steps": []}}`,
	}
	taskID := uuid.New()
	if err := w.saveArtifact(context.Background(), taskID, agentRec, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(repo.created))
	}
	got := repo.created[0]
	if got.TaskID != taskID {
		t.Errorf("TaskID mismatch")
	}
	if got.Kind != models.ArtifactKindPlan {
		t.Errorf("Kind = %q, want plan", got.Kind)
	}
	if got.Summary != "test plan" {
		t.Errorf("Summary mismatch: %q", got.Summary)
	}
	if got.Status != models.ArtifactStatusReady {
		t.Errorf("Status = %q, want ready", got.Status)
	}
	if got.ProducerAgent != "planner" {
		t.Errorf("ProducerAgent = %q, want planner", got.ProducerAgent)
	}
}

// TestSaveArtifact_FallbackOnInvalidEnvelope — агент не выдал envelope; сохраняем
// kind='raw_output' с урезанным summary, чтобы цепочка не падала.
func TestSaveArtifact_FallbackOnInvalidEnvelope(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{
		artifactRepo: repo,
		logger:       discardLogger(),
	}
	agentRec := &models.Agent{Name: "unknown_agent"}
	result := &agent.ExecutionResult{
		Success: true,
		Output:  "Я сделал то-то и то-то, без JSON-обёртки, прости.",
	}
	if err := w.saveArtifact(context.Background(), uuid.New(), agentRec, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(repo.created))
	}
	got := repo.created[0]
	if got.Kind != "raw_output" {
		t.Errorf("expected Kind=raw_output for fallback, got %q", got.Kind)
	}
	if got.Summary == "" {
		t.Error("Summary must be filled from result.Output")
	}
}

// TestSaveArtifact_FallbackWithMappedAgent — проверяет, что маппинг fallbackKind
// работает для известных агентов без привязки к конкретному taskID.
func TestSaveArtifact_FallbackWithMappedAgent(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{
		artifactRepo: repo,
		logger:       discardLogger(),
	}
	agentRec := &models.Agent{Name: "developer"}
	result := &agent.ExecutionResult{
		Success: true,
		Output:  "Я сделал то-то и то-то, без JSON-обёртки, прости.",
	}
	if err := w.saveArtifact(context.Background(), uuid.New(), agentRec, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(repo.created))
	}
	got := repo.created[0]
	if got.Kind != models.ArtifactKindCodeDiff {
		t.Errorf("expected Kind=code_diff for fallback developer, got %q", got.Kind)
	}
}

// TestSaveArtifact_SupersedePreviousReview — при сохранении review с parent_artifact_id
// должна быть вызвана SupersedePrevious на repo (старые ревью того же артефакта → superseded).
func TestSaveArtifact_SupersedePreviousReview(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{
		artifactRepo: repo,
		logger:       discardLogger(),
	}
	parentID := uuid.New()
	taskID := uuid.New()
	envelope := AgentResponseEnvelope{
		Kind:             "review",
		Summary:          "approved",
		ParentArtifactID: &parentID,
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}

	if err := w.saveArtifact(context.Background(), taskID, &models.Agent{Name: "reviewer"}, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.superseded) != 1 {
		t.Fatalf("expected SupersedePrevious to be called once, got %d", len(repo.superseded))
	}
	call := repo.superseded[0]
	if call.taskID != taskID {
		t.Errorf("supersede task_id mismatch")
	}
	if call.parentID == nil || *call.parentID != parentID {
		t.Errorf("supersede parent_id mismatch")
	}
	if call.kind != models.ArtifactKindReview {
		t.Errorf("supersede kind = %q, want review", call.kind)
	}
}

// TestSaveArtifact_NoSupersedeForPlan — supersede логика срабатывает ТОЛЬКО для review.
// Plan / code_diff / etc. — не вызывают supersede (новая итерация — отдельный артефакт).
func TestSaveArtifact_NoSupersedeForPlan(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	parentID := uuid.New()
	envelope := AgentResponseEnvelope{
		Kind:             "plan",
		Summary:          "v2 plan",
		ParentArtifactID: &parentID,
	}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "planner"}, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	if len(repo.superseded) != 0 {
		t.Errorf("supersede MUST NOT be called for kind=plan, got %d calls", len(repo.superseded))
	}
}

// TestSaveArtifact_LongSummaryTruncated — оверсайз summary режется до 500 рун.
func TestSaveArtifact_LongSummaryTruncated(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	// 600 кириллических символов (1200+ байт)
	long := strings.Repeat("а", 600)
	envelope := AgentResponseEnvelope{Kind: "plan", Summary: long}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "planner"}, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	got := repo.created[0]
	if !models.ValidateArtifactSummary(got.Summary) {
		t.Errorf("saved summary must satisfy ValidateArtifactSummary; got %d runes", len([]rune(got.Summary)))
	}
}

// TestSaveArtifact_EmptyContentBecomesEmptyJSON — content nil/пустой нормализуется в "{}"
// (БД CHECK не позволит пустой jsonb).
func TestSaveArtifact_EmptyContentBecomesEmptyJSON(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	envelope := AgentResponseEnvelope{Kind: "plan", Summary: "no content"}
	envBytes, _ := json.Marshal(envelope)
	result := &agent.ExecutionResult{Success: true, Output: string(envBytes)}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "planner"}, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}
	got := repo.created[0]
	if string(got.Content) != "{}" {
		t.Errorf("expected empty content normalized to {}, got %q", string(got.Content))
	}
	// Verify it's valid JSON.
	var _ datatypes.JSON = got.Content
	if !json.Valid(got.Content) {
		t.Error("content must be valid JSON")
	}
}

// TestAllocateWorktreeForJob_RejectsMissingBaseBranch — если в payload нет _base_branch,
// allocator возвращает ошибку (caller fail'ит event).
func TestAllocateWorktreeForJob_RejectsMissingBaseBranch(t *testing.T) {
	w := &AgentWorker{worktreeMgr: nil, logger: discardLogger()}
	ev := &models.TaskEvent{ID: 1, TaskID: uuid.New()}
	payload := &models.AgentJobPayload{AgentName: "developer", Input: map[string]any{}}
	_, err := w.allocateWorktreeForJob(context.Background(), ev, payload)
	if err == nil {
		t.Error("expected error when _base_branch missing")
	}
	if !strings.Contains(err.Error(), "_base_branch") {
		t.Errorf("error must reference _base_branch, got: %v", err)
	}
}

// TestAllocateWorktreeForJob_RejectsNonStringBaseBranch — _base_branch не строка.
func TestAllocateWorktreeForJob_RejectsNonStringBaseBranch(t *testing.T) {
	w := &AgentWorker{worktreeMgr: nil, logger: discardLogger()}
	ev := &models.TaskEvent{ID: 1, TaskID: uuid.New()}
	payload := &models.AgentJobPayload{Input: map[string]any{"_base_branch": 12345}}
	_, err := w.allocateWorktreeForJob(context.Background(), ev, payload)
	if err == nil {
		t.Error("expected error when _base_branch is not a string")
	}
}

// TestSaveArtifact_LeakCanaryNotLogged — security integration test:
// агент возвращает сломанный envelope с canary; raw output попадает в логгер только
// через SafeRawAttr; canary не должен появиться в выходе логов.
func TestSaveArtifact_LeakCanaryNotLogged(t *testing.T) {
	repo := newMemArtifactRepo()
	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	w := &AgentWorker{artifactRepo: repo, logger: logger}

	canary := "AGENT_OUTPUT_CANARY_xyz_no_envelope"
	// Output не-JSON, чтобы попасть в fallback path с SafeRawAttr-логом.
	result := &agent.ExecutionResult{Success: true, Output: "Plain text: " + canary}
	if err := w.saveArtifact(context.Background(), uuid.New(), &models.Agent{Name: "developer"}, result, nil); err != nil {
		t.Fatalf("saveArtifact: %v", err)
	}

	logged := buf.String()
	if strings.Contains(logged, canary) {
		t.Fatalf("CANARY LEAKED in logs: %s", logged)
	}
	// Должно быть упоминание о fallback (для observability):
	if !strings.Contains(logged, "raw_output fallback") {
		t.Errorf("expected fallback log entry, got: %s", logged)
	}
}

func TestExtractMultipleArtifacts(t *testing.T) {
	taskID := uuid.New()
	parentID := uuid.New()

	t.Run("ValidArrayOfArtifacts", func(t *testing.T) {
		output := fmt.Sprintf(`[
			{"kind": "subtask_description", "summary": "First", "parent": "%s"},
			{"kind": "subtask_description", "summary": "Second", "parent": "%s"}
		]`, parentID, parentID)

		result := &agent.ExecutionResult{Output: output}
		arts, ok := extractMultipleArtifacts(result, "developer", taskID, nil)
		if !ok {
			t.Fatal("expected true")
		}
		if len(arts) != 2 {
			t.Fatalf("expected 2 artifacts, got %d", len(arts))
		}
		if arts[0].Kind != "subtask_description" || arts[0].Summary != "First" {
			t.Errorf("incorrect art[0]: %+v", arts[0])
		}
		if arts[1].Kind != "subtask_description" || arts[1].Summary != "Second" {
			t.Errorf("incorrect art[1]: %+v", arts[1])
		}
		if arts[0].ParentID == nil || *arts[0].ParentID != parentID {
			t.Errorf("expected parent ID %s, got %v", parentID, arts[0].ParentID)
		}
	})

	t.Run("ValidObjectWithArtifactsField", func(t *testing.T) {
		output := fmt.Sprintf(`{
			"artifacts": [
				{"kind": "subtask_description", "title": "Subtask Title", "parent": "%s"}
			]
		}`, parentID)

		result := &agent.ExecutionResult{Output: output}
		arts, ok := extractMultipleArtifacts(result, "developer", taskID, nil)
		if !ok {
			t.Fatal("expected true")
		}
		if len(arts) != 1 {
			t.Fatalf("expected 1 artifact, got %d", len(arts))
		}
		if arts[0].Kind != "subtask_description" || arts[0].Summary != "Subtask Title" {
			t.Errorf("incorrect art[0]: %+v", arts[0])
		}
	})

	t.Run("FallbackOnSingleEnvelope", func(t *testing.T) {
		output := `{"kind": "review", "summary": "approved"}`
		result := &agent.ExecutionResult{Output: output}
		_, ok := extractMultipleArtifacts(result, "developer", taskID, nil)
		if ok {
			t.Fatal("expected false for single envelope")
		}
	})

	t.Run("FallbackOnPlainText", func(t *testing.T) {
		output := `Not a JSON at all`
		result := &agent.ExecutionResult{Output: output}
		_, ok := extractMultipleArtifacts(result, "developer", taskID, nil)
		if ok {
			t.Fatal("expected false for plain text")
		}
	})
}

// Unused import guard:
var _ = io.Discard
var _ error
var _ = errors.New

// TestAgentResultUnusable — пустой/неуспешный вывод агента считается непригодным и
// должен уходить в failEvent, а не сохраняться как ready-артефакт (фикс задачи 1.1).
func TestAgentResultUnusable(t *testing.T) {
	cases := []struct {
		name   string
		result *agent.ExecutionResult
		want   bool
	}{
		{"nil result", nil, true},
		{"success false (OOM/crash)", &agent.ExecutionResult{Success: false, Output: "boom"}, true},
		{"success but empty output, no artifacts", &agent.ExecutionResult{Success: true, Output: "   "}, true},
		{"success with output", &agent.ExecutionResult{Success: true, Output: `{"kind":"code_diff"}`}, false},
		{"success empty output but has artifacts json", &agent.ExecutionResult{Success: true, Output: "", ArtifactsJSON: []byte(`{"diff":"x"}`)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := agentResultUnusable(tc.result); got != tc.want {
				t.Errorf("agentResultUnusable() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExtractSubtasks — обе формы content декомпозиции + пустая.
func TestExtractSubtasks(t *testing.T) {
	flat := datatypes.JSON(`{"subtasks":[{"title":"a"},{"title":"b"}]}`)
	if got := extractSubtasks(flat); len(got) != 2 {
		t.Errorf("flat: expected 2 subtasks, got %d", len(got))
	}
	nested := datatypes.JSON(`{"content":{"subtasks":[{"title":"a"}]}}`)
	if got := extractSubtasks(nested); len(got) != 1 {
		t.Errorf("nested: expected 1 subtask, got %d", len(got))
	}
	if got := extractSubtasks(datatypes.JSON(`{"foo":"bar"}`)); got != nil {
		t.Errorf("no subtasks: expected nil, got %v", got)
	}
}

// TestSplitDecomposition_CreatesAndIdempotent — split создаёт subtask_description с parent=
// декомпозиция, повторный вызов ничего не создаёт (защита от дублей bypass + first-pass).
func TestSplitDecomposition_CreatesAndIdempotent(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	taskID := uuid.New()
	decompID := uuid.New()
	content := datatypes.JSON(`{"subtasks":[{"title":"Domain layer"},{"title":"Repository"},{"title":"HTTP handlers"}]}`)

	n, err := w.splitDecomposition(context.Background(), nil, taskID, decompID, content)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 subtasks created, got %d", n)
	}
	// все привязаны к декомпозиции и имеют правильный kind
	subs := 0
	for _, a := range repo.created {
		if a.Kind == models.ArtifactKindSubtaskDescription {
			subs++
			if a.ParentID == nil || *a.ParentID != decompID {
				t.Errorf("subtask %q parent != decomposition", a.Summary)
			}
		}
	}
	if subs != 3 {
		t.Errorf("expected 3 subtask_description artifacts, got %d", subs)
	}

	// Идемпотентность: второй вызов не создаёт дублей.
	n2, err := w.splitDecomposition(context.Background(), nil, taskID, decompID, content)
	if err != nil {
		t.Fatalf("split #2: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 on idempotent re-split, got %d", n2)
	}
}

// multiRepoProject — проект с primary `main` и `self-service` для мульти-репо тестов сплита.
func multiRepoProject() *models.Project {
	return &models.Project{
		Repositories: []models.ProjectRepository{
			{Slug: "main", IsPrimary: true},
			{Slug: "self-service"},
		},
	}
}

func repoSlugOfArtifact(t *testing.T, a models.Artifact) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.Content, &m); err != nil {
		t.Fatalf("unmarshal subtask content: %v", err)
	}
	s, _ := m["repo_slug"].(string)
	return s
}

// TestSplitDecomposition_MultiRepo_KeepsAndInfers — в мульти-репо проекте явный валидный
// repo_slug сохраняется, а пропущенный выводится из текста подзадачи и проставляется в content.
func TestSplitDecomposition_MultiRepo_KeepsAndInfers(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	content := datatypes.JSON(`{"subtasks":[
		{"title":"explicit","description":"x","repo_slug":"self-service"},
		{"title":"Repo: self-service router","description":"Правка в self-service routers.py"}
	]}`)

	n, err := w.splitDecomposition(context.Background(), multiRepoProject(), uuid.New(), uuid.New(), content)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 created, got %d", n)
	}
	for _, a := range repo.created {
		if a.Kind != models.ArtifactKindSubtaskDescription {
			continue
		}
		if got := repoSlugOfArtifact(t, a); got != "self-service" {
			t.Errorf("subtask %q: repo_slug = %q, want self-service", a.Summary, got)
		}
	}
}

// TestSplitDecomposition_MultiRepo_RejectsUnresolvable — подзадача без repo_slug и без
// упоминания известного slug в тексте отвергает всю декомпозицию (ничего не создаётся).
func TestSplitDecomposition_MultiRepo_RejectsUnresolvable(t *testing.T) {
	repo := newMemArtifactRepo()
	w := &AgentWorker{artifactRepo: repo, logger: discardLogger()}
	content := datatypes.JSON(`{"subtasks":[
		{"title":"ok","description":"work in self-service"},
		{"title":"ambiguous","description":"generic change with no repo hint"}
	]}`)

	n, err := w.splitDecomposition(context.Background(), multiRepoProject(), uuid.New(), uuid.New(), content)
	if !errors.Is(err, ErrDecompositionRepoSlugMissing) {
		t.Fatalf("err = %v, want ErrDecompositionRepoSlugMissing", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 created on reject, got %d", n)
	}
	for _, a := range repo.created {
		if a.Kind == models.ArtifactKindSubtaskDescription {
			t.Fatalf("no subtask_description must be created on reject, got %q", a.Summary)
		}
	}
}

// TestInferRepoSlugFromText — единственный токен выводится, неоднозначность/подстрока — нет.
func TestInferRepoSlugFromText(t *testing.T) {
	slugs := []string{"main", "self-service", "mcp-servers"}
	cases := []struct {
		name string
		text string
		want string
	}{
		{"explicit prose", "Репозиторий: self-service. Backend: FastAPI.", "self-service"},
		{"none", "just some generic description", ""},
		{"ambiguous", "touch self-service and mcp-servers", ""},
		{"substring not matched", "this is the domain logic", ""}, // 'main' внутри 'domain' не считается
		{"whole token main", "work in main repo", "main"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferRepoSlugFromText(c.text, slugs); got != c.want {
				t.Errorf("inferRepoSlugFromText(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}

func TestSandboxRealDiff(t *testing.T) {
	realDiff := "diff --git a/x.go b/x.go\n@@ -1,1 +1,2 @@\n a\n+b\n"
	cases := []struct {
		name   string
		result *agent.ExecutionResult
		want   string
	}{
		{"nil result", nil, ""},
		{"no artifacts json", &agent.ExecutionResult{}, ""},
		// sandbox executor: ArtifactsJSON = {"diff","commit_hash","branch_name"} без kind → берём реальный diff
		{"sandbox shape", &agent.ExecutionResult{ArtifactsJSON: []byte(`{"diff":"` + "diff --git a/x b/x\\n" + `","branch_name":"b","commit_hash":"abc"}`)}, "diff --git a/x b/x\n"},
		{"sandbox full diff", &agent.ExecutionResult{ArtifactsJSON: mustJSON(map[string]any{"diff": realDiff, "branch_name": "b"})}, realDiff},
		// LLM executor: ArtifactsJSON — это envelope с kind → НЕ трогаем (вернуть пусто)
		{"llm envelope has kind", &agent.ExecutionResult{ArtifactsJSON: mustJSON(map[string]any{"kind": "code_diff", "diff": "ignored", "content": map[string]any{"diff": "x"}})}, ""},
		{"empty diff", &agent.ExecutionResult{ArtifactsJSON: mustJSON(map[string]any{"branch_name": "b"})}, ""},
		{"invalid json", &agent.ExecutionResult{ArtifactsJSON: []byte(`not-json`)}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sandboxRealDiff(c.result); got != c.want {
				t.Fatalf("sandboxRealDiff = %q, want %q", got, c.want)
			}
		})
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
