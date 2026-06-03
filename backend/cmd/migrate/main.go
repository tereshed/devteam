// cmd/migrate — одноразовый накат goose-миграций для multi-instance деплоя.
//
// Запускается отдельным one-shot job (см. сервис `migrate` в docker-compose), ДО старта
// реплик API/worker. Это убирает гонку, когда goose.Up бежал бы на каждой реплике
// одновременно (см. AUTO_MIGRATE и internal/database.RunMigrations).
package main

import (
	"log"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("migrate: failed to load config: %v", err)
	}

	// Минимальное подключение: одноразовому job тюнинг пула/логгера не нужен.
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("migrate: failed to connect to database: %v", err)
	}

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("migrate: failed to apply migrations: %v", err)
	}

	log.Println("migrate: migrations applied successfully")
}
