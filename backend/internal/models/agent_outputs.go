package models

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// agent_outputs.go — Sprint 17 / Sprint 4 — типобезопасные читалки `artifact.content`
// для kind=merged_code и kind=test_result.
//
// Контракт content для каждого kind задан в seed/refined system_prompts агентов
// (migrations 038 + 040). Несоответствие → ошибка с указанием на конкретное поле,
// чтобы оператор мог поправить промпт.

// ─────────────────────────────────────────────────────────────────────────────
// Merger output
// ─────────────────────────────────────────────────────────────────────────────

// MergerOutput — содержимое artifact.content для kind='merged_code'.
type MergerOutput struct {
	// MergedBranch — имя ветки, в которую слились changes (обычно task-<uuid>-merged).
	MergedBranch string `json:"merged_branch"`
	// SourceWorktreeIDs — список worktree-ID, чьи ветки участвовали в merge.
	SourceWorktreeIDs []uuid.UUID `json:"source_worktree_ids"`
	// MergeConflictsResolved — описания разрешённых конфликтов; пусто если merge был fast-forward.
	MergeConflictsResolved []MergeConflictResolution `json:"merge_conflicts_resolved,omitempty"`
	// ChecksRun / ChecksPassed — какие проверки прогнаны и результат.
	ChecksRun    []string `json:"checks_run,omitempty"`
	ChecksPassed bool     `json:"checks_passed"`
	// HeadCommitSHA — SHA итогового коммита на MergedBranch.
	HeadCommitSHA string `json:"head_commit_sha,omitempty"`
}

// MergeConflictResolution — один разрешённый конфликт.
type MergeConflictResolution struct {
	File       string `json:"file"`
	Resolution string `json:"resolution"`
}

// ParseMergerOutput читает artifact.content в MergerOutput с валидацией обязательных полей.
func ParseMergerOutput(content []byte) (*MergerOutput, error) {
	var out MergerOutput
	if err := json.Unmarshal(content, &out); err != nil {
		return nil, fmt.Errorf("parse merger output: %w", err)
	}
	if out.MergedBranch == "" {
		return nil, fmt.Errorf("merger output: merged_branch is required")
	}
	if len(out.SourceWorktreeIDs) == 0 {
		return nil, fmt.Errorf("merger output: source_worktree_ids must be non-empty")
	}
	return &out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Tester output
// ─────────────────────────────────────────────────────────────────────────────

// TestResult — содержимое artifact.content для kind='test_result'.
type TestResult struct {
	Passed          int           `json:"passed"`
	Failed          int           `json:"failed"`
	Skipped         int           `json:"skipped"`
	DurationMS      int64         `json:"duration_ms"`
	CoveragePercent *float64      `json:"coverage_percent,omitempty"`
	BuildPassed     bool          `json:"build_passed"`
	LintPassed      bool          `json:"lint_passed"`
	TypecheckPassed bool          `json:"typecheck_passed"`
	Failures        []TestFailure `json:"failures,omitempty"`
	// RawOutputTruncated — первые ~4КБ stdout/stderr. Может содержать имена файлов
	// или stack traces; чувствительных данных типа секретов быть не должно (если
	// агент следует промпту), но redact-фильтрация всё равно применяется в логе.
	RawOutputTruncated string `json:"raw_output_truncated,omitempty"`
}

// TestFailure — одна упавшая проверка.
type TestFailure struct {
	TestName   string `json:"test_name"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message"`
	StackTrace string `json:"stack_trace,omitempty"`
}

// AllPassed — true, если ни один тест/чек не упал. Удобно для Router'а — он смотрит
// summary, но code на go-стороне может проверить структурный исход.
func (r *TestResult) AllPassed() bool {
	return r.Failed == 0 && r.BuildPassed && r.LintPassed && r.TypecheckPassed
}

// requiredTestResultBoolFields — поля, обязательное присутствие которых проверяется
// через предварительный map-парсинг. encoding/json не отличает "false" от "missing"
// для bool, поэтому без этой проверки забытый ключ молча превращается в "что-то упало".
var requiredTestResultBoolFields = []string{"build_passed", "lint_passed", "typecheck_passed"}

// ParseTestResult читает artifact.content в TestResult со строгой валидацией.
//
// Контракт (синхронизирован с system_prompt в миграции 040):
//   - Поля build_passed/lint_passed/typecheck_passed ОБЯЗАНЫ присутствовать в JSON.
//     Отсутствующий ключ → ошибка (а не молчаливое false), чтобы оператор увидел
//     несоответствие промпта и не подумал, что "всё упало".
//   - passed/failed/skipped/duration_ms — могут отсутствовать (default 0); если
//     указаны — должны быть неотрицательными.
//   - failed > 0 ОБЯЗАТЕЛЬНО подразумевает непустой failures[] (иначе агент не
//     детализировал, что считается контрактным нарушением).
func ParseTestResult(content []byte) (*TestResult, error) {
	// Первый проход — map[string]json.RawMessage для проверки наличия обязательных ключей.
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawMap); err != nil {
		return nil, fmt.Errorf("parse test result: %w", err)
	}
	for _, key := range requiredTestResultBoolFields {
		if _, ok := rawMap[key]; !ok {
			return nil, fmt.Errorf("test result: required field %q missing (must be explicitly true|false)", key)
		}
	}

	// Второй проход — типизированный.
	var out TestResult
	if err := json.Unmarshal(content, &out); err != nil {
		return nil, fmt.Errorf("parse test result: %w", err)
	}
	if out.Passed < 0 || out.Failed < 0 || out.Skipped < 0 {
		return nil, fmt.Errorf("test result: passed/failed/skipped must be non-negative")
	}
	if out.DurationMS < 0 {
		return nil, fmt.Errorf("test result: duration_ms must be non-negative")
	}
	if out.Failed > 0 && len(out.Failures) == 0 {
		return nil, fmt.Errorf("test result: failed=%d but failures[] is empty (agent must detail each failure)", out.Failed)
	}
	return &out, nil
}
