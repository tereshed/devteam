package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseMergerOutput_Valid(t *testing.T) {
	wt1, wt2 := uuid.New(), uuid.New()
	raw := []byte(`{
		"merged_branch": "task-abc-merged",
		"source_worktree_ids": ["` + wt1.String() + `", "` + wt2.String() + `"],
		"merge_conflicts_resolved": [
			{"file": "internal/auth/jwt.go", "resolution": "kept feature-A token rotation"}
		],
		"checks_run": ["go build", "go vet"],
		"checks_passed": true,
		"head_commit_sha": "abc123"
	}`)
	got, err := ParseMergerOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MergedBranch != "task-abc-merged" {
		t.Errorf("MergedBranch = %q, want task-abc-merged", got.MergedBranch)
	}
	if len(got.SourceWorktreeIDs) != 2 {
		t.Errorf("expected 2 source worktrees, got %d", len(got.SourceWorktreeIDs))
	}
	if !got.ChecksPassed {
		t.Error("expected ChecksPassed=true")
	}
}

func TestParseMergerOutput_RejectsMissingBranch(t *testing.T) {
	raw := []byte(`{"source_worktree_ids": ["` + uuid.New().String() + `"], "checks_passed": true}`)
	if _, err := ParseMergerOutput(raw); err == nil {
		t.Error("expected error for missing merged_branch")
	}
}

func TestParseMergerOutput_RejectsEmptySourceWorktrees(t *testing.T) {
	raw := []byte(`{"merged_branch": "task-x-merged", "source_worktree_ids": [], "checks_passed": true}`)
	if _, err := ParseMergerOutput(raw); err == nil {
		t.Error("expected error for empty source_worktree_ids")
	}
}

func TestParseTestResult_Valid(t *testing.T) {
	raw := []byte(`{
		"passed": 12,
		"failed": 0,
		"skipped": 1,
		"duration_ms": 5430,
		"coverage_percent": 87.5,
		"build_passed": true,
		"lint_passed": true,
		"typecheck_passed": true
	}`)
	r, err := ParseTestResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Passed != 12 || r.Skipped != 1 {
		t.Errorf("counts mismatch: %+v", r)
	}
	if !r.AllPassed() {
		t.Error("expected AllPassed=true for 12/0/1 with all checks passed")
	}
	if r.CoveragePercent == nil || *r.CoveragePercent != 87.5 {
		t.Errorf("coverage_percent expected 87.5, got %v", r.CoveragePercent)
	}
}

func TestParseTestResult_AllPassedFalseOnFailures(t *testing.T) {
	raw := []byte(`{
		"passed": 5,
		"failed": 1,
		"build_passed": true,
		"lint_passed": true,
		"typecheck_passed": true,
		"failures": [{"test_name": "TestFoo", "message": "expected X got Y"}]
	}`)
	r, err := ParseTestResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.AllPassed() {
		t.Error("expected AllPassed=false when Failed > 0")
	}
}

func TestParseTestResult_AllPassedFalseOnBrokenBuild(t *testing.T) {
	raw := []byte(`{
		"passed": 0, "failed": 0,
		"build_passed": false, "lint_passed": true, "typecheck_passed": true
	}`)
	r, err := ParseTestResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.AllPassed() {
		t.Error("expected AllPassed=false when build_passed=false")
	}
}

func TestParseTestResult_RejectsNegativeCounts(t *testing.T) {
	raw := []byte(`{"passed": -1, "build_passed": true, "lint_passed": true, "typecheck_passed": true}`)
	if _, err := ParseTestResult(raw); err == nil {
		t.Error("expected error for negative passed count")
	}
}

// TestParseTestResult_RejectsMissingBuildPassed — Sprint 4 review fix:
// encoding/json молча превращает отсутствующий bool в false. Строгая проверка
// через map должна выдать конкретную ошибку про "missing field".
func TestParseTestResult_RejectsMissingBuildPassed(t *testing.T) {
	for _, missingField := range []string{"build_passed", "lint_passed", "typecheck_passed"} {
		t.Run(missingField, func(t *testing.T) {
			// Все три поля присутствуют, кроме missingField.
			fields := map[string]any{
				"passed":           5,
				"failed":           0,
				"build_passed":     true,
				"lint_passed":      true,
				"typecheck_passed": true,
			}
			delete(fields, missingField)
			raw, _ := json.Marshal(fields)
			_, err := ParseTestResult(raw)
			if err == nil {
				t.Fatalf("expected error when %q is missing, got nil", missingField)
			}
			if !strings.Contains(err.Error(), missingField) {
				t.Errorf("error must reference missing field name %q, got: %v", missingField, err)
			}
		})
	}
}

