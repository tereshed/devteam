package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsPostgresUniqueViolation — конфликт UNIQUE (код 23505), в т.ч. через обёртки GORM.
func IsPostgresUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// PostgresErrorFields — код SQLSTATE и имя ограничения для логов (без текста запроса).
func PostgresErrorFields(err error) (code, constraint string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}
