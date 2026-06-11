// Package agentloop — общий движок tool-calling LLM-петли (Sprint 21 §3.2).
//
// Используется Assistant'ом (sidebar agent) и, в follow-up'е, Router'ом —
// чтобы цикл «собрать историю → вызвать LLM с tools → обработать tool_use /
// final_text → итерация» не дублировался по сервисам.
//
// Контракт:
//   - Executor.Run принимает RunRequest с историей сообщений, каталогом
//     инструментов и хуками.
//   - На каждой итерации до LLM-вызова проверяется ctx.Err() — гарантия,
//     что cancellation/timeout не «пролетает» между шагами.
//   - Per-LLM-call timeout навешивается дополнительно поверх parent ctx.
//   - Tool_result для истории всегда проходит через truncateToolResultForHistory
//     (см. history.go) — сырой payload остаётся только в БД и в WS-эмиссии.
//   - При requiresConfirmation=true Executor вызывает Hooks.OnConfirmRequired;
//     возвращённый ConfirmDecision определяет дальнейшее поведение (Park/
//     Approve/Deny). Assistant в своём хуке возвращает Park, что приводит
//     к Status=Parked и завершению Run без выполнения tool'а.
package agentloop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/devteam/backend/pkg/llm"
)

// Status — итоговое состояние Run().
type Status string

const (
	// StatusCompleted — LLM выдал финальный текст без tool_calls. Result.Iterations
	// содержит число выполненных итераций.
	StatusCompleted Status = "completed"

	// StatusParked — петля «припаркована» на ожидании пользовательского
	// подтверждения destructive-инструмента. ParkedCall != nil.
	StatusParked Status = "parked"

	// StatusLimitExceeded — превышен MaxIterations. Assistant пишет в историю
	// сообщение «превышен лимит шагов».
	StatusLimitExceeded Status = "limit_exceeded"

	// StatusFailed — инфраструктурная ошибка (LLM/tool/ctx). Result.Cause содержит
	// причину. Assistant пишет в историю «запрос к модели не завершился вовремя,
	// попробуйте ещё раз» без сырых деталей.
	StatusFailed Status = "failed"
)

// AuthContext — кто исполняет петлю (см. §3.3). Pass-through в каждый
// ToolHandler.Invoke — handler сам кладёт UserID в свой ctx-ключ перед
// вызовом MCP/сервиса.
type AuthContext struct {
	UserID    string
	ProjectID string // Сюда пробрасывается ID проекта, если сессия привязана к проекту
	Scope     string // "assistant" | "router" — для аудита/метрик
}

// ToolHandler — исполнитель одного MCP-инструмента. Возвращает СЫРОЙ JSON
// результата (как должен видеть LLM после truncation). Внутри handler:
//   - вытащить UserID из auth и положить в свой ctx-ключ;
//   - провалидировать args;
//   - вызвать сервис;
//   - смаршалить результат в []byte.
//
// Ошибки handler'а попадают в tool_result как `{status:"error", message:...}`
// и идут на следующую итерацию LLM — Executor не прерывает Run на tool error,
// чтобы LLM мог понять провал и попробовать иначе. Сетевые/таймаут ошибки
// должны быть смаплены в `error` Go-уровня, чтобы Executor вернул Failed.
type ToolHandler func(ctx context.Context, auth AuthContext, args json.RawMessage) (result json.RawMessage, err error)

// Tool — дескриптор инструмента в каталоге.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// RequiresConfirmation — если true и LLM запросил этот tool, Executor
	// перед исполнением зовёт Hooks.OnConfirmRequired. Зависит от ConfirmDecision.
	RequiresConfirmation bool
	Handler              ToolHandler
}

// Message — нейтральное представление одного шага истории (БД-row или
// synthetic system-сообщение). Executor конвертирует в llm.Message
// через history.go (с применением truncation).
type Message struct {
	Role          llm.Role
	Content       string
	ToolCallID    string          // tool-row → ссылка на парный assistant.ToolCalls[i].ID
	ToolCalls     []llm.ToolCall  // assistant-row с tool_use
	ToolResult    json.RawMessage // tool-row — сырой результат; будет truncated при подаче в LLM
	ToolName      string          // tool-row — для подсказки модели «{tool}: ...truncated...»
	ToolArguments json.RawMessage // только для аудита/логов, в историю LLM не идёт повторно
}

// ToolCall — событие «LLM запросил инструмент» для хука Hooks.OnToolCall.
// Поля копируются из llm.ToolCall + парсенные аргументы.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
	// RequiresConfirmation — копия из Tool.RequiresConfirmation; хук Assistant'а
	// использует это, чтобы решить — звать OnConfirmRequired или сразу OnToolCall.
	// (Сейчас Executor сам ветвит, но поле полезно для логов.)
	RequiresConfirmation bool
}

// ToolResult — событие «инструмент отработал» для хука Hooks.OnToolResult.
// Result — сырой payload (до truncation). Status — короткий ярлык для UI:
// "ok" | "error" | "forbidden" | "denied" (последний — synthetic от Deny).
type ToolResult struct {
	CallID string
	Name   string
	Status string
	Result json.RawMessage
}

// AssistantMsg — событие «LLM выдал промежуточный или финальный assistant-текст».
// Передаётся в Hooks.OnAssistantMessage. ToolCalls != nil → промежуточный шаг
// (LLM хочет вызвать tools); пустой ToolCalls и непустой Content → финал.
type AssistantMsg struct {
	Content   string
	ToolCalls []llm.ToolCall
	Usage     llm.Usage
}

// ConfirmDecision — результат хука Hooks.OnConfirmRequired.
type ConfirmDecision string

