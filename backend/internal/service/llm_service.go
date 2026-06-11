package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/google/uuid"
)

// LLMService defines the interface for LLM operations
type LLMService interface {
	Generate(ctx context.Context, req llm.Request) (*llm.Response, error)
	ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error)
}

// userLLMKeyReader — минимальный срез UserLlmCredentialService для per-user
// ключей провайдеров (блок «Ключи» в UI). nil → поведение как раньше (env-only).
type userLLMKeyReader interface {
	GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error)
}

type llmService struct {
	providers       map[llm.ProviderType]llm.Provider
	defaultProvider llm.ProviderType
	defaultModels   map[llm.ProviderType]string
	repo            repository.LLMRepository
	modelRepo       repository.LLMModelRepository

	// Per-user ключи (user_llm_credentials) приоритетнее env: ключ из блока
	// пользователя в UI должен действовать на ВСЕ LLM-вызовы его проектов
	// (router/planner/decomposer/...), а не только на sandbox-агентов.
	factory  *factory.Factory
	cfg      config.LLMConfig
	userKeys userLLMKeyReader
	// userProviders — кэш per-user провайдеров, ключ = "<kind>:<sha256(key)[:8]>".
	// Ротация ключа в UI меняет hash → новый инстанс; старые записи ограничены
	// числом ротаций, eviction не нужен.
	mu            sync.RWMutex
	userProviders map[string]llm.Provider
}

func NewLLMService(llmFactory *factory.Factory, cfg config.LLMConfig, repo repository.LLMRepository, modelRepo repository.LLMModelRepository, userKeys userLLMKeyReader) LLMService {
	providers := make(map[llm.ProviderType]llm.Provider)
	defaultModels := make(map[llm.ProviderType]string)

	// Helper to create and register provider
	createProvider := func(pType llm.ProviderType, pCfg config.ProviderConfig) {
		// Дефолтная модель нужна и без env-ключа: провайдер может быть собран
		// per-user из user_llm_credentials, а модель у агента не задана.
		defaultModels[pType] = pCfg.Model
		if pCfg.APIKey != "" {
			provider, err := llmFactory.CreateProvider(pType, llm.Config{
				APIKey:  pCfg.APIKey,
				BaseURL: pCfg.BaseURL,
			})
			if err == nil {
				providers[pType] = provider
			}
		}
	}

	createProvider(llm.ProviderOpenAI, cfg.OpenAI)
	createProvider(llm.ProviderAnthropic, cfg.Anthropic)
	createProvider(llm.ProviderGemini, cfg.Gemini)
	createProvider(llm.ProviderDeepseek, cfg.Deepseek)
	createProvider(llm.ProviderQwen, cfg.Qwen)
	// OpenRouter — глобальный провайдер для assistant/orchestrator/planner
	// (см. Phase 5 review). Без этой регистрации `req.Provider="openrouter"`
	// падал с «unsupported provider», даже если OPENROUTER_API_KEY был в env.
	createProvider(llm.ProviderOpenRouter, cfg.OpenRouter)
	createProvider(llm.ProviderZhipu, cfg.Zhipu)
	createProvider(llm.ProviderAntigravity, cfg.Antigravity)

	return &llmService{
		providers:       providers,
		defaultProvider: llm.ProviderType(cfg.DefaultProvider),
		defaultModels:   defaultModels,
		repo:            repo,
		modelRepo:       modelRepo,
		factory:         llmFactory,
		cfg:             cfg,
		userKeys:        userKeys,
		userProviders:   make(map[string]llm.Provider),
	}
}

// resolveProvider — выбор backend'а для запроса: ключ владельца проекта из
// user_llm_credentials приоритетнее env-провайдера. Семантика ошибок:
//   - ключа у пользователя нет (ErrUserLlmCredentialNotFound) → штатный fallback на env;
//   - ключ есть, но не читается (decrypt/db) → ошибка, БЕЗ тихого отката на env:
//     откат маскировал бы проблему ключа пользователя (см. постмортем e3c06668 про fail-silent).
func (s *llmService) resolveProvider(ctx context.Context, pt llm.ProviderType, ownerUserID string) (llm.Provider, error) {
	if ownerUserID != "" && s.userKeys != nil && models.IsValidUserLLMProvider(string(pt)) {
		uid, parseErr := uuid.Parse(ownerUserID)
		if parseErr == nil {
			key, kerr := s.userKeys.GetPlaintext(ctx, uid, models.UserLLMProvider(pt))
			switch {
			case kerr == nil && key != "":
				return s.userScopedProvider(pt, key)
			case kerr != nil && !errors.Is(kerr, repository.ErrUserLlmCredentialNotFound):
				return nil, fmt.Errorf("user llm credential (%s, user=%s): %w", pt, uid, kerr)
			}
		}
	}
	provider, ok := s.providers[pt]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", pt)
	}
	return provider, nil
}

