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

// ErrTaskEventNotFound — sentinel.
var ErrTaskEventNotFound = errors.New("task event not found")

// ErrNoTaskEventAvailable — нет доступного события для забора. Это НЕ ошибка —
// нормальный возврат у воркера, который должен заснуть до следующего polling-цикла
// или Redis-NOTIFY.
var ErrNoTaskEventAvailable = errors.New("no task event available")

// TaskEventRepository — durable очередь поверх таблицы task_events.
//
// Yugabyte НЕ поддерживает LISTEN/NOTIFY — wakeup через Redis (см. redis_notifier.go).
// Забор работы — SELECT ... FOR UPDATE SKIP LOCKED (Yugabyte 2.18+).
type TaskEventRepository interface {
	// Enqueue — кладёт событие в очередь.
	Enqueue(ctx context.Context, event *models.TaskEvent) error

	// ClaimNext атомарно забирает ОДНО ближайшее по scheduled_at непрожатое событие
	// нужного kind, у которого ещё есть попытки (attempts < max_attempts).
	// Помечает locked_by = workerID, locked_at = now() и возвращает запись.
	//
	// Возвращает ErrNoTaskEventAvailable если нет работы (воркер должен заснуть).
	// НЕ ошибка sql.ErrNoRows: caller отличает "нет работы" от "сбой БД".
	ClaimNext(ctx context.Context, kind models.TaskEventKind, workerID string) (*models.TaskEvent, error)

	// Complete — успешное завершение: completed_at = now(), снимаем lock.
	Complete(ctx context.Context, id int64) error

	// Fail — увеличивает attempts, ставит last_error, снимает lock, переносит scheduled_at.
	// Если attempts достиг max_attempts — событие "умирает" (idx_task_events_pollable
	// его уже не вернёт, так как partial index имеет WHERE attempts < max_attempts).
	Fail(ctx context.Context, id int64, errMsg string, retryAfter time.Duration) error

	// ReleaseStuckLocks — освобождает locks старше cutoff (воркер упал, не успел снять).
	// Возвращает количество освобождённых событий.
	ReleaseStuckLocks(ctx context.Context, cutoff time.Time) (int64, error)

	// ListPendingByTaskID — события задачи, не завершённые и не мёртвые.
	// Используется Router'у для построения in-flight списка.
	ListPendingByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskEvent, error)
}

type taskEventRepository struct {
	db *gorm.DB
}

// NewTaskEventRepository — конструктор.
func NewTaskEventRepository(db *gorm.DB) TaskEventRepository {
	return &taskEventRepository{db: db}
}

func (r *taskEventRepository) Enqueue(ctx context.Context, event *models.TaskEvent) error {
	if !event.Kind.IsValid() {
		return fmt.Errorf("invalid task event kind: %q", event.Kind)
	}
	if event.MaxAttempts <= 0 {
		event.MaxAttempts = 3
	}
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("failed to enqueue task event: %w", err)
	}
	return nil
}

// ClaimNext использует raw SQL для FOR UPDATE SKIP LOCKED — это критично для семантики
// очереди и не нативно выражается через GORM-API без падения в Find+Update гонки.
//
// План запроса:
//  1. Подзапрос: ближайший по scheduled_at "живой" event подходящего kind.
//     FOR UPDATE SKIP LOCKED — если кто-то другой уже взял эту строку, мы её пропускаем
//     и берём следующую (Yugabyte 2.18+).
//  2. UPDATE locked_by/locked_at в этой же транзакции.
//  3. RETURNING полную строку.
func (r *taskEventRepository) ClaimNext(ctx context.Context, kind models.TaskEventKind, workerID string) (*models.TaskEvent, error) {
	if workerID == "" {
		return nil, fmt.Errorf("workerID is required for claim (lock ownership)")
	}
	const query = `
WITH next_event AS (
    SELECT id
      FROM task_events
     WHERE kind = ?
       AND locked_by IS NULL
       AND completed_at IS NULL
       AND attempts < max_attempts
       AND scheduled_at <= NOW()
     ORDER BY scheduled_at ASC
     FOR UPDATE SKIP LOCKED
     LIMIT 1
)
UPDATE task_events SET locked_by = ?, locked_at = NOW()
 WHERE id = (SELECT id FROM next_event)
 RETURNING *
`
	var event models.TaskEvent
	err := r.db.WithContext(ctx).
		Raw(query, string(kind), workerID).
		Scan(&event).Error
	if err != nil {
		return nil, fmt.Errorf("failed to claim next task event: %w", err)
	}
	// Scan не вернёт ErrRecordNotFound при пустом результате RETURNING — проверяем сами.
	if event.ID == 0 {
		return nil, ErrNoTaskEventAvailable
	}
	return &event, nil
}

func (r *taskEventRepository) Complete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).
		Model(&models.TaskEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"completed_at": time.Now(),
			"locked_by":    nil,
			"locked_at":    nil,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to complete task event %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTaskEventNotFound
	}
	return nil
}

func (r *taskEventRepository) Fail(ctx context.Context, id int64, errMsg string, retryAfter time.Duration) error {
	scheduledAt := time.Now().Add(retryAfter)
	result := r.db.WithContext(ctx).Exec(
		`UPDATE task_events
		    SET attempts     = attempts + 1,
		        last_error   = ?,
		        scheduled_at = ?,
		        locked_by    = NULL,
		        locked_at    = NULL
		  WHERE id = ?`,
		errMsg, scheduledAt, id,
	)
	if result.Error != nil {
		return fmt.Errorf("failed to mark task event %d as failed: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTaskEventNotFound
	}
	return nil
}

func (r *taskEventRepository) ReleaseStuckLocks(ctx context.Context, cutoff time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Exec(
		`UPDATE task_events
		    SET locked_by = NULL, locked_at = NULL
		  WHERE locked_by IS NOT NULL
		    AND locked_at < ?`,
		cutoff,
	)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to release stuck task event locks: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *taskEventRepository) ListPendingByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TaskEvent, error) {
	var events []models.TaskEvent
	err := r.db.WithContext(ctx).
		Where("task_id = ? AND completed_at IS NULL AND attempts < max_attempts", taskID).
		Order("scheduled_at ASC").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list pending task events for task %s: %w", taskID, err)
	}
	return events, nil
}
