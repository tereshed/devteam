package repository

import (
	"context"

	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LLMModelRepository interface {
	Upsert(ctx context.Context, models []models.LLMModel) error
	ListActive(ctx context.Context) ([]models.LLMModel, error)
	ListAll(ctx context.Context) ([]models.LLMModel, error)
	GetByID(ctx context.Context, id string) (*models.LLMModel, error)
}

type llmModelRepository struct {
	db *gorm.DB
}

func NewLLMModelRepository(db *gorm.DB) LLMModelRepository {
	return &llmModelRepository{db: db}
}

func (r *llmModelRepository) Upsert(ctx context.Context, list []models.LLMModel) error {
	if len(list) == 0 {
		return nil
	}
	
	// Используем GORM Upsert (OnConflict)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "description", "context_length", "architecture",
			"pricing_prompt", "pricing_completion", "pricing_request", "pricing_image",
			"updated_at",
		}),
	}).Create(&list).Error
}

func (r *llmModelRepository) ListActive(ctx context.Context) ([]models.LLMModel, error) {
	var items []models.LLMModel
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("name ASC").Find(&items).Error
	return items, err
}

func (r *llmModelRepository) ListAll(ctx context.Context) ([]models.LLMModel, error) {
	var items []models.LLMModel
	err := r.db.WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}

func (r *llmModelRepository) GetByID(ctx context.Context, id string) (*models.LLMModel, error) {
	var item models.LLMModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	return &item, err
}

