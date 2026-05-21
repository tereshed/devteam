package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPServerRegistryRepository — CRUD по mcp_servers_registry.
type MCPServerRegistryRepository interface {
	List(ctx context.Context, onlyActive bool) ([]models.MCPServerRegistry, error)
	GetByName(ctx context.Context, name string) (*models.MCPServerRegistry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.MCPServerRegistry, error)
	Create(ctx context.Context, srv *models.MCPServerRegistry) error
	Update(ctx context.Context, srv *models.MCPServerRegistry) error
	Delete(ctx context.Context, id uuid.UUID) error
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

func (r *mcpServerRegistryRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MCPServerRegistry, error) {
	var s models.MCPServerRegistry
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMCPServerRegistryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get mcp server %s: %w", id, err)
	}
	return &s, nil
}

func (r *mcpServerRegistryRepository) Create(ctx context.Context, srv *models.MCPServerRegistry) error {
	if err := r.db.WithContext(ctx).Create(srv).Error; err != nil {
		return fmt.Errorf("create mcp server: %w", err)
	}
	return nil
}

func (r *mcpServerRegistryRepository) Update(ctx context.Context, srv *models.MCPServerRegistry) error {
	if err := r.db.WithContext(ctx).Save(srv).Error; err != nil {
		return fmt.Errorf("update mcp server %s: %w", srv.ID, err)
	}
	return nil
}

func (r *mcpServerRegistryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&models.MCPServerRegistry{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		return fmt.Errorf("soft-delete mcp server %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrMCPServerRegistryNotFound
	}
	return nil
}
