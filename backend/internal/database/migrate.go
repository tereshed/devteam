// Package database содержит общие операции с БД, переиспользуемые разными бинарями
// (cmd/api и одноразовый cmd/migrate).
package database

import (
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// migrationsDir — путь к goose-миграциям относительно рабочей директории процесса.
// В Docker WORKDIR=/root, миграции лежат в /root/db/migrations; при локальном запуске
// из backend/ — в db/migrations. Совпадает для cmd/api и cmd/migrate.
const migrationsDir = "db/migrations"

// RunMigrations применяет все ожидающие goose-миграции (диалект postgres / YugabyteDB).
//
// В multi-instance деплое вызывать ОДИН раз через отдельный one-shot job (cmd/migrate),
// а не на старте каждой реплики API — goose.Up на нескольких репликах одновременно
// устраивает гонку на goose_db_version (см. AUTO_MIGRATE и сервис migrate в compose).
func RunMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, migrationsDir)
}