const (
	// ConfirmPark — пользователь ещё не ответил; petlja паркуется, Assistant
	// сохраняет pending_tool_call_id и завершает горутину. Resume произойдёт
	// при POST /confirm через ConfirmAndClosePending → новый Run.
	ConfirmPark ConfirmDecision = "park"

	// ConfirmApprove — пользователь подтвердил; Executor выполняет tool как обычно.
	// Используется при resume — Assistant возвращает Approve, потому что
	// решение уже зафиксировано в БД (ConfirmAndClosePending).
	ConfirmApprove ConfirmDecision = "approve"

	// ConfirmDeny — пользователь отказал; Executor НЕ выполняет tool, а
	// инжектит synthetic tool_result `{status:"denied", ...}` и идёт дальше.
	// Используется при resume по deny-пути.
	ConfirmDeny ConfirmDecision = "deny"
)

// Hooks — callbacks для Executor. Любой возврат error приводит к
// Status=Failed и прерыванию Run. Hooks могут быть nil-обнулёнными — это
// допустимо для Router (нет UI-эмиссии).
//
// Контракт OnConfirmRequired:
//   - nil → инструменты с RequiresConfirmation=true НЕЛЬЗЯ регистрировать;
//     Executor вернёт Failed (config error), чтобы не уйти в бесконечный loop.
//   - возврат ConfirmPark → Run завершается Status=Parked, ParkedCall заполнен.
//   - возврат ConfirmApprove/Deny → Run продолжается соответственно.
type Hooks struct {
	OnAssistantMessage func(ctx context.Context, msg AssistantMsg) error
	OnToolCall         func(ctx context.Context, call ToolCall) error
	OnToolResult       func(ctx context.Context, res ToolResult) error
	OnConfirmRequired  func(ctx context.Context, call ToolCall) (ConfirmDecision, error)
	OnFinalText        func(ctx context.Context, text string) error
}

// Config — параметры Executor. Все значения обязательны, magic numbers в
// коде запрещены (§3.4). Передаются через DI в конструкторе Executor.
type Config struct {
	// MaxIterations — жёсткий лимит шагов LLM↔tools в одном Run. Превышение →
	// Status=LimitExceeded. План §3.2: дефолт 12.
	MaxIterations int

	// MaxToolResultBytes — лимит сериализованного tool_result, после которого
	// Executor отдаёт LLM-у truncated preview с маркером (§3.4 п.1). План: 16 KiB.
	MaxToolResultBytes int

	// MaxHistoryBytes — оценочный бюджет всей истории перед LLM-вызовом
	// (§3.4 п.3). При превышении самые старые tool_result-сообщения сжимаются
	// до коротких summary. План: 0.8 * model_context_window, выраженное в байтах.
	MaxHistoryBytes int

	// HistoryTailKeep — сколько последних user/assistant сообщений всегда
	// остаются в полном виде при sliding-window-сжатии. План: 8.
	HistoryTailKeep int

	// PerLLMCallTimeout — обёртка `context.WithTimeout` вокруг каждого
	// LLMClient.Chat (§3.1 защита от slow-stream). План: 90s.
	PerLLMCallTimeout time.Duration
}

// RunRequest — вход для Executor.Run.
type RunRequest struct {
	// Client — LLM-клиент, готовый к вызову (созданный фабрикой
	// internal/llm.NewLLMClient). Не nil.
	Client llm.Client

	// Model, SystemPrompt, Temperature, MaxTokens — те же поля, что у llm.Request.
	Model        string
	SystemPrompt string
	Temperature  *float64
	MaxTokens    *int

	// Provider — kind LLM-провайдера (openai/anthropic/openrouter/...).
	// Пустая строка → Executor оставит llm.Request.Provider="" и llmService
	// упадёт на defaultProvider. Заполнять обязательно для агентов с
	// фиксированным provider_kind (assistant, orchestrator, planner).
	// Тип — string, а не llm.ProviderType, чтобы агентный код мог скастить
	// из models.AgentProviderKind без импорта pkg/llm. Executor сам приведёт
	// к llm.ProviderType.
	Provider string

	// History — полная история сессии в хронологическом порядке (старые → новые).
	// Executor сам прогоняет её через history.go (truncation/sliding window) перед
	// сборкой llm.Request.Messages.
	History []Message

	// Tools — каталог инструментов, видимых LLM в этой Run. Имена должны быть
	// уникальны. Пустой каталог допустим (Router use case): tool_calls в ответе
	// LLM в этом случае трактуется как ошибка модели — Executor вернёт Failed.
	Tools []Tool

	// ServerTools — провайдер-специфичные server-side тулы (например
	// {"type": "openrouter:web_search"}): исполняются самим провайдером,
	// tool_call не приходит, результат уже в тексте ответа. Прокидываются
	// в llm.Request.ServerTools как есть. Только для провайдеров, понимающих
	// нестандартный type (OpenRouter); для остальных оставлять nil.
	ServerTools []map[string]any

	// Auth — пробрасывается в каждый ToolHandler.Invoke (§3.3).
	Auth AuthContext

	// Hooks — событийные колбэки. См. Hooks doc выше.
	Hooks Hooks
}

// Result — итог Run. Status — терминальное состояние. Cause != nil только
// для StatusFailed. ParkedCall != nil только для StatusParked.
type Result struct {
	Status     Status
	Iterations int
	ParkedCall *ToolCall
	Cause      error
	// LastAssistantText — финальный текст модели (если есть). Для Parked/
	// LimitExceeded/Failed может быть пустым.
	LastAssistantText string
}
