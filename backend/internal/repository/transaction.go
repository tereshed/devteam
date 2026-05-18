package repository

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type gormTxCtxKey struct{}

// WithGormTx кладёт в ctx сессию GORM (активную транзакцию). Репозитории используют её внутри TransactionManager.WithTransaction.
func WithGormTx(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, gormTxCtxKey{}, db)
}

// GormTxFromContext возвращает транзакционную сессию из ctx (удобно для unit-тестов).
func GormTxFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(gormTxCtxKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return nil, false
	}
	return tx, true
}

func gormDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := GormTxFromContext(ctx); ok {
		return tx
	}
	return defaultDB
}

// TransactionManager объединяет несколько вызовов репозиториев в одну SQL-транзакцию без протекания *gorm.DB в слой service.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type gormTransactionManager struct {
	db *gorm.DB
}

// NewTransactionManager создаёт менеджер транзакций на базе GORM.
func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &gormTransactionManager{db: db}
}

// txRetryMaxAttempts — верхняя граница количества попыток одной транзакции.
// YugabyteDB по умолчанию использует Snapshot Isolation; при конкурентной
// модификации одной строки несколько TX-фаз могут вернуть SQLSTATE 40001
// (serialization_failure) или 40P01 (deadlock_detected). Это recoverable-
// ошибки: правильное поведение клиента — повторить транзакцию, см.
// https://www.yugabyte.com/blog/transactional-databases-stale-data/.
//
// 10 попыток с jitter покрывают тяжёлый contention'ный паттерн (parallel-
// orchestrator workers + smoke-тесты) — при дефолтном poll-interval'е
// step/agent workers'ов 500ms и 5 одновременных тестах гонка случается часто.
const txRetryMaxAttempts = 10

// txRetryBaseBackoff — стартовая задержка между попытками; растёт экспоненциально
// (cap 1s) с jitter ±25%.
const txRetryBaseBackoff = 10 * time.Millisecond
const txRetryMaxBackoff = 1 * time.Second

func (m *gormTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	var lastErr error
	backoff := txRetryBaseBackoff
	for attempt := 0; attempt < txRetryMaxAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return fn(WithGormTx(ctx, tx))
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryablePgError(err) {
			return err
		}
		// jitter ±25% — разводим параллельные ретраи во избежание thundering herd.
		jitterRange := backoff / 2
		jitter := time.Duration(rand.Int63n(int64(jitterRange) + 1))
		sleep := backoff - jitterRange/2 + jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
		backoff *= 2
		if backoff > txRetryMaxBackoff {
			backoff = txRetryMaxBackoff
		}
	}
	return lastErr
}

// isRetryablePgError возвращает true для recoverable SQLSTATE-кодов:
//   - 40001 serialization_failure — конкурентная модификация снэпшота;
//   - 40P01 deadlock_detected     — две транзакции взаимно заблокированы.
//
// Только эти два кода — НЕ универсальный «retry на любую DB-ошибку». Расширять
// список без необходимости опасно: бизнес-ошибки (FK violation, unique etc.)
// retry'ить нельзя — они детерминированно повторятся.
func isRetryablePgError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01":
		return true
	}
	return false
}