// TestParseTestResult_RejectsFailedWithoutFailures — failed > 0 без failures[] —
// контрактное нарушение (агент не детализировал падения).
func TestParseTestResult_RejectsFailedWithoutFailures(t *testing.T) {
	raw := []byte(`{
		"passed": 5, "failed": 2,
		"build_passed": true, "lint_passed": true, "typecheck_passed": true
	}`)
	if _, err := ParseTestResult(raw); err == nil {
		t.Error("expected error when failed>0 but failures[] empty")
	}
}

// ─── Схема 082 (acceptance-driven) ────────────────────────────────────────────

// TestParseTestResult_Schema082Valid — regression на задачу fc6b2b05/DEV-482:
// актуальный промпт tester'а (миграция 082) отдаёт decision/acceptance/tests/issues
// БЕЗ легаси build_passed. Раньше fail-loud-гард валил такой артефакт 3× → needs_human.
func TestParseTestResult_Schema082Valid(t *testing.T) {
	// Форма из реального принятого артефакта DEV-481.
	raw := []byte(`{
		"decision": "passed",
		"acceptance": [
			{"criterion": "GET /request_events admin-only → 200", "status": "verified", "evidence": "integration test green against real PG"}
		],
		"tests": {"unit": "PASS", "integration": "PASS", "lint": "clean", "build": "go build ./... exit 0"},
		"issues": ["Non-blocking: migration lives in self-service repo"],
		"summary": "all acceptance criteria verified"
	}`)
	r, err := ParseTestResult(raw)
	if err != nil {
		t.Fatalf("unexpected error for valid 082 payload: %v", err)
	}
	if r.Decision != "passed" {
		t.Errorf("decision expected 'passed', got %q", r.Decision)
	}
	if !r.AllPassed() {
		t.Error("expected AllPassed=true when decision=passed")
	}
	if r.Tests == nil || r.Tests.Build == "" {
		t.Errorf("tests breakdown not parsed: %+v", r.Tests)
	}
	if len(r.Acceptance) != 1 || r.Acceptance[0].Status != "verified" {
		t.Errorf("acceptance not parsed: %+v", r.Acceptance)
	}
}

func TestParseTestResult_Schema082VerdictMapping(t *testing.T) {
	cases := map[string]bool{"passed": true, "failed": false, "blocked": false, "PASSED": true, " passed ": true}
	for verdict, wantAllPassed := range cases {
		t.Run(verdict, func(t *testing.T) {
			raw := []byte(`{"decision": "` + verdict + `", "summary": "x"}`)
			r, err := ParseTestResult(raw)
			if err != nil {
				t.Fatalf("unexpected error for decision %q: %v", verdict, err)
			}
			if r.AllPassed() != wantAllPassed {
				t.Errorf("decision %q: AllPassed=%v, want %v", verdict, r.AllPassed(), wantAllPassed)
			}
		})
	}
}

func TestParseTestResult_Schema082RejectsEmptyDecision(t *testing.T) {
	if _, err := ParseTestResult([]byte(`{"decision": ""}`)); err == nil {
		t.Error("expected error for empty decision")
	}
	if _, err := ParseTestResult([]byte(`{"decision": "  "}`)); err == nil {
		t.Error("expected error for whitespace-only decision")
	}
}

func TestParseTestResult_Schema082RejectsUnknownDecision(t *testing.T) {
	if _, err := ParseTestResult([]byte(`{"decision": "maybe"}`)); err == nil {
		t.Error("expected error for unknown decision value")
	}
}

// TestParseTestResult_RejectsEmptyPayload — fail-loud-гард: пустой объект (ни decision,
// ни легаси-булевых) — это тихий no-op артефакт, его обязаны отклонить.
func TestParseTestResult_RejectsEmptyPayload(t *testing.T) {
	if _, err := ParseTestResult([]byte(`{}`)); err == nil {
		t.Error("expected error for empty test_result payload")
	}
}
