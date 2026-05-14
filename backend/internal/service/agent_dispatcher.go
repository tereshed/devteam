package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
)

// agent_dispatcher.go — Sprint 17 / Orchestration v2 — единая точка резолва агента → исполнителя.
//
// ВАЖНО: это ЕДИНСТВЕННОЕ место в коде с `switch agent.ExecutionKind`. Это не нарушает
// принцип "flow as data" — здесь выбирается runtime агента (llm vs sandbox), а не
// маршрутизация задачи. Маршрутизация (кому какой запрос) — внутри Router'а на основе
// LLM-промпта.
//
// Зависимости (LLMProviderResolver, SandboxExecutorFactory) — интерфейсы, поэтому
// конкретные реализации (resolve provider_kind → llm.Provider; build sandbox runner)
// можно подменять в тестах и подключать в Sprint 3 через DI в cmd/api/main.go.

// ErrUnknownExecutionKind — agent.ExecutionKind не соответствует ни одному из известных.
// Защита от случая когда в БД через миграцию попало неизвестное значение.
var ErrUnknownExecutionKind = errors.New("unknown agent execution kind")

// LLMProviderResolver — резолвит llm.Provider для llm-агента.
//
// Существующая реализация в проекте (см. SandboxAuthEnvResolver и связанные) маппит
// agent.ProviderKind в models.LLMProvider, потом передаёт в llm.NewLLMClient. В Sprint 3
// мы оборачиваем это в LLMProviderResolver для использования из dispatcher'а.
type LLMProviderResolver interface {
	ResolveLLMProvider(ctx context.Context, agent *models.Agent) (llm.Provider, error)
}

// SandboxExecutorFactory — строит agent.AgentExecutor для sandbox-агента.
//
// Существующий agent.SandboxAgentExecutor уже умеет работать с code_backend
// (claude-code|aider|hermes|custom) и AgentSettings (см. internal/agent/sandbox_executor.go).
// Factory собирает его с учётом agent.SandboxPermissions, agent.CodeBackendSettings и
// (новое в Sprint 1) agent_secrets через AgentSecretRepository.
type SandboxExecutorFactory interface {
	BuildSandboxExecutor(ctx context.Context, agent *models.Agent) (agent.AgentExecutor, error)
}

// AgentDispatcher — резолвит agent.AgentExecutor по типу агента.
type AgentDispatcher interface {
	BuildExecutor(ctx context.Context, a *models.Agent) (agent.AgentExecutor, error)
}

// agentDispatcher — основная реализация.
type agentDispatcher struct {
	llmResolver    LLMProviderResolver
	sandboxFactory SandboxExecutorFactory
}

// NewAgentDispatcher — конструктор. Обе зависимости обязательны: попытка resolve'нуть
// агента с ExecutionKind, для которого нет соответствующей зависимости — ошибка.
func NewAgentDispatcher(llmResolver LLMProviderResolver, sandboxFactory SandboxExecutorFactory) AgentDispatcher {
	return &agentDispatcher{
		llmResolver:    llmResolver,
		sandboxFactory: sandboxFactory,
	}
}

// BuildExecutor возвращает исполнителя для агента.
//
// Логика — простой switch по execution_kind. Дополнительная проверка: для llm-агента
// agent.Model должен быть задан (а code_backend — пуст); для sandbox — наоборот.
// Это уже гарантировано CHECK chk_agents_kind_requirements в БД, но защищает на случай
// если кто-то соберёт Agent struct в коде в обход модели (тесты, миграция данных).
func (d *agentDispatcher) BuildExecutor(ctx context.Context, a *models.Agent) (agent.AgentExecutor, error) {
	if a == nil {
		return nil, fmt.Errorf("agent dispatcher: agent is nil")
	}

	switch a.ExecutionKind {
	case models.AgentExecutionKindLLM:
		if d.llmResolver == nil {
			return nil, fmt.Errorf("agent dispatcher: llmResolver is not configured (agent=%s)", a.Name)
		}
		if a.Model == nil || *a.Model == "" {
			return nil, fmt.Errorf("agent dispatcher: llm-agent %q has empty model", a.Name)
		}
		provider, err := d.llmResolver.ResolveLLMProvider(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("agent dispatcher: resolve llm provider for %q: %w", a.Name, err)
		}
		return agent.NewLLMAgentExecutor(provider), nil

	case models.AgentExecutionKindSandbox:
		if d.sandboxFactory == nil {
			return nil, fmt.Errorf("agent dispatcher: sandboxFactory is not configured (agent=%s)", a.Name)
		}
		if a.CodeBackend == nil {
			return nil, fmt.Errorf("agent dispatcher: sandbox-agent %q has empty code_backend", a.Name)
		}
		return d.sandboxFactory.BuildSandboxExecutor(ctx, a)

	default:
		return nil, fmt.Errorf("%w: %q (agent=%s)", ErrUnknownExecutionKind, a.ExecutionKind, a.Name)
	}
}
