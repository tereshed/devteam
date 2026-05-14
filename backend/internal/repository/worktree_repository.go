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

// ErrWorktreeNotFound — sentinel.
var ErrWorktreeNotFound = errors.New("worktree not found")

// WorktreeRepository — учёт git worktree'ев для параллельных sandbox-агентов.
//
// БЕЗОПАСНОСТЬ: путь к worktree НЕ хранится в БД — он вычисляется
// в Go-коде через Worktree.ComputePath(worktreesRoot). Repository отвечает только
// за метаданные (state, branch_name, agent_job_id).
type WorktreeRepository interface {
	Create(ctx context.Context, w *models.Worktree) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Worktree, error)

	// ListByTaskID — все worktree'ы задачи (для UI и cancel cleanup).
	ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.Worktree, error)

	// UpdateState — атомарно меняет state + released_at (последний выставляется при state='released').
	// Возвращает ErrWorktreeNotFound если запись не существует.
	UpdateState(ctx context.Context, id uuid.UUID, newState models.WorktreeState) error

	// ListForCleanup — released worktree'ы старше cutoff, готовые к физическому удалению.
	// Используется cron'ом (retention 1 сутки после release).
	ListForCleanup(ctx context.Context, cutoff time.Time) ([]models.Worktree, error)

	// Delete — физическое удаление записи. Должно вызываться ПОСЛЕ os.RemoveAll успешного.
	Delete(ctx context.Context, id uuid.UUID) error
}

type worktreeRepository struct {
	db *gorm.DB
}

// NewWorktreeRepository — конструктор.
func NewWorktreeRepository(db *gorm.DB) WorktreeRepository {
	return &worktreeRepository{db: db}
}

func (r *worktreeRepository) Create(ctx context.Context, w *models.Worktree) error {
	// BranchName форсится в BeforeCreate; здесь валидируем что итог соответствует формату.
	// (Защита от случайной модификации между BeforeCreate и DB-INSERT.)
	if w.BranchName != "" && !models.ValidateWorktreeBranchName(w.BranchName) {
		return fmt.Errorf("invalid worktree branch_name format: %q", w.BranchName)
	}
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	return nil
}

func (r *worktreeRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Worktree, error) {
	var w models.Worktree
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorktreeNotFound
		}
		return nil, fmt.Errorf("failed to get worktree %s: %w", id, err)
	}
	return &w, nil
}

func (r *worktreeRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.Worktree, error) {
	var ws []models.Worktree
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("allocated_at ASC").
		Find(&ws).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees for task %s: %w", taskID, err)
	}
	return ws, nil
}

func (r *worktreeRepository) UpdateState(ctx context.Context, id uuid.UUID, newState models.WorktreeState) error {
	if !newState.IsValid() {
		return fmt.Errorf("invalid worktree state: %q", newState)
	}
	updates := map[string]any{"state": newState}
	if newState == models.WorktreeStateReleased {
		updates["released_at"] = time.Now()
	}
	result := r.db.WithContext(ctx).
		Model(&models.Worktree{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update worktree %s state: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWorktreeNotFound
	}
	return nil
}

func (r *worktreeRepository) ListForCleanup(ctx context.Context, cutoff time.Time) ([]models.Worktree, error) {
	var ws []models.Worktree
	err := r.db.WithContext(ctx).
		Where("state = ? AND released_at IS NOT NULL AND released_at < ?",
			models.WorktreeStateReleased, cutoff).
		Order("released_at ASC").
		Find(&ws).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees for cleanup before %s: %w", cutoff, err)
	}
	return ws, nil
}

func (r *worktreeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Worktree{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete worktree %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWorktreeNotFound
	}
	return nil
}