func (s *llmService) userScopedProvider(pt llm.ProviderType, key string) (llm.Provider, error) {
	sum := sha256.Sum256([]byte(key))
	cacheKey := string(pt) + ":" + hex.EncodeToString(sum[:8])

	s.mu.RLock()
	cached := s.userProviders[cacheKey]
	s.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	created, err := s.factory.CreateProvider(pt, llm.Config{
		APIKey:  key,
		BaseURL: s.baseURLFor(pt),
	})
	if err != nil {
		return nil, fmt.Errorf("create user-scoped provider %s: %w", pt, err)
	}

	s.mu.Lock()
	s.userProviders[cacheKey] = created
	s.mu.Unlock()
	return created, nil
}

// baseURLFor — BaseURL провайдера из конфига процесса: per-user меняется только
// ключ, эндпоинт остаётся общим (включая локальные override'ы вроде staging-прокси).
func (s *llmService) baseURLFor(pt llm.ProviderType) string {
	switch pt {
	case llm.ProviderOpenAI:
		return s.cfg.OpenAI.BaseURL
	case llm.ProviderAnthropic:
		return s.cfg.Anthropic.BaseURL
	case llm.ProviderGemini:
		return s.cfg.Gemini.BaseURL
	case llm.ProviderDeepseek:
		return s.cfg.Deepseek.BaseURL
	case llm.ProviderQwen:
		return s.cfg.Qwen.BaseURL
	case llm.ProviderOpenRouter:
		return s.cfg.OpenRouter.BaseURL
	case llm.ProviderZhipu:
		return s.cfg.Zhipu.BaseURL
	case llm.ProviderAntigravity:
		return s.cfg.Antigravity.BaseURL
	default:
		return ""
	}
}

func (s *llmService) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	return s.repo.ListLogs(ctx, limit, offset)
}

func (s *llmService) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	providerType := req.Provider
	if providerType == "" {
		providerType = s.defaultProvider
	}

	// Determine model used (if empty, use default from config)
	modelUsed := req.Model
	if modelUsed == "" {
		modelUsed = s.defaultModels[providerType]
	}

	provider, err := s.resolveProvider(ctx, providerType, req.OwnerUserID)
	if err != nil {
		return nil, err
	}

	req.Provider = providerType
	req.Model = modelUsed

	startTime := time.Now()
	resp, err := provider.Generate(ctx, req)
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
			// Try to find model in DB to get pricing
			modelID := modelUsed
			// Try exact match first
			model, err := s.modelRepo.GetByID(logCtx, modelID)
			if err != nil {
				// Try constructing ID: "provider/model"
				// Note: providerType might be "openai", but OpenRouter ID is "openai/gpt-4o"
				// If providerType is "openrouter", checking "openrouter/gpt-4o" might not work if the ID is "openai/gpt-4o".
				// But usually if we use OpenRouter, we pass the full model ID in req.Model?
				// If we use standard OpenAI client, req.Model is "gpt-4o".
				// Let's try prefixing provider.
				fullID := fmt.Sprintf("%s/%s", providerType, modelID)
				model, err = s.modelRepo.GetByID(logCtx, fullID)
			}

			if err == nil && model != nil {
				// Cost = (Input * PromptPrice) + (Output * CompletionPrice) + RequestPrice
				cost := (float64(logEntry.InputTokens) * model.PricingPrompt) +
					(float64(logEntry.OutputTokens) * model.PricingCompletion) +
					model.PricingRequest
				logEntry.Cost = cost
			}
		}

		if logErr := s.repo.CreateLog(logCtx, logEntry); logErr != nil {
			log.Printf("Failed to create LLM log: %v", logErr)
		}
	}()

	return resp, err
}
