package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// task_lock.go — Sprint 17 / Orchestration v2 — сериализация Orchestrator.Step per-task.
//
// YugabyteDB не поддерживает Postgres advisory locks с надёжными гарантиями в распределённой
// среде, поэтому используем row-level lock через `SELECT ... FOR UPDATE NOWAIT` на tasks.id.
// Это нативный для Yugabyte примитив.
//
// Семантика:
//   - Если поток успешно взял lock — он эксклюзивно владеет задачей до конца транзакции.
//   - Если другой воркер уже держит row-lock на этой строке — NOWAIT мгновенно
//     возвращает ошибку lock_not_available; мы трактуем её как "не моя смена,
//     выходим", задача отдана другому Step'у. ErrTaskLockBusy — sentinel для caller'а.
//   - Если задача не существует — gorm.ErrRecordNotFound, обёрнутый в ErrTaskNotFoundForLock.
//
// Использование (внутри сервис-транзакции):
//
//	err := db.Transaction(func(tx *gorm.DB) error {
//	    if err := TryLockTaskForStep(ctx, tx, taskID); err != nil {
//	        if errors.Is(err, ErrTaskLockBusy) { return nil } // другой воркер
//	        return err
//	    }
//	    // ... router.Decide / enqueue jobs ...
//	    return nil
//	})

// ErrTaskLockBusy — задача занята другим Orchestrator.Step. Это НЕ ошибка
// в смысле сбоя; воркер должен молча выйти, событие в очереди останется
// для следующей попытки.
var ErrTaskLockBusy = errors.New("task is locked by another worker")

// ErrTaskNotFoundForLock — задача с заданным id не существует.
var ErrTaskNotFoundForLock = errors.New("task not found for lock")

// pgErrCodeLockNotAvailable — SQLSTATE 55P03, возвращается при NOWAIT когда строка
// уже занята.
const pgErrCodeLockNotAvailable = "55P03"

// TryLockTaskForStep — атомарно берёт эксклюзивный lock на строку tasks.id внутри
// текущей транзакции через SELECT FOR UPDATE NOWAIT.
//
// ВАЖНО: вызывается ВНУТРИ открытой транзакции (tx, не *gorm.DB.WithContext напрямую),
// чтобы lock жил до COMMIT/ROLLBACK. Lock автоматически снимается с завершением tx.
//
// Возвращает:
//   - nil — lock взят, безопасно продолжать Step.
//   - ErrTaskLockBusy — другой воркер уже работает с задачей; молча выйти.
//   - ErrTaskNotFoundForLock — записи нет (например, задачу удалили между enqueue и pick).
//   - другая ошибка — сбой БД, repackage и пробрасываем.
func TryLockTaskForStep(ctx context.Context, tx *gorm.DB, taskID uuid.UUID) error {
	// SELECT ... FOR UPDATE NOWAIT — стандартный SQL, поддерживается Yugabyte.
	// Возвращаемое значение нам не важно — нам нужен сам lock.
	var dummy struct {
		ID uuid.UUID
	}
	err := tx.WithContext(ctx).
		Raw(`SELECT id FROM tasks WHERE id = ? FOR UPDATE NOWAIT`, taskID).
		Scan(&dummy).Error

	if err == nil {
		// Scan не вернёт gorm.ErrRecordNotFound на пустом результате; проверяем сами.
		if dummy.ID == uuid.Nil {
			return ErrTaskNotFoundForLock
		}
		return nil
	}

	// Распакуем pgconn.PgError, чтобы поймать SQLSTATE 55P03.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeLockNotAvailable {
		return ErrTaskLockBusy
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTaskNotFoundForLock
	}
	return fmt.Errorf("failed to acquire task lock for %s: %w", taskID, err)
}
