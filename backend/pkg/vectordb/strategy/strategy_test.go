package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
)

// ========================================
// StrategyRegistry Tests
// ========================================

func TestNewStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.strategies)
	assert.Empty(t, registry.strategies)
}

func TestStrategyRegistry_Register(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewGenericStrategy("content", "id")
	contentType := models.ContentType("article")

	registry.Register(contentType, strategy)

	registered, err := registry.Get(contentType)
	require.NoError(t, err)
	assert.Equal(t, strategy, registered)
}

func TestStrategyRegistry_Get_NotFound(t *testing.T) {
	registry := NewStrategyRegistry()
	contentType := models.ContentType("unknown")

	_, err := registry.Get(contentType)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no strategy registered")
}

func TestStrategyRegistry_Register_Override(t *testing.T) {
	registry := NewStrategyRegistry()
	contentType := models.ContentType("article")

	strategy1 := NewGenericStrategy("content1", "id")
	strategy2 := NewGenericStrategy("content2", "id")

	registry.Register(contentType, strategy1)
	registry.Register(contentType, strategy2) // Переопределение

	registered, err := registry.Get(contentType)
	require.NoError(t, err)
	assert.Equal(t, strategy2, registered)
}

// ========================================
// DefaultRegistry Tests
// ========================================

func TestDefaultRegistry(t *testing.T) {
	// Сохраняем состояние
	originalRegistry := DefaultRegistry
	defer func() {
		DefaultRegistry = originalRegistry
	}()

	// Создаём чистый реестр
	DefaultRegistry = NewStrategyRegistry()

	contentType := models.ContentType("test_type")
	strategy := NewGenericStrategy("content", "id")

	RegisterStrategy(contentType, strategy)

	registered, err := GetStrategy(contentType)
	require.NoError(t, err)
	assert.Equal(t, strategy, registered)
}

// ========================================
// GenericStrategy Tests
// ========================================

func TestNewGenericStrategy(t *testing.T) {
	strategy := NewGenericStrategy("body", "uuid")

	assert.Equal(t, "body", strategy.ContentField)
	assert.Equal(t, "uuid", strategy.IDField)
	assert.Contains(t, strategy.RequiredFields, "uuid")
	assert.Contains(t, strategy.RequiredFields, "body")
	assert.Empty(t, strategy.MetadataFields)
}

func TestGenericStrategy_WithRequiredFields(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithRequiredFields("title", "author")

	assert.Contains(t, strategy.RequiredFields, "id")
	assert.Contains(t, strategy.RequiredFields, "body")
	assert.Contains(t, strategy.RequiredFields, "title")
	assert.Contains(t, strategy.RequiredFields, "author")
}

func TestGenericStrategy_WithMetadataFields(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithMetadataFields("category", "tags", "published_at")

	assert.Len(t, strategy.MetadataFields, 3)
	assert.Contains(t, strategy.MetadataFields, "category")
	assert.Contains(t, strategy.MetadataFields, "tags")
	assert.Contains(t, strategy.MetadataFields, "published_at")
}

func TestGenericStrategy_PrepareContent_Success(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	data := map[string]interface{}{
		"id":   "123",
		"body": "  Hello World  ",
	}

	content, err := strategy.PrepareContent(data)

	require.NoError(t, err)
	assert.Equal(t, "Hello World", content) // Trimmed
}

func TestGenericStrategy_PrepareContent_MissingField(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	data := map[string]interface{}{
		"id": "123",
		// body отсутствует
	}

	_, err := strategy.PrepareContent(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "body")
}

