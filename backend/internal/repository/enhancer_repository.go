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
	// ErrEnhancerConfigNotFound — конфиг энхансера не найден.
	ErrEnhancerConfigNotFound = errors.New("enhancer config not found")
	// ErrEnhancerRunNotFound — прогон энхансера не найден.
	ErrEnhancerRunNotFound = errors.New("enhancer run not found")
)

// EnhancerRepository — доступ к enhancer_configs / enhancer_runs / enhancer_changes.
type EnhancerRepository interface {
	// Конфиг (одна строка на проект).
	GetConfigByProjectID(ctx context.Context, projectID uuid.UUID) (*models.EnhancerConfig, error)
	CreateConfig(ctx context.Context, cfg *models.EnhancerConfig) error
	UpdateConfig(ctx context.Context, cfg *models.EnhancerConfig) error
	// ListDueConfigs — активные конфиги с next_run_at <= now (для раннера).
	// limit==0 → дефолт 100.
	ListDueConfigs(ctx context.Context, now time.Time, limit int) ([]models.EnhancerConfig, error)

	// Прогоны.
	CreateRun(ctx context.Context, run *models.EnhancerRun) error
	GetRunByID(ctx context.Context, id uuid.UUID) (*models.EnhancerRun, error)
	UpdateRun(ctx context.Context, run *models.EnhancerRun) error
	ListRunsByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]models.EnhancerRun, error)
	// HasRunningRun — есть ли незавершённый прогон проекта моложе staleAfter.
	// Прогоны старше staleAfter помечаются failed (краш-восстановление: иначе
	// зависший 'running' навсегда заблокировал бы новые прогоны).
	HasRunningRun(ctx context.Context, projectID uuid.UUID, staleAfter time.Duration) (bool, error)

	// Предложения.
	CreateChange(ctx context.Context, change *models.EnhancerChange) error
	CountChangesByRunID(ctx context.Context, runID uuid.UUID) (int64, error)
	ListChangesByRunID(ctx context.Context, runID uuid.UUID) ([]models.EnhancerChange, error)
}

type enhancerRepository struct {
	db *gorm.DB
}

// NewEnhancerRepository создаёт репозиторий энхансера.
func NewEnhancerRepository(db *gorm.DB) EnhancerRepository {
	return &enhancerRepository{db: db}
}

// ─────────────────────────────────────────────────────────────────────────────
// Конфиг.
// ─────────────────────────────────────────────────────────────────────────────

func (r *enhancerRepository) GetConfigByProjectID(ctx context.Context, projectID uuid.UUID) (*models.EnhancerConfig, error) {
	db := gormDB(ctx, r.db)
	var cfg models.EnhancerConfig
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEnhancerConfigNotFound
		}
		return nil, fmt.Errorf("failed to get enhancer config: %w", err)
	}
	return &cfg, nil
}

func (r *enhancerRepository) CreateConfig(ctx context.Context, cfg *models.EnhancerConfig) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("failed to create enhancer config: %w", err)
	}
	return nil
}

func (r *enhancerRepository) UpdateConfig(ctx context.Context, cfg *models.EnhancerConfig) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.EnhancerConfig{}).
		Where("id = ?", cfg.ID).
		Updates(map[string]any{
			"is_active":            cfg.IsActive,
			"autonomy":             cfg.Autonomy,
			"cron_expression":      cfg.CronExpression,
			"analysis_window_days": cfg.AnalysisWindowDays,
			"max_changes_per_run":  cfg.MaxChangesPerRun,
			"last_run_at":          cfg.LastRunAt,
			"next_run_at":          cfg.NextRunAt,
			"updated_at":           time.Now(),
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update enhancer config: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrEnhancerConfigNotFound
	}
	return nil
}

func (r *enhancerRepository) ListDueConfigs(ctx context.Context, now time.Time, limit int) ([]models.EnhancerConfig, error) {
	if limit <= 0 {
		limit = 100
	}
	db := gormDB(ctx, r.db)
	var items []models.EnhancerConfig
	if err := db.WithContext(ctx).
		Where("is_active = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list due enhancer configs: %w", err)
	}
	return items, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Прогоны.
// ─────────────────────────────────────────────────────────────────────────────

func (r *enhancerRepository) CreateRun(ctx context.Context, run *models.EnhancerRun) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(run).Error; err != nil {
		return fmt.Errorf("failed to create enhancer run: %w", err)
	}
	return nil
}

func (r *enhancerRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*models.EnhancerRun, error) {
	db := gormDB(ctx, r.db)
	var run models.EnhancerRun
	if err := db.WithContext(ctx).Where("id = ?", id).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEnhancerRunNotFound
		}
		return nil, fmt.Errorf("failed to get enhancer run: %w", err)
	}
	return &run, nil
}

func (r *enhancerRepository) UpdateRun(ctx context.Context, run *models.EnhancerRun) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.EnhancerRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]any{
			"status":      run.Status,
			"report":      run.Report,
			"error":       run.Error,
			"finished_at": run.FinishedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update enhancer run: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrEnhancerRunNotFound
	}
	return nil
}

func (r *enhancerRepository) ListRunsByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]models.EnhancerRun, error) {
	if limit <= 0 {
		limit = 20
	}
	db := gormDB(ctx, r.db)
	var items []models.EnhancerRun
	if err := db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("started_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list enhancer runs: %w", err)
	}
	return items, nil
}

func (r *enhancerRepository) HasRunningRun(ctx context.Context, projectID uuid.UUID, staleAfter time.Duration) (bool, error) {
	db := gormDB(ctx, r.db)
	cutoff := time.Now().Add(-staleAfter)

	// Краш-восстановление: прогоны 'running' старше cutoff гасим в failed,
	// иначе упавший процесс навсегда блокировал бы запуск новых прогонов
	// (урок stuck-индексации: статус без recovery — вечный лок).
	if err := db.WithContext(ctx).
		Model(&models.EnhancerRun{}).
		Where("project_id = ? AND status = ? AND started_at < ?", projectID, models.EnhancerRunStatusRunning, cutoff).
		Updates(map[string]any{
			"status":      models.EnhancerRunStatusFailed,
			"error":       "run timed out (stale running state recovered)",
			"finished_at": time.Now(),
		}).Error; err != nil {
		return false, fmt.Errorf("failed to recover stale enhancer runs: %w", err)
	}

	var count int64
	if err := db.WithContext(ctx).
		Model(&models.EnhancerRun{}).
		Where("project_id = ? AND status = ?", projectID, models.EnhancerRunStatusRunning).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to count running enhancer runs: %w", err)
	}
	return count > 0, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Предложения.
// ─────────────────────────────────────────────────────────────────────────────

func (r *enhancerRepository) CreateChange(ctx context.Context, change *models.EnhancerChange) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(change).Error; err != nil {
		return fmt.Errorf("failed to create enhancer change: %w", err)
	}
	return nil
}

func (r *enhancerRepository) CountChangesByRunID(ctx context.Context, runID uuid.UUID) (int64, error) {
	db := gormDB(ctx, r.db)
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.EnhancerChange{}).
		Where("run_id = ?", runID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count enhancer changes: %w", err)
	}
	return count, nil
}

func (r *enhancerRepository) ListChangesByRunID(ctx context.Context, runID uuid.UUID) ([]models.EnhancerChange, error) {
	db := gormDB(ctx, r.db)
	var items []models.EnhancerChange
	if err := db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list enhancer changes: %w", err)
	}
	return items, nil
}
