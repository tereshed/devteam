package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/pkg/llm"
)

// executor_test.go — Sprint 21 §3.2 покрытие.
//
// Группы:
//   - final_text path (Completed)
//   - tool_use → tool_result → final_text
//   - MaxIterations превышен → LimitExceeded
//   - empty response без tool_calls → Failed
//   - confirm-gate: Park / Approve / Deny
//   - PARITY: park посреди батча → остаток получает synthetic skipped tool_result
//   - ctx cancel / per-call timeout
//   - unknown tool → synthetic error, петля продолжает
//   - hook ошибки → Failed
//   - валидация конфигурации каталога (duplicate / nil handler / destructive без OnConfirmRequired / nil client)
//
// Зависимости — заглушки. Никаких сетевых вызовов, никаких реальных LLM.

// fakeLLM — programmable mock llm.Client. Каждый Chat возвращает следующий
// ответ из responses; если responses исчерпались — последняя ошибка или nil.
type fakeLLM struct {
	mu        sync.Mutex
	responses []*llm.Response
	errs      []error
	calls     int
	// chatHook — позволяет тесту вставить дополнительную проверку/задержку
	// внутри вызова Chat (для timeout-тестов).
	chatHook func(ctx context.Context, req llm.Request)
}

func (f *fakeLLM) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	f.mu.Lock()
	idx := f.calls
	f.calls++
	f.mu.Unlock()

	if f.chatHook != nil {
		f.chatHook(ctx, req)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return nil, errors.New("fakeLLM: no more responses")
}

func (f *fakeLLM) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return nil, llm.ErrEmbeddingsNotSupported
}

func (f *fakeLLM) HealthCheck(ctx context.Context) error { return nil }

func (f *fakeLLM) ResolveBaseURL() string { return "test://fake" }

func (f *fakeLLM) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// ─────────────────────────────────────────────────────────────────────────────
// Final-text happy path.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_FinalText_CompletedOnFirstIteration(t *testing.T) {
	t.Parallel()

	client := &fakeLLM{
		responses: []*llm.Response{
			{Content: "Готово, проектов нет.", ToolCalls: nil},
		},
	}

	var finalText string
	var finalCalled int
	hooks := Hooks{
		OnFinalText: func(ctx context.Context, text string) error {
			finalCalled++
			finalText = text
			return nil
		},
	}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:       client,
		SystemPrompt: "you are tester",
		History:      []Message{{Role: llm.RoleUser, Content: "привет"}},
		Hooks:        hooks,
	})
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s (cause=%v)", res.Status, res.Cause)
	}
	if res.Iterations != 1 {
		t.Fatalf("expected 1 iteration, got %d", res.Iterations)
	}
	if finalCalled != 1 || finalText != "Готово, проектов нет." {
		t.Fatalf("OnFinalText not invoked correctly: called=%d text=%q", finalCalled, finalText)
	}
	if res.LastAssistantText != "Готово, проектов нет." {
		t.Fatalf("LastAssistantText mismatch: %q", res.LastAssistantText)
	}
}

