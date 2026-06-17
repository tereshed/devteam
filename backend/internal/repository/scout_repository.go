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

var (
	// ErrScoutConfigNotFound — конфиг разведчика не найден.
	ErrScoutConfigNotFound = errors.New("scout config not found")
	// ErrScoutRunNotFound — прогон разведчика не найден.
	ErrScoutRunNotFound = errors.New("scout run not found")
)

// ScoutRepository — доступ к scout_configs / scout_runs.
type ScoutRepository interface {
	// Конфиг (одна строка на проект).
	GetConfigByProjectID(ctx context.Context, projectID uuid.UUID) (*models.ScoutConfig, error)
	CreateConfig(ctx context.Context, cfg *models.ScoutConfig) error
	UpdateConfig(ctx context.Context, cfg *models.ScoutConfig) error

	// Прогоны.
	CreateRun(ctx context.Context, run *models.ScoutRun) error
	GetRunByID(ctx context.Context, id uuid.UUID) (*models.ScoutRun, error)
	UpdateRun(ctx context.Context, run *models.ScoutRun) error
	ListRunsByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]models.ScoutRun, error)
}

type scoutRepository struct {
	db *gorm.DB
}

// NewScoutRepository создаёт репозиторий разведчика.
func NewScoutRepository(db *gorm.DB) ScoutRepository {
	return &scoutRepository{db: db}
}

func (r *scoutRepository) GetConfigByProjectID(ctx context.Context, projectID uuid.UUID) (*models.ScoutConfig, error) {
	db := gormDB(ctx, r.db)
	var cfg models.ScoutConfig
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoutConfigNotFound
		}
		return nil, fmt.Errorf("failed to get scout config: %w", err)
	}
	return &cfg, nil
}

func (r *scoutRepository) CreateConfig(ctx context.Context, cfg *models.ScoutConfig) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("failed to create scout config: %w", err)
	}
	return nil
}

func (r *scoutRepository) UpdateConfig(ctx context.Context, cfg *models.ScoutConfig) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.ScoutConfig{}).
		Where("id = ?", cfg.ID).
		Updates(map[string]any{
			"is_enabled":            cfg.IsEnabled,
			"prompt":                cfg.Prompt,
			"code_backend":          cfg.CodeBackend,
			"provider_kind":         cfg.ProviderKind,
			"temperature":           cfg.Temperature,
			"code_backend_settings": cfg.CodeBackendSettings,
			"sandbox_permissions":   cfg.SandboxPermissions,
			"subscription_id":       cfg.SubscriptionID,
			"timeout_seconds":       cfg.TimeoutSeconds,
			"updated_at":            time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update scout config: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrScoutConfigNotFound
	}
	return nil
}

func (r *scoutRepository) CreateRun(ctx context.Context, run *models.ScoutRun) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("failed to create scout run: %w", err)
	}
	return nil
}

func (r *scoutRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*models.ScoutRun, error) {
	db := gormDB(ctx, r.db)
	var run models.ScoutRun
	if err := db.WithContext(ctx).Where("id = ?", id).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoutRunNotFound
		}
		return nil, fmt.Errorf("failed to get scout run: %w", err)
	}
	return &run, nil
}

func (r *scoutRepository) UpdateRun(ctx context.Context, run *models.ScoutRun) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.ScoutRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]any{
			"status":              run.Status,
			"dossier":             run.Dossier,
			"error":               run.Error,
			"sandbox_instance_id": run.SandboxInstanceID,
			"finished_at":         run.FinishedAt,
			"updated_at":          time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update scout run: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrScoutRunNotFound
	}
	return nil
}

func (r *scoutRepository) ListRunsByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]models.ScoutRun, error) {
	if limit <= 0 {
		limit = 50
	}
	db := gormDB(ctx, r.db)
	var items []models.ScoutRun
	if err := db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("started_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list scout runs: %w", err)
	}
	return items, nil
}
