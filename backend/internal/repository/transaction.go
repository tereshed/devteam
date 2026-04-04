package repository

import (
	"context"

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

func (m *gormTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(WithGormTx(ctx, tx))
	})
}