// Empty response без tool_calls → Failed (LLM выдал ничего).
func TestExecutor_EmptyResponse_Failed(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{{Content: "", ToolCalls: nil}}}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if res.Cause == nil {
		t.Fatal("expected Cause != nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// tool_use path.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_ToolUse_ExecutesHandlerAndContinues(t *testing.T) {
	t.Parallel()

	client := &fakeLLM{
		responses: []*llm.Response{
			// Iteration 1: модель просит вызов project_list.
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "tc-1",
					Type: "function",
					Function: llm.Function{
						Name:      "project_list",
						Arguments: `{"limit":20}`,
					},
				}},
			},
			// Iteration 2: модель отвечает финальным текстом.
			{Content: "У вас 2 проекта."},
		},
	}

	var handlerCalls int32
	var capturedAuth AuthContext
	tools := []Tool{{
		Name: "project_list",
		Handler: func(ctx context.Context, auth AuthContext, args json.RawMessage) (json.RawMessage, error) {
			atomic.AddInt32(&handlerCalls, 1)
			capturedAuth = auth
			if !json.Valid(args) {
				t.Errorf("invalid JSON args: %s", args)
			}
			return json.RawMessage(`{"items":[{"id":"p1"},{"id":"p2"}]}`), nil
		},
	}}

	var toolCallEvents int
	var toolResultEvents int
	hooks := Hooks{
		OnToolCall: func(ctx context.Context, call ToolCall) error {
			toolCallEvents++
			if call.ID != "tc-1" || call.Name != "project_list" {
				t.Errorf("unexpected ToolCall: %+v", call)
			}
			return nil
		},
		OnToolResult: func(ctx context.Context, res ToolResult) error {
			toolResultEvents++
			if res.Status != "ok" {
				t.Errorf("expected status=ok, got %q (result=%s)", res.Status, string(res.Result))
			}
			return nil
		},
		OnFinalText: func(ctx context.Context, text string) error { return nil },
	}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "сколько проектов?"}},
		Tools:   tools,
		Auth:    AuthContext{UserID: "u-42", Scope: "assistant"},
		Hooks:   hooks,
	})
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s (cause=%v)", res.Status, res.Cause)
	}
	if res.Iterations != 2 {
		t.Fatalf("expected 2 iterations, got %d", res.Iterations)
	}
	if atomic.LoadInt32(&handlerCalls) != 1 {
		t.Fatalf("expected 1 handler invocation, got %d", handlerCalls)
	}
	if capturedAuth.UserID != "u-42" || capturedAuth.Scope != "assistant" {
		t.Fatalf("auth not propagated: %+v", capturedAuth)
	}
	if toolCallEvents != 1 || toolResultEvents != 1 {
		t.Fatalf("hooks counts: call=%d result=%d", toolCallEvents, toolResultEvents)
	}
}

// Unknown tool → synthetic error в LLM, петля продолжает.
func TestExecutor_UnknownTool_ContinuesWithSyntheticError(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{
		responses: []*llm.Response{
			{ToolCalls: []llm.ToolCall{{
				ID: "tc-x", Type: "function",
				Function: llm.Function{Name: "ghost_tool", Arguments: `{}`},
			}}},
			{Content: "не могу — нет такого инструмента"},
		},
	}

	var seenStatus string
	hooks := Hooks{
		OnToolResult: func(ctx context.Context, r ToolResult) error {
			seenStatus = r.Status
			return nil
		},
		OnFinalText: func(ctx context.Context, t string) error { return nil },
	}

	tools := []Tool{{
		Name: "project_list",
		Handler: func(ctx context.Context, auth AuthContext, args json.RawMessage) (json.RawMessage, error) {
			t.Error("handler must not be called for unknown tool")
			return nil, nil
		},
	}}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", res.Status)
	}
	if seenStatus != "error" {
		t.Fatalf("expected synthetic error status, got %q", seenStatus)
	}
}

