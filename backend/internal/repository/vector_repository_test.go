package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/vectordb"
)

// ========================================
// NewVectorRepository Tests
// ========================================

func TestNewVectorRepository(t *testing.T) {
	// Создаём клиент (без реального подключения)
	cfg := &vectordb.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := vectordb.NewClient(cfg)
	require.NoError(t, err)

	repo := NewVectorRepository(client)

	assert.NotNil(t, repo)
}

// ========================================
// Interface Implementation Tests
// ========================================

func TestVectorRepository_ImplementsInterface(t *testing.T) {
	cfg := &vectordb.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := vectordb.NewClient(cfg)
	require.NoError(t, err)

	repo := NewVectorRepository(client)

	// Проверяем что реализует интерфейс VectorRepository
	var _ VectorRepository = repo
}

// ========================================
// Error Variable Tests
// ========================================

func TestErrVectorDocumentNotFound(t *testing.T) {
	assert.NotNil(t, ErrVectorDocumentNotFound)
	assert.Equal(t, "vector document not found", ErrVectorDocumentNotFound.Error())
}

// ========================================
// SearchParams Default Tests
// ========================================

func TestVectorRepository_Search_DefaultParams(t *testing.T) {
	cfg := &vectordb.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := vectordb.NewClient(cfg)
	require.NoError(t, err)

	repo := NewVectorRepository(client)

	// Без реального Weaviate этот тест проверяет только что метод не паникует
	ctx := context.Background()

	// Тест с nil params - должен использовать дефолты
	// Примечание: это вызовет ошибку т.к. нет реального Weaviate
	_, err = repo.Search(ctx, nil)

	// Ожидаем ошибку подключения, но не панику
	assert.Error(t, err)
}

// ========================================
// VectorDocument Model Tests
// ========================================

func TestVectorDocument_ForRepository(t *testing.T) {
	doc := models.NewVectorDocument(
		"content-123",
		"Test content for repository",
		models.ContentType("article"),
	).
		WithCategory("tech").
		WithTags("go", "testing")

	doc.SetMetadata("author", "John")

	// Проверяем что документ готов для репозитория
	assert.Equal(t, "content-123", doc.ContentID)
	assert.Equal(t, "Test content for repository", doc.Content)
	assert.True(t, doc.ContentType.IsValid())
	assert.Equal(t, "tech", doc.Category)
	assert.Len(t, doc.Tags, 2)

	author, exists := doc.GetMetadata("author")
	assert.True(t, exists)
	assert.Equal(t, "John", author)
}

// ========================================
// Integration Test Notes
// ========================================
// Для полных интеграционных тестов VectorRepository требуется:
// 1. Запущенный Weaviate сервер
// 2. Тег //go:build integration
// 3. Отдельный файл vector_repository_integration_test.go
//
// Пример запуска:
//   make test-integration
//   или
//   go test -tags=integration ./internal/repository/...

