package repository

import (
	"context"

	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/gorm"
)

type LLMRepository interface {
	CreateLog(ctx context.Context, log *models.LLMLog) error
	ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error)
}

type llmRepository struct {
	db *gorm.DB
}

func NewLLMRepository(db *gorm.DB) LLMRepository {
	return &llmRepository{db: db}
}

func (r *llmRepository) CreateLog(ctx context.Context, log *models.LLMLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *llmRepository) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	var logs []models.LLMLog
	var count int64

	db := r.db.WithContext(ctx).Model(&models.LLMLog{})
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, count, nil
}

