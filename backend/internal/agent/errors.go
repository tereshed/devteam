package agent

import "errors"

// Sentinel-ошибки пакета agent (см. docs/tasks/6.1-agent-executor-interface.md).

var (
	// ErrInvalidExecutionInput — пустой TaskID, отсутствует обязательное поле для реализации,
	// невалидный JSON при ненулевой длине и т.п.
	ErrInvalidExecutionInput = errors.New("agent: invalid execution input")

	// ErrExecutionCancelled — опциональная обёртка над context.Canceled / DeadlineExceeded
	// для единого стиля в реализациях 6.2–6.3 (возврат как error != nil, не Success == false).
	ErrExecutionCancelled = errors.New("agent: execution cancelled")

	// ErrExecutorNotConfigured — нет API-ключа, не задан SandboxRunner и т.п.
	ErrExecutorNotConfigured = errors.New("agent: executor not configured")
)
