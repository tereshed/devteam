package repository

import (
	"context"
	"errors"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LLMProviderRepository — доступ к таблице llm_providers (Sprint 15.10).
type LLMProviderRepository interface {
	Create(ctx context.Context, p *models.LLMProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.LLMProvider, error)
	GetByName(ctx context.Context, name string) (*models.LLMProvider, error)
	List(ctx context.Context, onlyEnabled bool) ([]models.LLMProvider, error)
	Update(ctx context.Context, p *models.LLMProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type llmProviderRepository struct {
	db *gorm.DB
}

func NewLLMProviderRepository(db *gorm.DB) LLMProviderRepository {
	return &llmProviderRepository{db: db}
}

func (r *llmProviderRepository) Create(ctx context.Context, p *models.LLMProvider) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		if IsPostgresUniqueViolation(err) {
			return ErrLLMProviderNameExists
		}
		return err
	}
	return nil
}

func (r *llmProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.LLMProvider, error) {
	var p models.LLMProvider
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLLMProviderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *llmProviderRepository) GetByName(ctx context.Context, name string) (*models.LLMProvider, error) {
	var p models.LLMProvider
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLLMProviderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *llmProviderRepository) List(ctx context.Context, onlyEnabled bool) ([]models.LLMProvider, error) {
	q := r.db.WithContext(ctx)
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	var items []models.LLMProvider
	if err := q.Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *llmProviderRepository) Update(ctx context.Context, p *models.LLMProvider) error {
	res := r.db.WithContext(ctx).Save(p)
	if err := res.Error; err != nil {
		if IsPostgresUniqueViolation(err) {
			return ErrLLMProviderNameExists
		}
		return err
	}
	if res.RowsAffected == 0 {
		return ErrLLMProviderNotFound
	}
	return nil
}

func (r *llmProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.LLMProvider{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrLLMProviderNotFound
	}
	return nil
}
