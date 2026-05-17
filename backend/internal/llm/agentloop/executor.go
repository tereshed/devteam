package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/pkg/llm"
)

// executor.go — Sprint 21 §3.2. Цикл tool-calling LLM-петли.
//
// Контракт жизненного цикла одной Run():
//
//   1. Валидируем RunRequest (client, tools уникальны, handler у каждого
//      инструмента есть, RequiresConfirmation→OnConfirmRequired не nil).
//   2. На каждой итерации (до MaxIterations):
//        a. ctx.Err() check — fail-fast если parent ctx уже отменён;
//        b. собираем llm.Request из History (через history.go);
//        c. per-call timeout ctx → Client.Chat;
//        d. интерпретация Response:
//             - есть ToolCalls → исполняем последовательно
//               (с возможной паркой/деней по confirm);
//             - нет ToolCalls, есть Content → финал, Status=Completed;
//             - пусто → Failed (LLM вернул ничего).
//   3. На каждом tool.execute мы:
//        - вызываем Tool.Handler с правильным auth;
//        - оборачиваем ошибку handler'а в `{status:"error",...}` (это идёт
//          в LLM, чтобы он мог отреагировать), но НЕ прерываем Run;
//        - сетевые / ctx ошибки трактуются как Failed (Cause != nil).
//
// Инвариант: Executor НЕ пишет в БД. Все side effects (assistant_messages
// append, WS-эмиссия, park-state) делает caller через Hooks.

// Executor — stateless движок tool-loop. Один экземпляр на процесс,
// потокобезопасный (все state — в аргументах Run).
type Executor struct {
	cfg    Config
	logger *slog.Logger
}

// NewExecutor — конструктор. logger ОБЯЗАН быть с redact-обёрткой
// (logging.NewHandler), иначе sensitive поля LLM-ответа утекут в логи.
// nil → NopLogger (тесты/локальный dev).
func NewExecutor(cfg Config, logger *slog.Logger) *Executor {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 12
	}
	if cfg.HistoryTailKeep < 0 {
		cfg.HistoryTailKeep = 0
	}
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Executor{cfg: cfg, logger: logger}
}

// Config возвращает копию конфигурации (для caller'а — например, чтобы
// синхронизировать stale-recovery threshold с loop timeout).
func (e *Executor) Config() Config {
	return e.cfg
}

