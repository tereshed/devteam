package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/config"
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
	if err := l.db.WithContext(ctx).Where("name = ? AND user_id IS NULL AND team_id IS NULL", name).First(&a).Error; err != nil {
		return nil, fmt.Errorf("DBAgentLoader: load agent %q: %w", name, err)
	}
	return &a, nil
}

// GetAgentByTeamAndName implements AgentLoader interface.
func (l *DBAgentLoader) GetAgentByTeamAndName(ctx context.Context, teamID uuid.UUID, name string) (*models.Agent, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("DBAgentLoader: db is not configured")
	}
	var a models.Agent
	err := l.db.WithContext(ctx).Where("team_id = ? AND name = ?", teamID, name).First(&a).Error
	if err == nil {
		return &a, nil
	}
	// Fallback to global agent
	return l.GetAgentByName(ctx, name)
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

// UpdateAgentProvider updates the agent's provider kind and model.
func (l *DBAgentLoader) UpdateAgentProvider(ctx context.Context, agentID uuid.UUID, providerKind models.AgentProviderKind, model string) error {
	if l == nil || l.db == nil {
		return errors.New("DBAgentLoader: db is not configured")
	}
	return l.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", agentID).Updates(map[string]any{
		"provider_kind": providerKind,
		"model":         model,
	}).Error
}

// ─────────────────────────────────────────────────────────────────────────────
// AssistantLLMClientAdapter — Sprint 21. Реализует AssistantLLMClientResolver,
// оборачивая свежесозданный llm.Provider (с ключом пользователя) в llm.Client через
// ProviderAdapter.
// ─────────────────────────────────────────────────────────────────────────────

type AssistantLLMClientAdapter struct {
	credsSvc  UserLlmCredentialService
	factory   *factory.Factory
	cfg       config.LLMConfig
	repo      repository.LLMRepository
	modelRepo repository.LLMModelRepository
}

// NewAssistantLLMClientAdapter — конструктор.
func NewAssistantLLMClientAdapter(
	credsSvc UserLlmCredentialService,
	f *factory.Factory,
	cfg config.LLMConfig,
	repo repository.LLMRepository,
	modelRepo repository.LLMModelRepository,
) *AssistantLLMClientAdapter {
	return &AssistantLLMClientAdapter{
		credsSvc:  credsSvc,
		factory:   f,
		cfg:       cfg,
		repo:      repo,
		modelRepo: modelRepo,
	}
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

	var baseURL string
	switch provKind {
	case models.AgentProviderKindAnthropic, models.AgentProviderKindAnthropicOAuth:
		baseURL = r.cfg.Anthropic.BaseURL
	case models.AgentProviderKindDeepSeek:
		baseURL = r.cfg.Deepseek.BaseURL
	case models.AgentProviderKindZhipu:
		baseURL = r.cfg.Zhipu.BaseURL
	case models.AgentProviderKindOpenRouter:
		baseURL = r.cfg.OpenRouter.BaseURL
	}

	provider, err := r.factory.CreateProvider(llm.ProviderType(provKind), llm.Config{
		APIKey:  key,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create llm provider: %w", err)
	}

	client := &llm.ProviderAdapter{Provider: provider}

	return &loggingLLMClient{
		client:    client,
		repo:      r.repo,
		modelRepo: r.modelRepo,
	}, nil
}

type loggingLLMClient struct {
	client    llm.Client
	repo      repository.LLMRepository
	modelRepo repository.LLMModelRepository
}

var _ llm.Client = (*loggingLLMClient)(nil)

func (c *loggingLLMClient) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	if c.repo == nil {
		return c.client.Chat(ctx, req)
	}

	providerType := req.Provider
	modelUsed := req.Model

	startTime := time.Now()
	resp, err := c.client.Chat(ctx, req)
	duration := time.Since(startTime)

	// Logging (async to not block response)
	go func() {
		// Create a detached context for logging
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logEntry := &models.LLMLog{
			Provider:   string(providerType),
			Model:      modelUsed,
			DurationMs: int(duration.Milliseconds()),
			CreatedAt:  startTime,
		}

		// Extract metadata
		if req.Metadata != nil {
			if val, ok := req.Metadata["execution_id"].(string); ok {
				if id, err := uuid.Parse(val); err == nil {
					logEntry.WorkflowExecutionID = &id
				}
			}
			if val, ok := req.Metadata["agent_id"].(string); ok {
				if id, err := uuid.Parse(val); err == nil {
					logEntry.AgentID = &id
				}
			}
			if val, ok := req.Metadata["step_id"].(string); ok {
				logEntry.StepID = val
			}
		}

		// Snapshots
		promptJSON, _ := json.Marshal(req)
		logEntry.PromptSnapshot = string(promptJSON)

		if err != nil {
			logEntry.ErrorMessage = err.Error()
		} else {
			respJSON, _ := json.Marshal(resp)
			logEntry.ResponseSnapshot = string(respJSON)
			logEntry.InputTokens = resp.Usage.PromptTokens
			logEntry.OutputTokens = resp.Usage.CompletionTokens
			logEntry.TotalTokens = resp.Usage.TotalTokens

			// Calculate Cost
			if c.modelRepo != nil {
				modelID := modelUsed
				model, err := c.modelRepo.GetByID(logCtx, modelID)
				if err != nil {
					fullID := fmt.Sprintf("%s/%s", providerType, modelID)
					model, err = c.modelRepo.GetByID(logCtx, fullID)
				}

				if err == nil && model != nil {
					cost := (float64(logEntry.InputTokens) * model.PricingPrompt) +
						(float64(logEntry.OutputTokens) * model.PricingCompletion) +
						model.PricingRequest
					logEntry.Cost = cost
				}
			}
		}

		if logErr := c.repo.CreateLog(logCtx, logEntry); logErr != nil {
			log.Printf("Failed to create LLM log: %v", logErr)
		}
	}()

	return resp, err
}

func (c *loggingLLMClient) Embed(ctx context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return c.client.Embed(ctx, req)
}

func (c *loggingLLMClient) HealthCheck(ctx context.Context) error {
	return c.client.HealthCheck(ctx)
}

func (c *loggingLLMClient) ResolveBaseURL() string {
	return c.client.ResolveBaseURL()
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