// LLM возвращает tool_calls, а Tools пуст → Failed.
func TestExecutor_ToolCallWithEmptyCatalog_Failed(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{{
		ToolCalls: []llm.ToolCall{{ID: "x", Type: "function",
			Function: llm.Function{Name: "anything", Arguments: `{}`}}},
	}}}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LimitExceeded.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_MaxIterations_LimitExceeded(t *testing.T) {
	t.Parallel()
	// Каждая итерация модель упорно дёргает один и тот же tool.
	mkResp := func(i int) *llm.Response {
		return &llm.Response{ToolCalls: []llm.ToolCall{{
			ID: "tc-" + itoa(i), Type: "function",
			Function: llm.Function{Name: "ping", Arguments: `{}`},
		}}}
	}
	client := &fakeLLM{responses: []*llm.Response{mkResp(1), mkResp(2), mkResp(3), mkResp(4)}}
	tools := []Tool{{Name: "ping", Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"status":"ok"}`), nil
	}}}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools,
		History: []Message{{Role: llm.RoleUser, Content: "ping forever"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusLimitExceeded {
		t.Fatalf("expected LimitExceeded, got %s", res.Status)
	}
	if res.Iterations != 3 {
		t.Fatalf("expected 3 iterations, got %d", res.Iterations)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Confirm gate.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_DestructiveTool_Park(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-delete", Type: "function",
			Function: llm.Function{Name: "project_delete", Arguments: `{"id":"p1"}`},
		}}},
	}}

	var handlerCalled int32
	tools := []Tool{{
		Name: "project_delete", RequiresConfirmation: true,
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			atomic.AddInt32(&handlerCalled, 1)
			return json.RawMessage(`{"status":"ok"}`), nil
		},
	}}

	var confirmCalled int
	hooks := Hooks{
		OnConfirmRequired: func(ctx context.Context, call ToolCall) (ConfirmDecision, error) {
			confirmCalled++
			if call.Name != "project_delete" {
				t.Errorf("unexpected tool in confirm: %s", call.Name)
			}
			return ConfirmPark, nil
		},
	}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "удали проект"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusParked {
		t.Fatalf("expected Parked, got %s (cause=%v)", res.Status, res.Cause)
	}
	if res.ParkedCall == nil || res.ParkedCall.ID != "tc-delete" {
		t.Fatalf("ParkedCall malformed: %+v", res.ParkedCall)
	}
	if atomic.LoadInt32(&handlerCalled) != 0 {
		t.Fatal("handler must NOT be called when parked")
	}
	if confirmCalled != 1 {
		t.Fatalf("OnConfirmRequired called %d times, want 1", confirmCalled)
	}
}

func TestExecutor_DestructiveTool_Approve(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-del", Type: "function",
			Function: llm.Function{Name: "project_delete", Arguments: `{"id":"p1"}`},
		}}},
		{Content: "проект удалён"},
	}}

	var handlerCalled int32
	tools := []Tool{{
		Name: "project_delete", RequiresConfirmation: true,
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			atomic.AddInt32(&handlerCalled, 1)
			return json.RawMessage(`{"status":"ok"}`), nil
		},
	}}

	hooks := Hooks{
		OnConfirmRequired: func(ctx context.Context, _ ToolCall) (ConfirmDecision, error) {
			return ConfirmApprove, nil
		},
		OnFinalText: func(ctx context.Context, _ string) error { return nil },
	}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "yes"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", res.Status)
	}
	if atomic.LoadInt32(&handlerCalled) != 1 {
		t.Fatalf("handler invoked %d times, want 1", handlerCalled)
	}
}

func TestExecutor_DestructiveTool_Deny(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-del", Type: "function",
			Function: llm.Function{Name: "project_delete", Arguments: `{"id":"p1"}`},
		}}},
		{Content: "ок, не удаляю"},
	}}

	var handlerCalled int32
	tools := []Tool{{
		Name: "project_delete", RequiresConfirmation: true,
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			atomic.AddInt32(&handlerCalled, 1)
			return json.RawMessage(`{"status":"ok"}`), nil
		},
	}}

	var deniedSeen bool
	hooks := Hooks{
		OnConfirmRequired: func(ctx context.Context, _ ToolCall) (ConfirmDecision, error) {
			return ConfirmDeny, nil
		},
		OnToolResult: func(ctx context.Context, r ToolResult) error {
			if r.Status == "denied" {
				deniedSeen = true
			}
			return nil
		},
		OnFinalText: func(ctx context.Context, _ string) error { return nil },
	}

	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "delete"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", res.Status)
	}
	if atomic.LoadInt32(&handlerCalled) != 0 {
		t.Fatal("handler must NOT be called on Deny")
	}
	if !deniedSeen {
		t.Fatal("OnToolResult must observe denied tool_result")
	}
}

// PARITY: park посреди батча — остаток батча получает synthetic skipped
// tool_result (Anthropic/OpenAI требуют парности call↔result).
func TestExecutor_ParkInMidBatch_RemainderGetsSkippedResult(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{
			{ID: "tc-1", Type: "function", Function: llm.Function{Name: "project_list", Arguments: `{}`}},
			{ID: "tc-2", Type: "function", Function: llm.Function{Name: "project_delete", Arguments: `{"id":"p"}`}},
			{ID: "tc-3", Type: "function", Function: llm.Function{Name: "project_list", Arguments: `{}`}},
		}},
	}}

	tools := []Tool{
		{Name: "project_list", Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"items":[]}`), nil
		}},
		{Name: "project_delete", RequiresConfirmation: true,
			Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"status":"ok"}`), nil
			}},
	}

	type seen struct {
		id     string
		status string
	}
	var got []seen
	hooks := Hooks{
		OnConfirmRequired: func(ctx context.Context, _ ToolCall) (ConfirmDecision, error) {
			return ConfirmPark, nil
		},
		OnToolResult: func(ctx context.Context, r ToolResult) error {
			got = append(got, seen{r.CallID, r.Status})
			return nil
		},
	}

	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "do stuff"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusParked {
		t.Fatalf("expected Parked, got %s", res.Status)
	}
	// tc-1 — успешно выполнен (был ДО парк-точки);
	// tc-3 — должен получить status=skipped (после парка);
	// tc-2 — собственно парк, OnToolResult для него НЕ дёргается (нет результата).
	var sawOK, sawSkipped bool
	for _, s := range got {
		if s.id == "tc-1" && s.status == "ok" {
			sawOK = true
		}
		if s.id == "tc-3" && s.status == "skipped" {
			sawSkipped = true
		}
		if s.id == "tc-2" {
			t.Errorf("parked call must not emit OnToolResult, got %+v", s)
		}
	}
	if !sawOK {
		t.Errorf("expected ok result for tc-1, got events: %+v", got)
	}
	if !sawSkipped {
		t.Errorf("expected skipped result for tc-3, got events: %+v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Cancellation / timeout.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_ParentCtxCancel_BeforeFirstCall(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{{Content: "ok"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем СРАЗУ
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(ctx, RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if !errors.Is(res.Cause, context.Canceled) {
		t.Fatalf("expected Cause=Canceled, got %v", res.Cause)
	}
	if client.Calls() != 0 {
		t.Fatalf("client must not be called on pre-cancelled ctx, got %d calls", client.Calls())
	}
}

func TestExecutor_PerLLMCallTimeout_FailsOnSlowProvider(t *testing.T) {
	t.Parallel()
	// Клиент «зависает» дольше, чем PerLLMCallTimeout. Executor должен
	// прервать через withOptionalTimeout и вернуть Failed.
	client := &fakeLLM{
		responses: []*llm.Response{{Content: "never"}},
		chatHook: func(ctx context.Context, _ llm.Request) {
			select {
			case <-ctx.Done():
			case <-time.After(200 * time.Millisecond):
			}
		},
	}
	exec := NewExecutor(Config{
		MaxIterations:     3,
		PerLLMCallTimeout: 20 * time.Millisecond,
	}, nil)
	start := time.Now()
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "slow"}},
	})
	dur := time.Since(start)
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if dur > 150*time.Millisecond {
		t.Fatalf("timeout не сработал, длительность %v", dur)
	}
}

func TestExecutor_ParentCtxCancelBetweenIterations(t *testing.T) {
	t.Parallel()
	// Первая итерация успешна (tool_call), вторая — НЕ должна стартовать,
	// потому что после первого tool-execute ctx отменён.
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeLLM{
		responses: []*llm.Response{
			{ToolCalls: []llm.ToolCall{{
				ID: "tc-1", Type: "function",
				Function: llm.Function{Name: "noop", Arguments: `{}`},
			}}},
			{Content: "should not be reached"},
		},
	}
	tools := []Tool{{
		Name: "noop",
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			// Отменяем родителя СРАЗУ после исполнения tool'а.
			cancel()
			return json.RawMessage(`{"status":"ok"}`), nil
		},
	}}
	exec := NewExecutor(Config{MaxIterations: 5}, nil)
	res, err := exec.Run(ctx, RunRequest{
		Client: client, Tools: tools,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
		Hooks:   Hooks{},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s (cause=%v)", res.Status, res.Cause)
	}
	if client.Calls() != 1 {
		t.Fatalf("expected exactly 1 client call (second iter must not start), got %d", client.Calls())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Hook ошибки.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_HookError_FailsRun(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{{Content: "ok"}}}
	hookErr := errors.New("persist failed")
	hooks := Hooks{
		OnFinalText: func(ctx context.Context, _ string) error { return hookErr },
	}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if res.Cause == nil || !strings.Contains(res.Cause.Error(), "persist failed") {
		t.Fatalf("unexpected cause: %v", res.Cause)
	}
}

// OnToolResult — observability-хук. По контракту его ошибка НЕ должна
// прерывать петлю (см. executor.go emitToolResult).
func TestExecutor_OnToolResultError_DoesNotFailRun(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "ping", Arguments: `{}`},
		}}},
		{Content: "done"},
	}}
	tools := []Tool{{Name: "ping", Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"status":"ok"}`), nil
	}}}
	hooks := Hooks{
		OnToolResult: func(ctx context.Context, _ ToolResult) error { return errors.New("ws down") },
		OnFinalText:  func(ctx context.Context, _ string) error { return nil },
	}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("OnToolResult error must NOT fail Run, got %s (cause=%v)", res.Status, res.Cause)
	}
}

