package repository

import (
	"context"
	"errors"

	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
)

// MCPServerRegistryRepository — read+lookup по mcp_servers_registry (Sprint 15.24).
// Минимальный набор для MCP-инструмента mcp_server_list и резолва имён в AgentSettingsService.
type MCPServerRegistryRepository interface {
	List(ctx context.Context, onlyActive bool) ([]models.MCPServerRegistry, error)
	GetByName(ctx context.Context, name string) (*models.MCPServerRegistry, error)
}

type mcpServerRegistryRepository struct{ db *gorm.DB }

func NewMCPServerRegistryRepository(db *gorm.DB) MCPServerRegistryRepository {
	return &mcpServerRegistryRepository{db: db}
}

func (r *mcpServerRegistryRepository) List(ctx context.Context, onlyActive bool) ([]models.MCPServerRegistry, error) {
	q := r.db.WithContext(ctx)
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}
	var items []models.MCPServerRegistry
	if err := q.Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *mcpServerRegistryRepository) GetByName(ctx context.Context, name string) (*models.MCPServerRegistry, error) {
	var s models.MCPServerRegistry
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMCPServerRegistryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
