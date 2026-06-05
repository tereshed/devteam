package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrScheduledTaskNotFound — расписание не найдено.
var ErrScheduledTaskNotFound = errors.New("scheduled task not found")

// ScheduledTaskRepository — CRUD по регулярным задачам (scheduled_tasks).
type ScheduledTaskRepository interface {
	Create(ctx context.Context, st *models.ScheduledTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledTask, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.ScheduledTask, error)
	Update(ctx context.Context, st *models.ScheduledTask) error
	Delete(ctx context.Context, id uuid.UUID) error
	// ListDue возвращает активные расписания, у которых next_run_at <= now,
	// отсортированные по next_run_at. limit==0 → дефолт 100.
	ListDue(ctx context.Context, now time.Time, limit int) ([]models.ScheduledTask, error)
}

type scheduledTaskRepository struct {
	db *gorm.DB
}

// NewScheduledTaskRepository создаёт репозиторий регулярных задач.
func NewScheduledTaskRepository(db *gorm.DB) ScheduledTaskRepository {
	return &scheduledTaskRepository{db: db}
}

func mapScheduledTaskFKViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	cn := pgErr.ConstraintName
	switch {
	case strings.Contains(cn, "project_id"):
		return ErrProjectNotFound
	case strings.Contains(cn, "team_id"):
		return ErrTeamNotFound
	default:
		return err
	}
}

func (r *scheduledTaskRepository) Create(ctx context.Context, st *models.ScheduledTask) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(st).Error; err != nil {
		if mapped := mapScheduledTaskFKViolation(err); mapped != err {
			return mapped
		}
		return fmt.Errorf("failed to create scheduled task: %w", err)
	}
	return nil
}

func (r *scheduledTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledTask, error) {
	db := gormDB(ctx, r.db)
	var st models.ScheduledTask
	if err := db.WithContext(ctx).Where("id = ?", id).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScheduledTaskNotFound
		}
		return nil, fmt.Errorf("failed to get scheduled task: %w", err)
	}
	return &st, nil
}

func (r *scheduledTaskRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.ScheduledTask, error) {
	db := gormDB(ctx, r.db)
	var items []models.ScheduledTask
	if err := db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list scheduled tasks: %w", err)
	}
	return items, nil
}

func (r *scheduledTaskRepository) Update(ctx context.Context, st *models.ScheduledTask) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).
		Model(&models.ScheduledTask{}).
		Where("id = ?", st.ID).
		Updates(map[string]any{
			"team_id":         st.TeamID,
			"name":            st.Name,
			"description":     st.Description,
			"cron_expression": st.CronExpression,
			"priority":        st.Priority,
			"is_active":       st.IsActive,
			"last_run_at":     st.LastRunAt,
			"next_run_at":     st.NextRunAt,
			"updated_at":      time.Now(),
		})
	if res.Error != nil {
		if mapped := mapScheduledTaskFKViolation(res.Error); mapped != res.Error {
			return mapped
		}
		return fmt.Errorf("failed to update scheduled task: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrScheduledTaskNotFound
	}
	return nil
}

func (r *scheduledTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&models.ScheduledTask{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete scheduled task: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrScheduledTaskNotFound
	}
	return nil
}

func (r *scheduledTaskRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]models.ScheduledTask, error) {
	if limit <= 0 {
		limit = 100
	}
	db := gormDB(ctx, r.db)
	var items []models.ScheduledTask
	if err := db.WithContext(ctx).
		Where("is_active = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list due scheduled tasks: %w", err)
	}
	return items, nil
}
