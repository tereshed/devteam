package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrAssistantMCPServerNotFound — конфиг MCP-сервера ассистента не найден.
var ErrAssistantMCPServerNotFound = errors.New("assistant mcp server not found")

// AssistantMCPServerRepository — доступ к assistant_mcp_servers (per-project MCP
// для in-process петли ассистента).
type AssistantMCPServerRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error)
	// ListEnabledByProject — только включённые (для сборки каталога инструментов).
	ListEnabledByProject(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AssistantMCPServer, error)
	Create(ctx context.Context, cfg *models.AssistantMCPServer) error
	Update(ctx context.Context, cfg *models.AssistantMCPServer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type assistantMCPServerRepository struct {
	db *gorm.DB
}

// NewAssistantMCPServerRepository создаёт репозиторий MCP-серверов ассистента.
func NewAssistantMCPServerRepository(db *gorm.DB) AssistantMCPServerRepository {
	return &assistantMCPServerRepository{db: db}
}

func (r *assistantMCPServerRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error) {
	db := gormDB(ctx, r.db)
	var items []models.AssistantMCPServer
	if err := db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assistant mcp servers: %w", err)
	}
	return items, nil
}

func (r *assistantMCPServerRepository) ListEnabledByProject(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error) {
	db := gormDB(ctx, r.db)
	var items []models.AssistantMCPServer
	if err := db.WithContext(ctx).
		Where("project_id = ? AND is_enabled = ?", projectID, true).
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list enabled assistant mcp servers: %w", err)
	}
	return items, nil
}

func (r *assistantMCPServerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AssistantMCPServer, error) {
	db := gormDB(ctx, r.db)
	var cfg models.AssistantMCPServer
	if err := db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssistantMCPServerNotFound
		}
		return nil, fmt.Errorf("failed to get assistant mcp server: %w", err)
	}
	return &cfg, nil
}

func (r *assistantMCPServerRepository) Create(ctx context.Context, cfg *models.AssistantMCPServer) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("failed to create assistant mcp server: %w", err)
	}
	return nil
}

func (r *assistantMCPServerRepository) Update(ctx context.Context, cfg *models.AssistantMCPServer) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.AssistantMCPServer{}).
		Where("id = ?", cfg.ID).
		Updates(map[string]any{
			"name":                 cfg.Name,
			"transport":            cfg.Transport,
			"url":                  cfg.URL,
			"headers":              cfg.Headers,
			"require_confirmation": cfg.RequireConfirmation,
			"is_enabled":           cfg.IsEnabled,
			"updated_at":           time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update assistant mcp server: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrAssistantMCPServerNotFound
	}
	return nil
}

func (r *assistantMCPServerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&models.AssistantMCPServer{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete assistant mcp server: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrAssistantMCPServerNotFound
	}
	return nil
}
