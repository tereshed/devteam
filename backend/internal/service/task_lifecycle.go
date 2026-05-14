package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// task_lifecycle.go — Sprint 17 / Orchestration v2 — пользовательские lifecycle-операции
// над задачами (cancel). Создание задач остаётся в существующем TaskService.
//
// Здесь — только новые операции, специфичные для v2-оркестрации.

// ErrTaskNotCancellable — задача в состоянии, которое нельзя отменить
// (уже done/failed/cancelled/needs_human). Caller возвращает 409 Conflict в HTTP.
var ErrTaskNotCancellable = errors.New("task is not in a cancellable state")

// TaskLifecycleService — операции над жизненным циклом задач v2.
type TaskLifecycleService struct {
	db       *gorm.DB
	notifier *RedisNotifier // опционально — может быть nil
	logger   *slog.Logger
}

// NewTaskLifecycleService — конструктор.
func NewTaskLifecycleService(db *gorm.DB, notifier *RedisNotifier, logger *slog.Logger) *TaskLifecycleService {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &TaskLifecycleService{db: db, notifier: notifier, logger: logger}
}

// RequestCancel — кооперативная отмена активной задачи:
//  1. UPDATE tasks SET cancel_requested=true WHERE id=? AND state='active'.
//  2. NotifyTaskCancel через Redis (best-effort, polling воркеры всё равно увидят).
//
// Орcheстратор и Agent Worker'ы увидят флаг на следующем чек'е и корректно завершат
// работу (см. AgentWorker.handleCancellation, Orchestrator.Step при cancel_requested=true).
//
// Возвращает ErrTaskNotCancellable если задача не в state='active'.
func (s *TaskLifecycleService) RequestCancel(ctx context.Context, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return fmt.Errorf("task lifecycle: taskID is required")
	}

	// Atomic UPDATE с фильтром по state — защищает от гонок (две одновременные
	// отмены, или попытка отменить уже завершённую задачу).
	result := s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ? AND state = ?", taskID, models.TaskStateActive).
		Update("cancel_requested", true)
	if result.Error != nil {
		return fmt.Errorf("task lifecycle: set cancel_requested: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTaskNotCancellable
	}

	// Redis-NOTIFY — best-effort. Если упадёт, in-flight воркеры всё равно проверят
	// cancel_requested через DB-polling (но с большей latency).
	if s.notifier != nil {
		if err := s.notifier.NotifyTaskCancel(ctx, taskID); err != nil {
			s.logger.WarnContext(ctx, "cancel NOTIFY failed (polling fallback active)",
				"task_id", taskID, "error", err.Error())
		}
	}

	s.logger.InfoContext(ctx, "cancel requested",
		"task_id", taskID)
	return nil
}
