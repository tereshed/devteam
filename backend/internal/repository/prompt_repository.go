package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PromptRepository интерфейс для работы с промптами
type PromptRepository interface {
	Create(ctx context.Context, prompt *models.Prompt) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error)
	GetByName(ctx context.Context, name string) (*models.Prompt, error)
	List(ctx context.Context) ([]models.Prompt, error)
	Update(ctx context.Context, prompt *models.Prompt) error
	Delete(ctx context.Context, id uuid.UUID) error
	Upsert(ctx context.Context, prompt *models.Prompt) error
}

type promptRepository struct {
	db *gorm.DB
}

// NewPromptRepository создает новый репозиторий
func NewPromptRepository(db *gorm.DB) PromptRepository {
	return &promptRepository{db: db}
}

func (r *promptRepository) Create(ctx context.Context, prompt *models.Prompt) error {
	if err := r.db.WithContext(ctx).Create(prompt).Error; err != nil {
		return fmt.Errorf("failed to create prompt: %w", err)
	}
	return nil
}

func (r *promptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error) {
	var prompt models.Prompt
	if err := r.db.WithContext(ctx).First(&prompt, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (r *promptRepository) GetByName(ctx context.Context, name string) (*models.Prompt, error) {
	var prompt models.Prompt
	if err := r.db.WithContext(ctx).First(&prompt, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (r *promptRepository) List(ctx context.Context) ([]models.Prompt, error) {
	var prompts []models.Prompt
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&prompts).Error; err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}
	return prompts, nil
}

func (r *promptRepository) Update(ctx context.Context, prompt *models.Prompt) error {
	if err := r.db.WithContext(ctx).Save(prompt).Error; err != nil {
		return fmt.Errorf("failed to update prompt: %w", err)
	}
	return nil
}

func (r *promptRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.Prompt{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete prompt: %w", err)
	}
	return nil
}

func (r *promptRepository) Upsert(ctx context.Context, prompt *models.Prompt) error {
	// Используем OnConflict для обновления существующих записей по имени
	// Если запись существует (по name), обновляем description, template, json_schema, is_active
	// ID не трогаем, чтобы сохранить ссылочную целостность (хотя он UUID, сгенерированный базой)

	// Важно: GORM требует, чтобы conflict target (name) был уникальным индексом (он у нас есть)

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "template", "json_schema", "is_active", "updated_at"}),
	}).Create(prompt).Error

	if err != nil {
		return fmt.Errorf("failed to upsert prompt: %w", err)
	}
	return nil
}
