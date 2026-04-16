package agent

import "context"

// AgentExecutor — один шаг выполнения логики агента над подготовленным контекстом задачи.
// Реализации: LLMAgentExecutor (6.2), SandboxAgentExecutor (6.3).
//
// Контракт отмены: ctx отмены/дедлайна должен прерывать сетевой вызов LLM и инициировать
// остановку sandbox (через SandboxRunner.Stop/Cleanup) в реализации 6.3 — детали в задаче 6.3.
//
// Контракт ошибок (жёстко, одинаково для 6.2 и 6.3):
//   - error != nil: только инфраструктура/система, отмена ctx, невалидный вход (в т.ч. JSON при len>0), не сконфигурированный executor.
//   - error == nil && Success == false: штатное завершение, но бизнес-цель не достигнута; детали в Output / ArtifactsJSON.
//   - error == nil && Success == true: шаг считается успешным по политике исполнителя.
//
// Исполнитель в MVP не пишет в БД и не меняет Task.Status — это зона OrchestratorService (6.4+).
type AgentExecutor interface {
	// Execute выполняет работу агента один раз (один «тик» пайплайна для данной задачи).
	// Не пишет в БД и не меняет Task.Status — это обязанность OrchestratorService (6.4+).
	Execute(ctx context.Context, in ExecutionInput) (*ExecutionResult, error)
}
