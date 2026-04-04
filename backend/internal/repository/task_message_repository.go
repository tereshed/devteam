package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrTaskMessageNotFound = errors.New("task message not found")
	// ErrTaskNotFound — несуществующий task_id при создании сообщения (FK).
	// При добавлении TaskRepository перенести в task_repository.go при дублировании.
	ErrTaskNotFound = errors.New("task not found")
)

// TaskMessageFilter фильтры и пагинация для списков сообщений задачи.
// Лимит и смещение задаёт сервис/хендлер.
type TaskMessageFilter struct {
	MessageType *models.MessageType
	SenderType  *models.SenderType
	Limit       int
	Offset      int
}

// TaskMessageRepository append-only доступ к task_messages.
type TaskMessageRepository interface {
	Create(ctx context.Context, msg *models.TaskMessage) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.TaskMessage, error)
	ListByTaskID(ctx context.Context, taskID uuid.UUID, filter TaskMessageFilter) ([]models.TaskMessage, int64, error)
	ListBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter TaskMessageFilter) ([]models.TaskMessage, int64, error)
	CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error)
}

type taskMessageRepository struct {
	db *gorm.DB
}

// NewTaskMessageRepository создаёт репозиторий сообщений задач.
func NewTaskMessageRepository(db *gorm.DB) TaskMessageRepository {
	return &taskMessageRepository{db: db}
}

func (r *taskMessageRepository) Create(ctx context.Context, msg *models.TaskMessage) error {
	if err := r.db.WithContext(ctx).Create(msg).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrTaskNotFound
		}
		return fmt.Errorf("failed to create task message: %w", err)
	}
	return nil
}

func (r *taskMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskMessage, error) {
	var m models.TaskMessage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskMessageNotFound
		}
		return nil, fmt.Errorf("failed to get task message: %w", err)
	}
	return &m, nil
}

func (r *taskMessageRepository) queryByTaskID(ctx context.Context, taskID uuid.UUID, filter TaskMessageFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.TaskMessage{}).Where("task_id = ?", taskID)
	if filter.MessageType != nil {
		q = q.Where("message_type = ?", *filter.MessageType)
	}
	if filter.SenderType != nil {
		q = q.Where("sender_type = ?", *filter.SenderType)
	}
	return q
}

func (r *taskMessageRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, filter TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	base := r.queryByTaskID(ctx, taskID, filter)
	var count int64
	if err := base.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count task messages: %w", err)
	}

	var list []models.TaskMessage
	q := r.queryByTaskID(ctx, taskID, filter).Order("created_at ASC")
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list task messages: %w", err)
	}
	return list, count, nil
}

func (r *taskMessageRepository) queryBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter TaskMessageFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.TaskMessage{}).
		Where("sender_type = ? AND sender_id = ?", senderType, senderID)
	if filter.MessageType != nil {
		q = q.Where("message_type = ?", *filter.MessageType)
	}
	return q
}

func (r *taskMessageRepository) ListBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	base := r.queryBySender(ctx, senderType, senderID, filter)
	var count int64
	if err := base.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count task messages by sender: %w", err)
	}

	var list []models.TaskMessage
	q := r.queryBySender(ctx, senderType, senderID, filter).Order("created_at DESC")
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list task messages by sender: %w", err)
	}
	return list, count, nil
}

func (r *taskMessageRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.TaskMessage{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count task messages by task id: %w", err)
	}
	return count, nil
}
