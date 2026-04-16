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

var (
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskConcurrentUpdate = errors.New("task was modified concurrently, please retry")
	ErrAgentNotFound        = errors.New("agent not found")
)

const (
	taskListDefaultLimit = 50
	taskListMaxLimit     = 200
)

var allowedTaskOrderColumns = map[string]bool{
	"created_at": true,
	"updated_at": true,
	"title":      true,
	"status":     true,
	"priority":   true,
	"started_at": true,
}

func sanitizeTaskOrder(orderBy, orderDir string) string {
	if !allowedTaskOrderColumns[orderBy] {
		orderBy = "created_at"
	}
	dir := "DESC"
	if strings.ToUpper(orderDir) == "ASC" {
		dir = "ASC"
	}
	return orderBy + " " + dir
}

func normalizeTaskListLimit(limit int) int {
	if limit <= 0 {
		return taskListDefaultLimit
	}
	if limit > taskListMaxLimit {
		return taskListMaxLimit
	}
	return limit
}

// TaskFilter фильтры и пагинация для списка задач
type TaskFilter struct {
	ProjectID       *uuid.UUID
	Status          *models.TaskStatus
	Statuses        []models.TaskStatus
	Priority        *models.TaskPriority
	AssignedAgentID *uuid.UUID
	CreatedByType   *models.CreatedByType
	CreatedByID     *uuid.UUID
	ParentTaskID    *uuid.UUID
	RootOnly        bool
	BranchName      *string
	Search          *string
	UpdatedAtBefore *time.Time
	Limit           int
	Offset          int
	OrderBy         string
	OrderDir        string
}

// TaskRepository CRUD по задачам (tasks)
type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	List(ctx context.Context, filter TaskFilter) ([]models.Task, int64, error)
	// Update атомарно сохраняет задачу при совпадении status и updated_at с момента чтения (optimistic lock).
	Update(ctx context.Context, task *models.Task, expectedStatus models.TaskStatus, expectedUpdatedAt time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error)
	ListByParentID(ctx context.Context, parentTaskID uuid.UUID) ([]models.Task, error)
}

// taskRepository все публичные методы начинают с db := gormDB(ctx, r.db), чтобы запросы
// выполнялись в транзакции из ctx (TransactionManager.WithTransaction), а не только на r.db.
type taskRepository struct {
	db *gorm.DB
}

// NewTaskRepository создаёт репозиторий задач
func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{db: db}
}

func mapTaskFKViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	cn := pgErr.ConstraintName
	switch {
	case strings.Contains(cn, "project_id"):
		return ErrProjectNotFound
	case strings.Contains(cn, "assigned_agent_id"):
		return ErrAgentNotFound
	case strings.Contains(cn, "parent_task_id"):
		return ErrTaskNotFound
	default:
		return err
	}
}

func (r *taskRepository) applyFilters(db *gorm.DB, filter TaskFilter) *gorm.DB {
	if filter.ProjectID != nil {
		db = db.Where("project_id = ?", *filter.ProjectID)
	}
	if filter.Status != nil {
		db = db.Where("status = ?", *filter.Status)
	}
	if len(filter.Statuses) > 0 {
		db = db.Where("status IN ?", filter.Statuses)
	}
	if filter.Priority != nil {
		db = db.Where("priority = ?", *filter.Priority)
	}
	if filter.AssignedAgentID != nil {
		db = db.Where("assigned_agent_id = ?", *filter.AssignedAgentID)
	}
	if filter.CreatedByType != nil && filter.CreatedByID != nil {
		db = db.Where("created_by_type = ? AND created_by_id = ?", *filter.CreatedByType, *filter.CreatedByID)
	}
	if filter.ParentTaskID != nil {
		db = db.Where("parent_task_id = ?", *filter.ParentTaskID)
	}
	if filter.RootOnly {
		db = db.Where("parent_task_id IS NULL")
	}
	if filter.BranchName != nil {
		db = db.Where("branch_name = ?", *filter.BranchName)
	}
	if filter.Search != nil && *filter.Search != "" {
		escaped := escapeILIKEWildcards(*filter.Search)
		pattern := "%" + escaped + "%"
		db = db.Where("(title ILIKE ? ESCAPE '\\' OR description ILIKE ? ESCAPE '\\')", pattern, pattern)
	}
	if filter.UpdatedAtBefore != nil {
		db = db.Where("updated_at < ?", *filter.UpdatedAtBefore)
	}
	return db
}

func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(task).Error; err != nil {
		if mapped := mapTaskFKViolation(err); mapped != err {
			return mapped
		}
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

func (r *taskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	db := gormDB(ctx, r.db)
	var task models.Task
	err := db.WithContext(ctx).
		Preload("AssignedAgent").
		Preload("SubTasks", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("created_at ASC")
		}).
		Preload("ParentTask").
		Where("id = ?", id).
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

func (r *taskRepository) List(ctx context.Context, filter TaskFilter) ([]models.Task, int64, error) {
	db := gormDB(ctx, r.db)
	base := db.WithContext(ctx).Model(&models.Task{})
	base = r.applyFilters(base, filter)

	var count int64
	if err := base.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}
	if count == 0 {
		return []models.Task{}, 0, nil
	}

	var tasks []models.Task
	q := db.WithContext(ctx).Model(&models.Task{})
	q = r.applyFilters(q, filter)
	order := sanitizeTaskOrder(filter.OrderBy, filter.OrderDir)
	limit := normalizeTaskListLimit(filter.Limit)
	if err := q.Preload("AssignedAgent").Order(order).Limit(limit).Offset(filter.Offset).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list tasks: %w", err)
	}
	return tasks, count, nil
}

// Update сохраняет поля задачи только если в БД всё ещё expectedStatus и expectedUpdatedAt (state machine + гонки).
func (r *taskRepository) Update(ctx context.Context, task *models.Task, expectedStatus models.TaskStatus, expectedUpdatedAt time.Time) error {
	db := gormDB(ctx, r.db)
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"parent_task_id":    task.ParentTaskID,
		"title":             task.Title,
		"description":       task.Description,
		"status":            task.Status,
		"priority":          task.Priority,
		"assigned_agent_id": task.AssignedAgentID,
		"created_by_type":   task.CreatedByType,
		"created_by_id":     task.CreatedByID,
		"context":           task.Context,
		"result":            task.Result,
		"artifacts":         task.Artifacts,
		"branch_name":       task.BranchName,
		"error_message":     task.ErrorMessage,
		"started_at":        task.StartedAt,
		"completed_at":      task.CompletedAt,
		"updated_at":        now,
	}
	result := db.WithContext(ctx).Model(&models.Task{}).
		Where("id = ? AND status = ? AND updated_at = ?", task.ID, expectedStatus, expectedUpdatedAt).
		Updates(updates)
	if result.Error != nil {
		if mapped := mapTaskFKViolation(result.Error); mapped != result.Error {
			return mapped
		}
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTaskConcurrentUpdate
	}
	task.UpdatedAt = now
	return nil
}

func (r *taskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Delete(&models.Task{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}

func (r *taskRepository) CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error) {
	db := gormDB(ctx, r.db)
	var count int64
	if err := db.WithContext(ctx).Model(&models.Task{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count tasks by project: %w", err)
	}
	return count, nil
}

func (r *taskRepository) ListByParentID(ctx context.Context, parentTaskID uuid.UUID) ([]models.Task, error) {
	db := gormDB(ctx, r.db)
	var tasks []models.Task
	if err := db.WithContext(ctx).
		Where("parent_task_id = ?", parentTaskID).
		Order("created_at ASC").
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to list subtasks: %w", err)
	}
	return tasks, nil
}
