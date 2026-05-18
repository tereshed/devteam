//go:build featuresmoke

package featuresmoke

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

// gooseDialectOnce — goose.SetDialect пишет в package-level переменную внутри
// goose, поэтому два параллельных теста (Idempotent + StatusReportsHistory)
// без mutex'а ловят DATA RACE. Один раз на весь тестовый процесс — достаточно.
var (
	gooseDialectOnce sync.Once
	gooseDialectErr  error
)

func setupGooseDialect(t *testing.T) {
	t.Helper()
	gooseDialectOnce.Do(func() {
		gooseDialectErr = goose.SetDialect("postgres")
	})
	if gooseDialectErr != nil {
		t.Fatalf("goose SetDialect: %v", gooseDialectErr)
	}
}

// migrations_smoke_test.go — P3 идемпотентность Goose-миграций.
//
// Гарантирует:
//   - все миграции в backend/db/migrations/ применяются без ошибок;
//   - повторный goose.Up на уже накатаной БД — no-op (RowsAffected=0 на
//     goose_db_version и НЕТ duplicate-create ошибок);
//   - текущая версия goose_db_version >= номера последней миграции на диске.
//
// КРИТИЧНО (зачем): mock featuresmoke использует ОДНУ shared YugabyteDB
// между прогонами. Если бы какая-то миграция была неидемпотентной (например,
// CREATE TABLE без IF NOT EXISTS внутри `+goose Up`), второй прогон
// `make test-features` уже бы валил backend на старте. Этот тест ловит
// регрессию ДО того, как разработчик уйдёт читать логи backend'а.

func TestMigrations_Idempotent(t *testing.T) {
	t.Parallel()
	// Подразумевает, что StartServer уже однократно запустил goose.Up
	// (через cmd/api). Здесь повторяем Up вторым клиентом — он должен
	// вернуться чисто и БЕЗ изменений.
	h := StartServer(t)
	_ = h // только чтобы достать общий backend и быть уверенным, что миграции уже накатаны

	db := directDB(t)

	// goose-API требует свой стилевой sql.DB. Мы используем тот же sql.DB,
	// что и для прямых SELECT'ов — pgx-stdlib совместим. SetDialect через
	// sync.Once (см. setupGooseDialect): goose хранит dialect в package-level
	// переменной, конкурентные SetDialect → DATA RACE под -race.
	setupGooseDialect(t)

	migrationsDir := migrationsAbsPath(t)
	beforeVersion, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("goose GetDBVersion before: %v", err)
	}
	if beforeVersion <= 0 {
		t.Fatalf("goose: версия %d <= 0 — миграции не накатаны? "+
			"StartServer должен был это сделать.", beforeVersion)
	}

	// Идемпотентный повтор.
	if err := goose.Up(db, migrationsDir); err != nil {
		t.Fatalf("повторный goose.Up должен быть no-op, получили ошибку: %v", err)
	}

	afterVersion, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("goose GetDBVersion after: %v", err)
	}
	if afterVersion != beforeVersion {
		t.Fatalf("goose: повторный Up сдвинул версию (%d → %d), ожидали no-op",
			beforeVersion, afterVersion)
	}

	// Sanity-check: версия в БД должна быть >= номера последней миграции на диске.
	highestOnDisk := highestMigrationNumber(t, migrationsDir)
	if int64(beforeVersion) < int64(highestOnDisk) {
		t.Fatalf("goose: версия в БД %d ниже последней миграции %d на диске",
			beforeVersion, highestOnDisk)
	}
}

// TestMigrations_StatusReportsHistory — sanity: goose.Status проходит без
// ошибок (читает goose_db_version + сверяет с файлами). Этого достаточно,
// чтобы гарантировать, что timestamp'ы и порядок не разошлись (повреждённая
// goose_db_version сломалa бы Status).
func TestMigrations_StatusReportsHistory(t *testing.T) {
	t.Parallel()
	_ = StartServer(t)
	db := directDB(t)

	setupGooseDialect(t)
	migrationsDir := migrationsAbsPath(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Status в goose v3 печатает в stdout; нам важно лишь отсутствие ошибки.
	done := make(chan error, 1)
	go func() {
		done <- goose.Status(db, migrationsDir)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("goose.Status: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("goose.Status: timeout 30s")
	}
}

// migrationsAbsPath возвращает абсолютный путь к backend/db/migrations.
// Идёт от backendRoot (см. harness.go).
func migrationsAbsPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(backendRoot(), "db", "migrations")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("migrationsAbsPath: %s: %v", p, err)
	}
	return p
}

// highestMigrationNumber возвращает максимальный числовой префикс файлов
// в migrationsDir (001_..., 047_..., и т.п.).
func highestMigrationNumber(t *testing.T, dir string) int64 {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var maxN int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}
		// goose требует имена вида <version>_<title>.sql, version — integer.
		var n int64
		for i := 0; i < len(name); i++ {
			c := name[i]
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int64(c-'0')
		}
		if n > maxN {
			maxN = n
		}
	}
	if maxN == 0 {
		t.Fatalf("highestMigrationNumber: ни одного *.sql файла не нашли в %s", dir)
	}
	return maxN
}

