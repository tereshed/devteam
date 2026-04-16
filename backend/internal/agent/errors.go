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

	// ErrRateLimit — превышен лимит запросов к LLM API.
	ErrRateLimit = errors.New("agent: rate limit exceeded")

	// ErrContextTooLarge — размер промпта и контекста превышает лимит модели.
	ErrContextTooLarge = errors.New("agent: context too large")

	// ErrInvalidResponse — невалидный ответ от LLM (например, невалидный JSON в Artifacts).
	ErrInvalidResponse = errors.New("agent: invalid response format")
)
