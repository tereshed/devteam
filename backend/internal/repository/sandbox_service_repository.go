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

// ErrSandboxServiceConfigNotFound — конфиг сервис-сайдкара не найден.
var ErrSandboxServiceConfigNotFound = errors.New("sandbox service config not found")

// SandboxServiceRepository — доступ к sandbox_service_configs (Sprint 22).
type SandboxServiceRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.SandboxServiceConfig, error)
	// ListEnabledByProject — только включённые сервисы (для диспатча прогона).
	ListEnabledByProject(ctx context.Context, projectID uuid.UUID) ([]models.SandboxServiceConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SandboxServiceConfig, error)
	GetByProjectAndAlias(ctx context.Context, projectID uuid.UUID, alias string) (*models.SandboxServiceConfig, error)
	Create(ctx context.Context, cfg *models.SandboxServiceConfig) error
	Update(ctx context.Context, cfg *models.SandboxServiceConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type sandboxServiceRepository struct {
	db *gorm.DB
}

// NewSandboxServiceRepository создаёт репозиторий конфигов сервис-сайдкаров.
func NewSandboxServiceRepository(db *gorm.DB) SandboxServiceRepository {
	return &sandboxServiceRepository{db: db}
}

func (r *sandboxServiceRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.SandboxServiceConfig, error) {
	db := gormDB(ctx, r.db)
	var items []models.SandboxServiceConfig
	if err := db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("alias ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list sandbox service configs: %w", err)
	}
	return items, nil
}

func (r *sandboxServiceRepository) ListEnabledByProject(ctx context.Context, projectID uuid.UUID) ([]models.SandboxServiceConfig, error) {
	db := gormDB(ctx, r.db)
	var items []models.SandboxServiceConfig
	if err := db.WithContext(ctx).
		Where("project_id = ? AND is_enabled = ?", projectID, true).
		Order("alias ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list enabled sandbox service configs: %w", err)
	}
	return items, nil
}

func (r *sandboxServiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SandboxServiceConfig, error) {
	db := gormDB(ctx, r.db)
	var cfg models.SandboxServiceConfig
	if err := db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSandboxServiceConfigNotFound
		}
		return nil, fmt.Errorf("failed to get sandbox service config: %w", err)
	}
	return &cfg, nil
}

func (r *sandboxServiceRepository) GetByProjectAndAlias(ctx context.Context, projectID uuid.UUID, alias string) (*models.SandboxServiceConfig, error) {
	db := gormDB(ctx, r.db)
	var cfg models.SandboxServiceConfig
	if err := db.WithContext(ctx).
		Where("project_id = ? AND alias = ?", projectID, alias).
		First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSandboxServiceConfigNotFound
		}
		return nil, fmt.Errorf("failed to get sandbox service config: %w", err)
	}
	return &cfg, nil
}

func (r *sandboxServiceRepository) Create(ctx context.Context, cfg *models.SandboxServiceConfig) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("failed to create sandbox service config: %w", err)
	}
	return nil
}

func (r *sandboxServiceRepository) Update(ctx context.Context, cfg *models.SandboxServiceConfig) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.SandboxServiceConfig{}).
		Where("id = ?", cfg.ID).
		Updates(map[string]any{
			"is_enabled":            cfg.IsEnabled,
			"kind":                  cfg.Kind,
			"alias":                 cfg.Alias,
			"image":                 cfg.Image,
			"db_name":               cfg.DBName,
			"db_user":               cfg.DBUser,
			"port":                  cfg.Port,
			"seed_kind":             cfg.SeedKind,
			"seed_value":            cfg.SeedValue,
			"ready_timeout_seconds": cfg.ReadyTimeoutSeconds,
			"updated_at":            time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update sandbox service config: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrSandboxServiceConfigNotFound
	}
	return nil
}

func (r *sandboxServiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&models.SandboxServiceConfig{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete sandbox service config: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrSandboxServiceConfigNotFound
	}
	return nil
}
