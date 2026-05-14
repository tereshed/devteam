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
	return db
}
