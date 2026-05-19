package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// v2_di_adapters.go — Sprint 17 / Stage 5g — тонкие адаптеры между новыми v2-интерфейсами
// (LLMProviderResolver, SandboxExecutorFactory, AgentLoader) и существующими singleton'ами
// (llmService, sandboxAgentExecutor, *gorm.DB), которые конструируются в cmd/api/main.go.
//
// Эти адаптеры — простые, не несут бизнес-логики; они нужны чтобы AgentDispatcher
// и RouterService получили зависимости через интерфейсы (testable + DI-friendly),
// а main.go при этом продолжал использовать существующую инфраструктуру без переписывания.

// ─────────────────────────────────────────────────────────────────────────────
// SingletonLLMProviderResolver — возвращает один и тот же llm.Provider для любого
// агента. Это корректно потому, что llmService (NewLLMService → llm.Provider) умеет
// выбирать конкретный backend по llm.Request.Provider/Model, переданных в Execute.
// ─────────────────────────────────────────────────────────────────────────────

type SingletonLLMProviderResolver struct {
	provider llm.Provider
}

// NewSingletonLLMProviderResolver — конструктор.
func NewSingletonLLMProviderResolver(provider llm.Provider) *SingletonLLMProviderResolver {
	return &SingletonLLMProviderResolver{provider: provider}
}

// ResolveLLMProvider реализует LLMProviderResolver. Игнорирует agent (модель/провайдер
// определяются в ExecutionInput внутри LLMAgentExecutor).
func (r *SingletonLLMProviderResolver) ResolveLLMProvider(ctx context.Context, a *models.Agent) (llm.Provider, error) {
	if r == nil || r.provider == nil {
		return nil, errors.New("SingletonLLMProviderResolver: provider is not configured")
	}
	return r.provider, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SingletonSandboxExecutorFactory — возвращает один и тот же agent.AgentExecutor
// для любого sandbox-агента. SandboxAgentExecutor сам выбирает образ контейнера по
// agent.CodeBackend (claude-code/aider/hermes/custom).
// ─────────────────────────────────────────────────────────────────────────────

type SingletonSandboxExecutorFactory struct {
	exec agent.AgentExecutor
}

// NewSingletonSandboxExecutorFactory — конструктор.
func NewSingletonSandboxExecutorFactory(exec agent.AgentExecutor) *SingletonSandboxExecutorFactory {
	return &SingletonSandboxExecutorFactory{exec: exec}
}

// BuildSandboxExecutor реализует SandboxExecutorFactory.
func (f *SingletonSandboxExecutorFactory) BuildSandboxExecutor(ctx context.Context, a *models.Agent) (agent.AgentExecutor, error) {
	if f == nil || f.exec == nil {
		return nil, errors.New("SingletonSandboxExecutorFactory: executor is not configured")
	}
	return f.exec, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// DBAgentLoader — реализует AgentLoader (RouterService) через прямой gorm-lookup.
// ─────────────────────────────────────────────────────────────────────────────

type DBAgentLoader struct {
	db *gorm.DB
}

// NewDBAgentLoader — конструктор.
func NewDBAgentLoader(db *gorm.DB) *DBAgentLoader {
	return &DBAgentLoader{db: db}
}

// GetAgentByName реализует AgentLoader.
func (l *DBAgentLoader) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("DBAgentLoader: db is not configured")
	}
	var a models.Agent
	if err := l.db.WithContext(ctx).Where("name = ?", name).First(&a).Error; err != nil {
		return nil, fmt.Errorf("DBAgentLoader: load agent %q: %w", name, err)
	}
	return &a, nil
}

// GetAgentByUserRole finds a user-owned agent by role (e.g. per-user assistant).
// Falls back to GetAgentByName for backward compatibility with global agents.
func (l *DBAgentLoader) GetAgentByUserRole(ctx context.Context, userID uuid.UUID, role string) (*models.Agent, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("DBAgentLoader: db is not configured")
	}
	var a models.Agent
	err := l.db.WithContext(ctx).Where("user_id = ? AND role = ?", userID, role).First(&a).Error
	if err == nil {
		return &a, nil
	}
	// Fallback to global (system-level) agent by name for backward compat.
	return l.GetAgentByName(ctx, role)
}

// ─────────────────────────────────────────────────────────────────────────────
// AssistantLLMClientAdapter — Sprint 21. Реализует AssistantLLMClientResolver,
// оборачивая свежесозданный llm.Provider (с ключом пользователя) в llm.Client через
// ProviderAdapter.
// ─────────────────────────────────────────────────────────────────────────────

type AssistantLLMClientAdapter struct {
	credsSvc UserLlmCredentialService
	factory  *factory.Factory
}

// NewAssistantLLMClientAdapter — конструктор.
func NewAssistantLLMClientAdapter(credsSvc UserLlmCredentialService, f *factory.Factory) *AssistantLLMClientAdapter {
	return &AssistantLLMClientAdapter{credsSvc: credsSvc, factory: f}
}

// ResolveAssistantClient реализует AssistantLLMClientResolver.
func (r *AssistantLLMClientAdapter) ResolveAssistantClient(ctx context.Context, a *models.Agent, userID uuid.UUID) (llm.Client, error) {
	if r == nil || r.credsSvc == nil || r.factory == nil {
		return nil, errors.New("AssistantLLMClientAdapter: not configured")
	}

	if a.ProviderKind == nil || !a.ProviderKind.IsValid() {
		return nil, errors.New("assistant agent has no valid provider_kind configured")
	}

	provKind := *a.ProviderKind
	userProvider := provKind.UserLLMProvider()
	if userProvider == "" {
		return nil, fmt.Errorf("provider kind %q has no user credential mapping", provKind)
	}

	key, err := r.credsSvc.GetPlaintext(ctx, userID, userProvider)
	if err != nil {
		if errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
			return nil, ErrAssistantNotConfiguredForUser
		}
		return nil, fmt.Errorf("failed to fetch user llm credential: %w", err)
	}
	if key == "" {
		return nil, ErrAssistantNotConfiguredForUser
	}

	provider, err := r.factory.CreateProvider(llm.ProviderType(provKind), llm.Config{
		APIKey: key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create llm provider: %w", err)
	}

	return &llm.ProviderAdapter{Provider: provider}, nil
}

// Compile-time проверки соответствия интерфейсам.
var (
	_ LLMProviderResolver         = (*SingletonLLMProviderResolver)(nil)
	_ SandboxExecutorFactory      = (*SingletonSandboxExecutorFactory)(nil)
	_ AgentLoader                 = (*DBAgentLoader)(nil)
	_ AssistantLLMClientResolver  = (*AssistantLLMClientAdapter)(nil)
)

// Ensure uuid import is used (silences linters if file evolves).
var _ = uuid.Nil