func TestGenericStrategy_PrepareContent_WrongType(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	data := "not a map" // Неверный тип данных

	_, err := strategy.PrepareContent(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expects map[string]interface{}")
}

func TestGenericStrategy_PrepareContent_FieldNotString(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	data := map[string]interface{}{
		"id":   "123",
		"body": 12345, // Число вместо строки
	}

	_, err := strategy.PrepareContent(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a string")
}

func TestGenericStrategy_ExtractMetadata_Success(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithMetadataFields("category", "author", "tags")

	data := map[string]interface{}{
		"id":       "123",
		"body":     "content",
		"category": "tech",
		"author":   "John Doe",
		"tags":     []string{"go", "testing"},
		"ignored":  "this field should be ignored",
	}

	metadata, err := strategy.ExtractMetadata(data)

	require.NoError(t, err)
	assert.Equal(t, "tech", metadata["category"])
	assert.Equal(t, "John Doe", metadata["author"])
	assert.Equal(t, []string{"go", "testing"}, metadata["tags"])
	assert.NotContains(t, metadata, "ignored")
	assert.NotContains(t, metadata, "id")
	assert.NotContains(t, metadata, "body")
}

func TestGenericStrategy_ExtractMetadata_MissingFields(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithMetadataFields("category", "author")

	data := map[string]interface{}{
		"id":   "123",
		"body": "content",
		// category и author отсутствуют
	}

	metadata, err := strategy.ExtractMetadata(data)

	require.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestGenericStrategy_ExtractMetadata_WrongType(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	_, err := strategy.ExtractMetadata("not a map")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expects map[string]interface{}")
}

func TestGenericStrategy_Validate_Success(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithRequiredFields("title")

	data := map[string]interface{}{
		"id":    "123",
		"body":  "content",
		"title": "My Article",
	}

	err := strategy.Validate(data)

	assert.NoError(t, err)
}

func TestGenericStrategy_Validate_MissingRequiredField(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithRequiredFields("title")

	data := map[string]interface{}{
		"id":   "123",
		"body": "content",
		// title отсутствует
	}

	err := strategy.Validate(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field 'title' is missing")
}

func TestGenericStrategy_Validate_EmptyStringField(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	data := map[string]interface{}{
		"id":   "123",
		"body": "  ", // Пробелы - считается пустым
	}

	err := strategy.Validate(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field 'body' is empty")
}

func TestGenericStrategy_Validate_WrongType(t *testing.T) {
	strategy := NewGenericStrategy("body", "id")

	err := strategy.Validate([]string{"not", "a", "map"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expects map[string]interface{}")
}

func TestGenericStrategy_Validate_NonStringFieldNotEmpty(t *testing.T) {
	strategy := NewGenericStrategy("body", "id").
		WithRequiredFields("count")

	data := map[string]interface{}{
		"id":    "123",
		"body":  "content",
		"count": 0, // Число 0 - не считается пустым (не строка)
	}

	err := strategy.Validate(data)

	assert.NoError(t, err)
}

// ========================================
// Integration Tests
// ========================================

func TestGenericStrategy_FullWorkflow(t *testing.T) {
	// Создаём стратегию для "статей"
	articleStrategy := NewGenericStrategy("content", "article_id").
		WithRequiredFields("title").
		WithMetadataFields("author", "category", "published_at")

	// Данные статьи
	article := map[string]interface{}{
		"article_id":   "abc-123",
		"title":        "Introduction to Go",
		"content":      "Go is a statically typed, compiled programming language.",
		"author":       "Jane Smith",
		"category":     "programming",
		"published_at": "2024-01-15T10:00:00Z",
	}

	// 1. Валидация
	err := articleStrategy.Validate(article)
	require.NoError(t, err)

	// 2. Подготовка контента
	content, err := articleStrategy.PrepareContent(article)
	require.NoError(t, err)
	assert.Equal(t, "Go is a statically typed, compiled programming language.", content)

	// 3. Извлечение метаданных
	metadata, err := articleStrategy.ExtractMetadata(article)
	require.NoError(t, err)
	assert.Equal(t, "Jane Smith", metadata["author"])
	assert.Equal(t, "programming", metadata["category"])
	assert.Equal(t, "2024-01-15T10:00:00Z", metadata["published_at"])
}

func TestMultipleStrategiesInRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	articleType := models.ContentType("article")
	productType := models.ContentType("product")
	faqType := models.ContentType("faq")

	articleStrategy := NewGenericStrategy("body", "id")
	productStrategy := NewGenericStrategy("description", "sku")
	faqStrategy := NewGenericStrategy("answer", "question_id")

	registry.Register(articleType, articleStrategy)
	registry.Register(productType, productStrategy)
	registry.Register(faqType, faqStrategy)

	// Проверяем что все стратегии зарегистрированы
	s1, err := registry.Get(articleType)
	require.NoError(t, err)
	assert.Equal(t, "body", s1.(*GenericStrategy).ContentField)

	s2, err := registry.Get(productType)
	require.NoError(t, err)
	assert.Equal(t, "description", s2.(*GenericStrategy).ContentField)

	s3, err := registry.Get(faqType)
	require.NoError(t, err)
	assert.Equal(t, "answer", s3.(*GenericStrategy).ContentField)
}

