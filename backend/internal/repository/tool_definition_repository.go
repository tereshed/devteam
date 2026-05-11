package repository

import (
	"context"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ToolDefinitionRepository — чтение реестра инструментов.
type ToolDefinitionRepository interface {
	ListActiveCatalog(ctx context.Context) ([]models.ToolDefinition, error)
	CountActiveInIDs(ctx context.Context, ids []uuid.UUID) (int64, error)
}

type toolDefinitionRepository struct {
	db *gorm.DB
}

// NewToolDefinitionRepository создаёт репозиторий tool_definitions.
func NewToolDefinitionRepository(db *gorm.DB) ToolDefinitionRepository {
	return &toolDefinitionRepository{db: db}
}

func (r *toolDefinitionRepository) ListActiveCatalog(ctx context.Context) ([]models.ToolDefinition, error) {
	var list []models.ToolDefinition
	err := r.db.WithContext(ctx).
		Model(&models.ToolDefinition{}).
		Where("is_active = ?", true).
		Order("category ASC, name ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list tool definitions: %w", err)
	}
	return list, nil
}

func (r *toolDefinitionRepository) CountActiveInIDs(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var n int64
	err := r.db.WithContext(ctx).
		Model(&models.ToolDefinition{}).
		Where("id IN ? AND is_active = ?", ids, true).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count active tool definitions: %w", err)
	}
	return n, nil
}