// Run — основной метод. См. контракт выше.
//
// Возвращаемые ошибки:
//   - error == nil — Result.Status валиден и обработан выше.
//   - error != nil — фатальная ошибка валидации входа (плохая конфигурация
//     handler'ов / RunRequest). Это всегда программная ошибка: caller должен
//     упасть, а не ретраить.
func (e *Executor) Run(ctx context.Context, req RunRequest) (Result, error) {
	if req.Client == nil {
		return Result{}, errors.New("agentloop: RunRequest.Client is nil")
	}
	toolCatalog, err := buildToolCatalog(req.Tools, req.Hooks)
	if err != nil {
		return Result{}, err
	}

	// Локальная копия истории — мы дописываем в неё новые assistant/tool
	// сообщения по ходу петли. Caller обычно сразу персистит их через хуки
	// (OnAssistantMessage / OnToolResult), но в самой Run-локальной истории
	// мы их тоже храним, чтобы следующий llm.Chat увидел контекст.
	history := append([]Message(nil), req.History...)

	result := Result{Status: StatusFailed}
	for iter := 0; iter < e.cfg.MaxIterations; iter++ {
		result.Iterations = iter + 1

		// (a) Cancellation check ДО LLM-вызова — гарантия §3.1, что timeout
		// не «пролетает» между итерациями.
		if err := ctx.Err(); err != nil {
			result.Status = StatusFailed
			result.Cause = err
			return result, nil
		}

		// (b) Сборка запроса.
		llmReq := llm.Request{
			Model:        req.Model,
			SystemPrompt: req.SystemPrompt,
			Messages:     buildLLMMessages(history, e.cfg),
			Tools:        descriptorsFor(req.Tools),
			Temperature:  req.Temperature,
			MaxTokens:    req.MaxTokens,
		}

		// (c) Per-call timeout.
		callCtx, cancel := withOptionalTimeout(ctx, e.cfg.PerLLMCallTimeout)
		resp, err := req.Client.Chat(callCtx, llmReq)
		cancel()
		if err != nil {
			result.Status = StatusFailed
			result.Cause = fmt.Errorf("llm chat (iter=%d): %w", iter, err)
			return result, nil
		}
		if resp == nil {
			result.Status = StatusFailed
			result.Cause = fmt.Errorf("llm chat returned nil response (iter=%d)", iter)
			return result, nil
		}

		// (d) Hook on the assistant message ДО исполнения тулзы — фронт
		// должен увидеть текст «думаю, сейчас вызову X» сразу.
		assistantMsg := AssistantMsg{
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
			Usage:     resp.Usage,
		}
		if req.Hooks.OnAssistantMessage != nil {
			if hookErr := req.Hooks.OnAssistantMessage(ctx, assistantMsg); hookErr != nil {
				result.Status = StatusFailed
				result.Cause = fmt.Errorf("hook OnAssistantMessage: %w", hookErr)
				return result, nil
			}
		}

		// Запоминаем в локальной истории — чтобы следующий итерационный
		// llm.Chat увидел tool_calls и ожидаемые tool-results.
		history = append(history, Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Финальный текст: нет вызовов tools — диалог завершён.
		if len(resp.ToolCalls) == 0 {
			if resp.Content == "" {
				// Анти-флейк: некоторые провайдеры могут вернуть пустой текст
				// без tool_calls (например, content_filter). Это явная Failed —
				// caller покажет пользователю «попробуйте ещё раз».
				result.Status = StatusFailed
				result.Cause = fmt.Errorf("llm returned empty response without tool calls (iter=%d)", iter)
				return result, nil
			}
			if req.Hooks.OnFinalText != nil {
				if hookErr := req.Hooks.OnFinalText(ctx, resp.Content); hookErr != nil {
					result.Status = StatusFailed
					result.Cause = fmt.Errorf("hook OnFinalText: %w", hookErr)
					return result, nil
				}
			}
			result.Status = StatusCompleted
			result.LastAssistantText = resp.Content
			return result, nil
		}

		// Есть tool_calls — но каталог пуст. Это противоречит RunRequest
		// (caller указал, что инструментов нет, а модель их зачем-то зовёт).
		if len(req.Tools) == 0 {
			result.Status = StatusFailed
			result.Cause = fmt.Errorf("llm requested %d tool_call(s) but RunRequest.Tools is empty (iter=%d)", len(resp.ToolCalls), iter)
			return result, nil
		}

		// Исполняем tool_calls последовательно. Параллельность — отдельная
		// история (нужны транзакционные гарантии на pending_tool_call_id),
		// в этот спринт не делаем.
		//
		// PARITY-инвариант: Anthropic/OpenAI требуют, чтобы для КАЖДОГО
		// assistant.tool_call в истории присутствовал парный tool.tool_result.
		// При обрыве батча на ConfirmPark остаток БАТЧА должен получить
		// synthetic skip-результаты, иначе следующий вызов LLM падает
		// фатальным «missing corresponding tool_result».
		for i, call := range resp.ToolCalls {
			tool, ok := toolCatalog[call.Function.Name]
			if !ok {
				// LLM запросил несуществующий инструмент — отдаём ему
				// synthetic error, идём на следующую итерацию (он должен
				// поправиться).
				errPayload := mustMarshalToolError("unknown_tool", fmt.Sprintf("tool %q is not in the catalog", call.Function.Name))
				history = appendToolResult(history, call, "", errPayload)
				e.emitToolResult(ctx, req.Hooks, call, "error", errPayload)
				continue
			}

			args := json.RawMessage(call.Function.Arguments)

			// Confirm-gate для destructive tools.
			if tool.RequiresConfirmation {
				decision, err := req.Hooks.OnConfirmRequired(ctx, ToolCall{
					ID:                   call.ID,
					Name:                 call.Function.Name,
					Arguments:            args,
					RequiresConfirmation: true,
				})
				if err != nil {
					result.Status = StatusFailed
					result.Cause = fmt.Errorf("hook OnConfirmRequired: %w", err)
					return result, nil
				}
				switch decision {
				case ConfirmPark:
					parked := ToolCall{
						ID:                   call.ID,
						Name:                 call.Function.Name,
						Arguments:            args,
						RequiresConfirmation: true,
					}
					result.Status = StatusParked
					result.ParkedCall = &parked

					// Дренаж остатка батча: парный tool_result обязателен
					// для каждого assistant.tool_call (см. PARITY-комментарий
					// выше). Skipped-payload идёт через те же OnToolResult-
					// хуки, что и обычные результаты — caller persist'ит
					// их в assistant_messages, и следующий LLM-вызов
					// получит закрытый батч.
					for j := i + 1; j < len(resp.ToolCalls); j++ {
						skipCall := resp.ToolCalls[j]
						skipPayload := mustMarshalToolError(
							"skipped",
							"инструмент пропущен: предыдущий вызов из этого батча ожидает подтверждения",
						)
						history = appendToolResult(history, skipCall, skipCall.Function.Name, skipPayload)
						e.emitToolResult(ctx, req.Hooks, skipCall, "skipped", skipPayload)
					}
					return result, nil
				case ConfirmApprove:
					// fallthrough — исполняем обычным путём ниже.
				case ConfirmDeny:
					denyPayload := mustMarshalToolError("denied", "user denied confirmation for this tool call")
					history = appendToolResult(history, call, tool.Name, denyPayload)
					e.emitToolResult(ctx, req.Hooks, call, "denied", denyPayload)
					continue
				default:
					result.Status = StatusFailed
					result.Cause = fmt.Errorf("invalid ConfirmDecision %q from hook", decision)
					return result, nil
				}
			}

			// OnToolCall — фронт показывает «🔧 tool_name(args)» карточку.
			if req.Hooks.OnToolCall != nil {
				if hookErr := req.Hooks.OnToolCall(ctx, ToolCall{
					ID:                   call.ID,
					Name:                 call.Function.Name,
					Arguments:            args,
					RequiresConfirmation: tool.RequiresConfirmation,
				}); hookErr != nil {
					result.Status = StatusFailed
					result.Cause = fmt.Errorf("hook OnToolCall: %w", hookErr)
					return result, nil
				}
			}

			// Исполняем handler. Сетевые/ctx ошибки → Failed.
			rawResult, execErr := tool.Handler(ctx, req.Auth, args)
			if execErr != nil {
				if isCtxErr(execErr) || isCtxErr(ctx.Err()) {
					result.Status = StatusFailed
					result.Cause = fmt.Errorf("tool %q exec: %w", call.Function.Name, execErr)
					return result, nil
				}
				// Бизнес-ошибка handler'а — отдаём LLM, продолжаем.
				errPayload := mustMarshalToolError("error", execErr.Error())
				history = appendToolResult(history, call, tool.Name, errPayload)
				e.emitToolResult(ctx, req.Hooks, call, "error", errPayload)
				continue
			}
			if len(rawResult) == 0 {
				rawResult = json.RawMessage(`{"status":"ok"}`)
			}

			history = appendToolResult(history, call, tool.Name, rawResult)
			e.emitToolResult(ctx, req.Hooks, call, classifyStatus(rawResult), rawResult)
		}
	}

	// Дошли сюда → исчерпали MaxIterations.
	result.Status = StatusLimitExceeded
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildToolCatalog валидирует каталог и строит lookup-map.
func buildToolCatalog(tools []Tool, hooks Hooks) (map[string]Tool, error) {
	out := make(map[string]Tool, len(tools))
	hasDestructive := false
	for _, t := range tools {
		if t.Name == "" {
			return nil, errors.New("agentloop: tool has empty name")
		}
		if _, dup := out[t.Name]; dup {
			return nil, fmt.Errorf("agentloop: duplicate tool name %q", t.Name)
		}
		if t.Handler == nil {
			return nil, fmt.Errorf("agentloop: tool %q has nil handler", t.Name)
		}
		if t.RequiresConfirmation {
			hasDestructive = true
		}
		out[t.Name] = t
	}
	if hasDestructive && hooks.OnConfirmRequired == nil {
		// Чтобы не уйти в loop, где caller просто игнорирует destructive
		// флаг и инструмент исполняется без подтверждения.
		return nil, errors.New("agentloop: catalog has destructive tool but Hooks.OnConfirmRequired is nil")
	}
	return out, nil
}

// descriptorsFor собирает llm.Tool-описания из локального каталога.
func descriptorsFor(tools []Tool) []llm.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]llm.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return out
}