// Tool handler возвращает бизнес-ошибку → синтетический error tool_result,
// петля продолжает.
func TestExecutor_ToolBusinessError_ContinuesLoop(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "validator", Arguments: `{}`},
		}}},
		{Content: "понял"},
	}}
	tools := []Tool{{
		Name: "validator",
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("bad request")
		},
	}}
	var seenStatus string
	hooks := Hooks{
		OnToolResult: func(ctx context.Context, r ToolResult) error {
			seenStatus = r.Status
			return nil
		},
		OnFinalText: func(ctx context.Context, _ string) error { return nil },
	}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools, Hooks: hooks,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", res.Status)
	}
	if seenStatus != "error" {
		t.Fatalf("expected error status, got %q", seenStatus)
	}
}

// Tool handler возвращает ctx-ошибку → Failed.
func TestExecutor_ToolCtxError_FailsRun(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{
			ID: "tc-1", Type: "function",
			Function: llm.Function{Name: "slow", Arguments: `{}`},
		}}},
	}}
	tools := []Tool{{
		Name: "slow",
		Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			return nil, context.DeadlineExceeded
		},
	}}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client: client, Tools: tools,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if !errors.Is(res.Cause, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded in cause, got %v", res.Cause)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation: bad config / catalog.
// ─────────────────────────────────────────────────────────────────────────────

func TestExecutor_NilClient_Error(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{MaxIterations: 1}, nil)
	_, err := exec.Run(context.Background(), RunRequest{})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestExecutor_DuplicateToolNames_ConfigError(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{MaxIterations: 1}, nil)
	noopH := func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	}
	_, err := exec.Run(context.Background(), RunRequest{
		Client: &fakeLLM{}, Tools: []Tool{
			{Name: "dup", Handler: noopH},
			{Name: "dup", Handler: noopH},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("expected duplicate tool name error, got %v", err)
	}
}

func TestExecutor_ToolWithoutHandler_ConfigError(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{MaxIterations: 1}, nil)
	_, err := exec.Run(context.Background(), RunRequest{
		Client: &fakeLLM{}, Tools: []Tool{{Name: "x", Handler: nil}},
	})
	if err == nil || !strings.Contains(err.Error(), "nil handler") {
		t.Fatalf("expected nil handler error, got %v", err)
	}
}

