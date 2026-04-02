package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
	"gorm.io/datatypes"
)

type ModelCatalogService interface {
	SyncOpenRouterModels(ctx context.Context) error
	ListModels(ctx context.Context) ([]models.LLMModel, error)
}

type modelCatalogService struct {
	repo   repository.LLMModelRepository
	apiKey string
}

func NewModelCatalogService(repo repository.LLMModelRepository, apiKey string) ModelCatalogService {
	return &modelCatalogService{
		repo:   repo,
		apiKey: apiKey,
	}
}

func (s *modelCatalogService) ListModels(ctx context.Context) ([]models.LLMModel, error) {
	// Возвращаем только активные модели
	return s.repo.ListActive(ctx)
}

func (s *modelCatalogService) SyncOpenRouterModels(ctx context.Context) error {
	if s.apiKey == "" {
		log.Println("Skipping OpenRouter sync: API Key is empty")
		return nil
	}

	url := "https://openrouter.ai/api/v1/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch models from openrouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openrouter api returned status: %d", resp.StatusCode)
	}

	var orResp openRouterListResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return fmt.Errorf("failed to decode openrouter response: %w", err)
	}

	var modelsToSave []models.LLMModel
	for _, data := range orResp.Data {
		archBytes, _ := json.Marshal(data.Architecture)
		
		m := models.LLMModel{
			ID:            data.ID,
			Name:          data.Name,
			Description:   data.Description,
			ContextLength: int(data.ContextLength),
			Architecture:  datatypes.JSON(archBytes),
			
			PricingPrompt:     parsePrice(data.Pricing.Prompt),
			PricingCompletion: parsePrice(data.Pricing.Completion),
			PricingRequest:    parsePrice(data.Pricing.Request),
			PricingImage:      parsePrice(data.Pricing.Image),
			
			IsActive:  true, // По умолчанию новые модели активны
			UpdatedAt: time.Now(),
		}
		// CreatedAt проставит GORM при создании, а при обновлении не тронет
		modelsToSave = append(modelsToSave, m)
	}

	if err := s.repo.Upsert(ctx, modelsToSave); err != nil {
		return fmt.Errorf("failed to upsert models: %w", err)
	}

	log.Printf("Successfully synced %d models from OpenRouter", len(modelsToSave))
	return nil
}

// --- OpenRouter DTOs ---

type openRouterListResponse struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	ContextLength float64            `json:"context_length"` // Может быть float
	Architecture  any                `json:"architecture"`   // Сохраняем как JSONB
	Pricing       openRouterPricing  `json:"pricing"`
}

type openRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

func parsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	// OpenRouter может возвращать "-1" для unknown? Обычно это строки типа "0.000001"
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if val < 0 {
		return 0
	}
	return val
}

