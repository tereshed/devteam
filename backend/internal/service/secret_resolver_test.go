package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Sprint 16.C-2 — DatabaseSecretResolver no-DB guards.
//
// Покрывают ветки, которые НЕ доходят до query — самые регрессионно-опасные:
// если кто-то заменит ErrSecretNotFound на «»+nil, MCP-сервер получит пустой
// токен и упадёт «401 Unauthorized» с непонятной диагностикой.
//
// БД-зависимый happy-path (resolver реально достаёт ключ из user_llm_credentials)
// покрыт интеграционно — модели используют postgres-specific типы (uuid/bytea/
// timestamp with time zone), которые in-memory SQLite не понимает; нужен
// настоящий Postgres под build tag integration.

func TestDatabaseSecretResolver_NilDepsErrors(t *testing.T) {
	r := &DatabaseSecretResolver{} // db и encryptor не выставлены
	_, err := r.Resolve(context.Background(), &models.Project{}, "anthropic")
	if err == nil {
		t.Fatalf("expected error when deps are nil")
	}
}

func TestDatabaseSecretResolver_NilProjectErrors(t *testing.T) {
	db := openInMemoryDB(t)
	r := NewDatabaseSecretResolver(db, NoopEncryptor{})
	_, err := r.Resolve(context.Background(), nil, "anthropic")
	if err == nil {
		t.Fatalf("expected error when project is nil")
	}
}

func TestDatabaseSecretResolver_EmptyNameErrors(t *testing.T) {
	db := openInMemoryDB(t)
	r := NewDatabaseSecretResolver(db, NoopEncryptor{})
	_, err := r.Resolve(context.Background(), &models.Project{UserID: uuid.New()}, "")
	if err == nil {
		t.Fatalf("expected error for empty name")
	}
}

func TestDatabaseSecretResolver_UnknownProviderReturnsNotFound(t *testing.T) {
	// "github_pat" — НЕ валидный UserLLMProvider, поэтому Resolve не идёт в БД
	// (см. ветку IsValidUserLLMProvider в secret_resolver.go) и fall-through
	// в ErrSecretNotFound. Если кто-то заменит sentinel на «»+nil — тест упадёт.
	db := openInMemoryDB(t)
	r := NewDatabaseSecretResolver(db, NoopEncryptor{})
	p := &models.Project{UserID: uuid.New()}
	_, err := r.Resolve(context.Background(), p, "github_pat")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

// Секрет из «Переменных проекта» (project_secrets) резолвится по key_name.
func TestDatabaseSecretResolver_ResolvesProjectSecret(t *testing.T) {
	db := openInMemoryDB(t)
	enc := makeAESEncryptor(t)
	r := NewDatabaseSecretResolver(db, enc)

	id := uuid.New()
	proj := &models.Project{ID: uuid.New()}
	blob, err := enc.Encrypt([]byte("tok-abc"), []byte(id.String()))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	ps := models.ProjectSecret{ID: id, ProjectID: proj.ID, KeyName: "PAI_TOKEN", EncryptedValue: blob}
	if err := db.Create(&ps).Error; err != nil {
		t.Fatalf("create secret: %v", err)
	}

	got, err := r.Resolve(context.Background(), proj, "PAI_TOKEN")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "tok-abc" {
		t.Fatalf("expected tok-abc, got %q", got)
	}
}

func openInMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	// in-memory SQLite — лёгкий заместитель *gorm.DB для тех веток, где Resolve
	// не делает реальных запросов (например, неизвестный provider). Для
	// happy-path с user_llm_credentials используется интеграционный тест с
	// настоящим Postgres (см. doc package level).
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// project_secrets резолвится вторым шагом — таблица должна существовать,
	// чтобы «секрет не найден» давал ErrSecretNotFound, а не ошибку «no such table».
	// AutoMigrate не годится (Postgres-дефолты модели ломают sqlite DDL) — создаём руками.
	if err := db.Exec(`CREATE TABLE project_secrets (
		id text PRIMARY KEY,
		project_id text,
		key_name text,
		encrypted_value blob,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create project_secrets: %v", err)
	}
	return db
}