func TestExecutor_DestructiveToolWithoutConfirmHook_ConfigError(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{MaxIterations: 1}, nil)
	_, err := exec.Run(context.Background(), RunRequest{
		Client: &fakeLLM{}, Tools: []Tool{{
			Name: "x", RequiresConfirmation: true,
			Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
				return nil, nil
			},
		}},
		Hooks: Hooks{}, // OnConfirmRequired = nil
	})
	if err == nil || !strings.Contains(err.Error(), "OnConfirmRequired") {
		t.Fatalf("expected OnConfirmRequired config error, got %v", err)
	}
}

func TestExecutor_EmptyToolName_ConfigError(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{MaxIterations: 1}, nil)
	_, err := exec.Run(context.Background(), RunRequest{
		Client: &fakeLLM{}, Tools: []Tool{{Name: "", Handler: func(ctx context.Context, _ AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			return nil, nil
		}}},
	})
	if err == nil || !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("expected empty name config error, got %v", err)
	}
}

// LLM-провайдер сам вернул error → Failed.
func TestExecutor_LLMClientError_Failed(t *testing.T) {
	t.Parallel()
	client := &fakeLLM{errs: []error{errors.New("provider 500")}}
	exec := NewExecutor(Config{MaxIterations: 3}, nil)
	res, err := exec.Run(context.Background(), RunRequest{
		Client:  client,
		History: []Message{{Role: llm.RoleUser, Content: "?"}},
	})
	if err != nil {
		t.Fatalf("config err: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected Failed, got %s", res.Status)
	}
	if res.Cause == nil || !strings.Contains(res.Cause.Error(), "provider 500") {
		t.Fatalf("cause mismatch: %v", res.Cause)
	}
}

// Default MaxIterations: 0 → 12 (план §3.2).
func TestExecutor_DefaultMaxIterations_12(t *testing.T) {
	t.Parallel()
	exec := NewExecutor(Config{}, nil)
	if exec.Config().MaxIterations != 12 {
		t.Fatalf("expected default 12, got %d", exec.Config().MaxIterations)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers (локальный itoa, чтобы не тянуть strconv).
// ─────────────────────────────────────────────────────────────────────────────

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
