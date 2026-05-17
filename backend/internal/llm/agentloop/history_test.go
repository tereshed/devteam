package agentloop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devteam/backend/pkg/llm"
)

// history_test.go — Sprint 21 §3.4 покрытие.
//
// Две защиты:
//   1. Per-tool truncation: tool_result > MaxToolResultBytes → preview-маркер
//      с truncated:true и hint про пагинацию.
//   2. Sliding-window compaction: суммарная history > MaxHistoryBytes →
//      старые tool_result сжимаются до summary; хвост (HistoryTailKeep)
//      остаётся как есть.
//
// Тесты на хелперы (truncateToolResultForHistory, slidingWindowCompact) +
// интеграционный тест через Executor.Run, чтобы убедиться, что итоговый
// llm.Request получает уже урезанный content.

// ─────────────────────────────────────────────────────────────────────────────
// truncateToolResultForHistory.
// ─────────────────────────────────────────────────────────────────────────────

func TestTruncateToolResult_FitsUnderLimit_Untouched(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"items":[1,2,3]}`)
	out := truncateToolResultForHistory(raw, "project_list", 1024)
	if string(out) != string(raw) {
		t.Fatalf("expected untouched, got %s", out)
	}
}

func TestTruncateToolResult_MaxZero_NoLimit(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(strings.Repeat("a", 100_000))
	out := truncateToolResultForHistory(raw, "tool_x", 0)
	if len(out) != len(raw) {
		t.Fatalf("expected pass-through when max<=0, got len=%d", len(out))
	}
}

func TestTruncateToolResult_OverLimit_ReturnsMarker(t *testing.T) {
	t.Parallel()
	big := json.RawMessage(`{"data":"` + strings.Repeat("x", 50_000) + `"}`)
	out := truncateToolResultForHistory(big, "task_list", 16*1024)

	if len(out) > 16*1024 {
		// Маркер сам по себе не должен превышать max существенно — у нас
		// preview = max*3/4 + обёртка ~150 байт. Но мы не гарантируем строгий
		// предел, просто хотим убедиться, что результат ощутимо меньше big.
		if len(out) >= len(big) {
			t.Fatalf("truncation did not reduce size: out=%d, in=%d", len(out), len(big))
		}
	}

	var marker struct {
		Status    string `json:"status"`
		Truncated bool   `json:"truncated"`
		Tool      string `json:"tool"`
		Size      int    `json:"original_size_bytes"`
		Preview   string `json:"preview"`
		Hint      string `json:"hint"`
	}
	if err := json.Unmarshal(out, &marker); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if !marker.Truncated {
		t.Fatalf("expected truncated=true, marker=%+v", marker)
	}
	if marker.Tool != "task_list" {
		t.Fatalf("expected tool=task_list, got %q", marker.Tool)
	}
	if marker.Size != len(big) {
		t.Fatalf("expected size=%d, got %d", len(big), marker.Size)
	}
	if marker.Preview == "" {
		t.Fatalf("preview must not be empty")
	}
	if !strings.Contains(marker.Hint, "пагинацию") {
		t.Fatalf("hint must mention pagination, got %q", marker.Hint)
	}
}

func TestTruncateToolResult_EmptyInput_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	out := truncateToolResultForHistory(nil, "x", 16)
	if len(out) != 0 {
		t.Fatalf("expected empty, got %s", out)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildLLMMessages — convert + truncate.
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildLLMMessages_ConvertsRoles(t *testing.T) {
	t.Parallel()
	hist := []Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "doing", ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "x", Arguments: `{}`},
		}}},
		{Role: llm.RoleTool, ToolCallID: "tc-1", ToolName: "x",
			ToolResult: json.RawMessage(`{"status":"ok"}`)},
	}
	out := buildLLMMessages(hist, Config{MaxToolResultBytes: 1024})
	if len(out) != 4 {
		t.Fatalf("expected 4 llm.Message, got %d", len(out))
	}
	if out[0].Role != llm.RoleSystem || out[1].Role != llm.RoleUser {
		t.Fatalf("role mismatch: %s/%s", out[0].Role, out[1].Role)
	}
	if out[2].Role != llm.RoleAssistant || len(out[2].ToolCalls) != 1 {
		t.Fatalf("assistant tool_calls lost: %+v", out[2])
	}
	if out[3].Role != llm.RoleTool || out[3].ToolCallID != "tc-1" || out[3].Name != "x" {
		t.Fatalf("tool message malformed: %+v", out[3])
	}
}

func TestBuildLLMMessages_TruncatesLargeToolResult(t *testing.T) {
	t.Parallel()
	big := json.RawMessage(`{"data":"` + strings.Repeat("y", 50_000) + `"}`)
	hist := []Message{
		{Role: llm.RoleUser, Content: "show me"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "task_list", Arguments: `{}`},
		}}},
		{Role: llm.RoleTool, ToolCallID: "tc-1", ToolName: "task_list", ToolResult: big},
	}
	out := buildLLMMessages(hist, Config{MaxToolResultBytes: 16 * 1024})
	toolMsg := out[2]
	if toolMsg.Role != llm.RoleTool {
		t.Fatalf("expected tool message at idx 2, got %s", toolMsg.Role)
	}
	// Content (=truncated payload) должен быть существенно меньше big.
	if len(toolMsg.Content) >= len(big) {
		t.Fatalf("tool content was not truncated: len=%d, big=%d",
			len(toolMsg.Content), len(big))
	}
	if !strings.Contains(toolMsg.Content, `"truncated":true`) {
		t.Fatalf("expected truncated marker in tool content: %s", toolMsg.Content[:min(200, len(toolMsg.Content))])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// slidingWindowCompact.
// ─────────────────────────────────────────────────────────────────────────────

func TestSlidingWindow_UnderBudget_NoOp(t *testing.T) {
	t.Parallel()
	hist := []Message{
		{Role: llm.RoleUser, Content: "short"},
		{Role: llm.RoleAssistant, Content: "ok"},
	}
	out := slidingWindowCompact(hist, Config{MaxHistoryBytes: 1024})
	if len(out) != len(hist) {
		t.Fatalf("under-budget history must be untouched: in=%d out=%d", len(hist), len(out))
	}
}

func TestSlidingWindow_ZeroBudget_NoOp(t *testing.T) {
	t.Parallel()
	hist := []Message{
		{Role: llm.RoleTool, ToolCallID: "tc-1", ToolName: "x",
			ToolResult: json.RawMessage(strings.Repeat("a", 5_000))},
	}
	out := slidingWindowCompact(hist, Config{MaxHistoryBytes: 0})
	if len(out) != 1 {
		t.Fatalf("MaxHistoryBytes<=0 must skip compaction")
	}
	if string(out[0].ToolResult) != string(hist[0].ToolResult) {
		t.Fatalf("tool_result must remain raw under no-budget mode")
	}
}

func TestSlidingWindow_OverBudget_CompactsOldToolResults(t *testing.T) {
	t.Parallel()
	// Старая тула (idx 1) должна быть сжата; новая (idx 5) — нет (в хвосте).
	bigOld := json.RawMessage(`{"items":"` + strings.Repeat("o", 8_000) + `"}`)
	bigNew := json.RawMessage(`{"items":"` + strings.Repeat("n", 8_000) + `"}`)
	hist := []Message{
		{Role: llm.RoleUser, Content: "first ask"},
		{Role: llm.RoleTool, ToolCallID: "tc-old", ToolName: "task_list", ToolResult: bigOld},
		{Role: llm.RoleAssistant, Content: "answer 1"},
		{Role: llm.RoleUser, Content: "second ask"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "tc-new", Type: "function",
			Function: llm.Function{Name: "project_list", Arguments: `{}`},
		}}},
		{Role: llm.RoleTool, ToolCallID: "tc-new", ToolName: "project_list", ToolResult: bigNew},
	}
	cfg := Config{
		MaxHistoryBytes:    4_000, // заведомо меньше суммы
		HistoryTailKeep:    2,     // оставляем последние 2 user/assistant
		MaxToolResultBytes: 0,     // отключаем per-tool truncation, чтобы изолировать sliding
	}
	out := slidingWindowCompact(hist, cfg)
	if len(out) != len(hist) {
		t.Fatalf("compact must preserve length: in=%d out=%d", len(hist), len(out))
	}
	// Старый tool-row → compacted summary, исходный payload — пропал.
	if strings.Contains(string(out[1].ToolResult), "oooo") {
		t.Fatalf("old tool_result must be compacted, got: %s", out[1].ToolResult)
	}
	if !strings.Contains(string(out[1].ToolResult), `"compacted":true`) {
		t.Fatalf("expected compacted marker on old tool, got: %s", out[1].ToolResult)
	}
	// Новый tool-row в хвосте → НЕ сжат.
	if !strings.Contains(string(out[5].ToolResult), "nnnn") {
		t.Fatalf("new tool_result (tail) must remain raw, got: %s", out[5].ToolResult[:min(200, len(out[5].ToolResult))])
	}
}

func TestSlidingWindow_KeepUserMessagesIntact(t *testing.T) {
	t.Parallel()
	// User-сообщения короткие и команды — даже в префиксе оставляем как есть
	// (compactToolMessage / compactAssistantWithCalls трогают только tool/assistant).
	hist := []Message{
		{Role: llm.RoleUser, Content: "первый запрос"},
		{Role: llm.RoleTool, ToolCallID: "tc-1", ToolName: "x",
			ToolResult: json.RawMessage(strings.Repeat("a", 5_000))},
		{Role: llm.RoleUser, Content: "второй"},
		{Role: llm.RoleAssistant, Content: "ответ"},
	}
	out := slidingWindowCompact(hist, Config{MaxHistoryBytes: 2_000, HistoryTailKeep: 1})
	if out[0].Content != "первый запрос" {
		t.Fatalf("old user message must remain intact, got %q", out[0].Content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Интеграция через Executor: убеждаемся, что LLM получает уже-усечённый payload.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_HistoryTruncation_AppliedBeforeLLMCall(t *testing.T) {
	t.Parallel()
	// LLM возвращает финал сразу. Заранее формируем «большую историю».
	big := json.RawMessage(`{"data":"` + strings.Repeat("z", 50_000) + `"}`)

	var capturedMessages []llm.Message
	client := &fakeLLM{
		responses: []*llm.Response{{Content: "ок"}},
		chatHook: func(ctx context.Context, req llm.Request) {
			capturedMessages = append([]llm.Message(nil), req.Messages...)
		},
	}

	hist := []Message{
		{Role: llm.RoleUser, Content: "что в task_list?"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "task_list", Arguments: `{}`},
		}}},
		{Role: llm.RoleTool, ToolCallID: "tc-1", ToolName: "task_list", ToolResult: big},
	}
	exec := NewExecutor(Config{
		MaxIterations:      3,
		MaxToolResultBytes: 16 * 1024,
	}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: hist,
		Hooks:   Hooks{OnFinalText: func(ctx context.Context, _ string) error { return nil }},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", res.Status)
	}

	// Ищем tool-row в LLM-запросе — он должен быть сильно меньше big.
	var found bool
	for _, m := range capturedMessages {
		if m.Role == llm.RoleTool && m.ToolCallID == "tc-1" {
			found = true
			if len(m.Content) >= len(big) {
				t.Fatalf("LLM получил неурезанный tool_result: len=%d", len(m.Content))
			}
			if !strings.Contains(m.Content, `"truncated":true`) {
				t.Fatalf("expected truncated marker in LLM content, got prefix=%q",
					m.Content[:min(120, len(m.Content))])
			}
			if !strings.Contains(m.Content, "пагинацию") {
				t.Fatalf("hint must be present, got prefix=%q",
					m.Content[:min(120, len(m.Content))])
			}
		}
	}
	if !found {
		t.Fatalf("tool-row не попал в LLM-сообщения; captured: %+v", capturedMessages)
	}
}

// tailStartIndex — корректно отделяет хвост (keep последних user/assistant).
func TestTailStartIndex(t *testing.T) {
	t.Parallel()
	hist := []Message{
		{Role: llm.RoleUser},
		{Role: llm.RoleAssistant},
		{Role: llm.RoleTool},
		{Role: llm.RoleUser},
		{Role: llm.RoleAssistant},
	}
	cases := []struct{ keep, want int }{
		{0, 5}, // keep=0 → весь массив = префикс
		{1, 4}, // последний (assistant) — хвост
		{2, 3}, // user+assistant — хвост
		{10, 0},
	}
	for _, c := range cases {
		if got := tailStartIndex(hist, c.keep); got != c.want {
			t.Errorf("keep=%d: got %d, want %d", c.keep, got, c.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