// appendToolResult добавляет tool-row в локальную историю с привязкой к
// исходному llm.ToolCall.ID.
func appendToolResult(history []Message, call llm.ToolCall, toolName string, result json.RawMessage) []Message {
	return append(history, Message{
		Role:          llm.RoleTool,
		ToolCallID:    call.ID,
		ToolName:      toolName,
		ToolArguments: json.RawMessage(call.Function.Arguments),
		ToolResult:    result,
		Content:       string(result),
	})
}

// emitToolResult зовёт OnToolResult, если он задан.
func (e *Executor) emitToolResult(ctx context.Context, hooks Hooks, call llm.ToolCall, status string, result json.RawMessage) {
	if hooks.OnToolResult == nil {
		return
	}
	if err := hooks.OnToolResult(ctx, ToolResult{
		CallID: call.ID,
		Name:   call.Function.Name,
		Status: status,
		Result: result,
	}); err != nil {
		// Хук — observability. Падение хука не должно прерывать петлю
		// (иначе один сбойный WS-broadcast блокирует всю беседу).
		// Логируем без содержимого результата.
		e.logger.WarnContext(ctx, "agentloop: OnToolResult hook failed",
			slog.String("tool", call.Function.Name),
			slog.String("call_id", call.ID),
			slog.String("error", err.Error()),
		)
	}
}

// mustMarshalToolError собирает payload `{status, message}` для отдачи в LLM.
func mustMarshalToolError(status, message string) json.RawMessage {
	b, err := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{status, message})
	if err != nil {
		return json.RawMessage(`{"status":"error","message":"failed to marshal error"}`)
	}
	return b
}

// classifyStatus возвращает короткий ярлык по содержимому tool_result.
// Использует тот же набор, что compactToolMessage — чтобы UI и compaction
// были согласованы.
func classifyStatus(raw json.RawMessage) string {
	var probe struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "ok"
	}
	switch probe.Status {
	case "error", "forbidden", "denied", "skipped", "ok":
		return probe.Status
	default:
		return "ok"
	}
}

// withOptionalTimeout оборачивает ctx таймаутом, если он > 0. Иначе
// возвращает no-op cancel — caller всегда зовёт cancel(), не различая случаи.
func withOptionalTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
