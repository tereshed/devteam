package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/config"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/factory"
)

// LLMService defines the interface for LLM operations
type LLMService interface {
	Generate(ctx context.Context, req llm.Request) (*llm.Response, error)
	ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error)
}

type llmService struct {
	providers       map[llm.ProviderType]llm.Provider
	defaultProvider llm.ProviderType
	defaultModels   map[llm.ProviderType]string
	repo            repository.LLMRepository
	modelRepo       repository.LLMModelRepository
}

func NewLLMService(factory *factory.Factory, cfg config.LLMConfig, repo repository.LLMRepository, modelRepo repository.LLMModelRepository) LLMService {
	providers := make(map[llm.ProviderType]llm.Provider)
	defaultModels := make(map[llm.ProviderType]string)

	// Helper to create and register provider
	createProvider := func(pType llm.ProviderType, pCfg config.ProviderConfig) {
		if pCfg.APIKey != "" {
			provider, err := factory.CreateProvider(pType, llm.Config{
				APIKey:  pCfg.APIKey,
				BaseURL: pCfg.BaseURL,
			})
			if err == nil {
				providers[pType] = provider
				defaultModels[pType] = pCfg.Model
			}
		}
	}

	createProvider(llm.ProviderOpenAI, cfg.OpenAI)
	createProvider(llm.ProviderAnthropic, cfg.Anthropic)
	createProvider(llm.ProviderGemini, cfg.Gemini)
	createProvider(llm.ProviderDeepseek, cfg.Deepseek)
	createProvider(llm.ProviderQwen, cfg.Qwen)

	return &llmService{
		providers:       providers,
		defaultProvider: llm.ProviderType(cfg.DefaultProvider),
		defaultModels:   defaultModels,
		repo:            repo,
		modelRepo:       modelRepo,
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

	provider, ok := s.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", providerType)
	}

	startTime := time.Now()
	resp, err := provider.Generate(ctx, req)
	duration := time.Since(startTime)

	// Logging (async to not block response)
	go func() {
		// Create a detached context for logging
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logEntry := &models.LLMLog{
			Provider:    string(providerType),
			Model:       modelUsed,
			DurationMs:  int(duration.Milliseconds()),
			CreatedAt:   startTime,
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
