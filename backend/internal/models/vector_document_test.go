package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// ContentType Tests
// ========================================

func TestContentType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		ct       ContentType
		expected bool
	}{
		{"valid_article", ContentType("article"), true},
		{"valid_product", ContentType("product"), true},
		{"valid_any_string", ContentType("custom_type"), true},
		{"empty", ContentType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ct.IsValid())
		})
	}
}

func TestContentType_String(t *testing.T) {
	ct := ContentType("article")
	assert.Equal(t, "article", ct.String())

	empty := ContentType("")
	assert.Equal(t, "", empty.String())
}

// ========================================
// NewVectorDocument Tests
// ========================================

func TestNewVectorDocument(t *testing.T) {
	contentID := "content-123"
	content := "Hello World"
	contentType := ContentType("article")

	doc := NewVectorDocument(contentID, content, contentType)

	require.NotNil(t, doc)
	assert.Equal(t, contentID, doc.ContentID)
	assert.Equal(t, content, doc.Content)
	assert.Equal(t, contentType, doc.ContentType)
	assert.Empty(t, doc.Category)
	assert.Empty(t, doc.Tags)
	assert.NotNil(t, doc.Metadata)
	assert.Empty(t, doc.Metadata)
	assert.False(t, doc.CreatedAt.IsZero())
	assert.False(t, doc.UpdatedAt.IsZero())
	assert.Equal(t, doc.CreatedAt, doc.UpdatedAt)
}

func TestNewVectorDocument_EmptyValues(t *testing.T) {
	doc := NewVectorDocument("", "", ContentType(""))

	require.NotNil(t, doc)
	assert.Empty(t, doc.ContentID)
	assert.Empty(t, doc.Content)
	assert.Empty(t, doc.ContentType)
}

// ========================================
// Fluent Interface Tests
// ========================================

func TestVectorDocument_WithCategory(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	result := doc.WithCategory("tech")

	// Fluent - возвращает тот же объект
	assert.Same(t, doc, result)
	assert.Equal(t, "tech", doc.Category)
}

func TestVectorDocument_WithTags(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	result := doc.WithTags("go", "testing", "tutorial")

	assert.Same(t, doc, result)
	assert.Len(t, doc.Tags, 3)
	assert.Contains(t, doc.Tags, "go")
	assert.Contains(t, doc.Tags, "testing")
	assert.Contains(t, doc.Tags, "tutorial")
}

func TestVectorDocument_FluentChain(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article")).
		WithCategory("programming").
		WithTags("go", "best-practices")

	assert.Equal(t, "programming", doc.Category)
	assert.Len(t, doc.Tags, 2)
}

// ========================================
// Metadata Tests
// ========================================

func TestVectorDocument_SetMetadata(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	doc.SetMetadata("author", "John Doe")
	doc.SetMetadata("views", 1000)
	doc.SetMetadata("tags", []string{"go", "testing"})

	assert.Equal(t, "John Doe", doc.Metadata["author"])
	assert.Equal(t, 1000, doc.Metadata["views"])
	assert.Equal(t, []string{"go", "testing"}, doc.Metadata["tags"])
}

func TestVectorDocument_SetMetadata_NilMap(t *testing.T) {
	doc := &VectorDocument{
		Metadata: nil, // nil metadata
	}

	doc.SetMetadata("key", "value")

	require.NotNil(t, doc.Metadata)
	assert.Equal(t, "value", doc.Metadata["key"])
}

func TestVectorDocument_SetMetadata_Override(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	doc.SetMetadata("key", "value1")
	doc.SetMetadata("key", "value2")

	assert.Equal(t, "value2", doc.Metadata["key"])
}

func TestVectorDocument_GetMetadata_Exists(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))
	doc.SetMetadata("author", "Jane")

	value, exists := doc.GetMetadata("author")

	assert.True(t, exists)
	assert.Equal(t, "Jane", value)
}

func TestVectorDocument_GetMetadata_NotExists(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	value, exists := doc.GetMetadata("nonexistent")

	assert.False(t, exists)
	assert.Nil(t, value)
}

func TestVectorDocument_GetMetadata_NilMap(t *testing.T) {
	doc := &VectorDocument{
		Metadata: nil,
	}

	value, exists := doc.GetMetadata("key")

	assert.False(t, exists)
	assert.Nil(t, value)
}

// ========================================
// Tags Tests
// ========================================

func TestVectorDocument_AddTag(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	doc.AddTag("go")
	doc.AddTag("testing")

	assert.Len(t, doc.Tags, 2)
	assert.Contains(t, doc.Tags, "go")
	assert.Contains(t, doc.Tags, "testing")
}

func TestVectorDocument_AddTag_NilSlice(t *testing.T) {
	doc := &VectorDocument{
		Tags: nil,
	}

	doc.AddTag("go")

	require.NotNil(t, doc.Tags)
	assert.Len(t, doc.Tags, 1)
	assert.Contains(t, doc.Tags, "go")
}

func TestVectorDocument_AddTag_Duplicate(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	doc.AddTag("go")
	doc.AddTag("go") // Дубликат

	// AddTag не проверяет дубликаты (простое добавление)
	assert.Len(t, doc.Tags, 2)
}

func TestVectorDocument_HasTag_True(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article")).
		WithTags("go", "testing")

	assert.True(t, doc.HasTag("go"))
	assert.True(t, doc.HasTag("testing"))
}

func TestVectorDocument_HasTag_False(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article")).
		WithTags("go")

	assert.False(t, doc.HasTag("python"))
	assert.False(t, doc.HasTag(""))
}

func TestVectorDocument_HasTag_EmptySlice(t *testing.T) {
	doc := NewVectorDocument("id", "content", ContentType("article"))

	assert.False(t, doc.HasTag("go"))
}

func TestVectorDocument_HasTag_NilSlice(t *testing.T) {
	doc := &VectorDocument{
		Tags: nil,
	}

	assert.False(t, doc.HasTag("go"))
}

// ========================================
// Timestamp Tests
// ========================================

func TestVectorDocument_Timestamps(t *testing.T) {
	before := time.Now()
	doc := NewVectorDocument("id", "content", ContentType("article"))
	after := time.Now()

	assert.True(t, doc.CreatedAt.After(before) || doc.CreatedAt.Equal(before))
	assert.True(t, doc.CreatedAt.Before(after) || doc.CreatedAt.Equal(after))
	assert.Equal(t, doc.CreatedAt, doc.UpdatedAt)
}

// ========================================
// Full Workflow Tests
// ========================================

func TestVectorDocument_FullWorkflow(t *testing.T) {
	// Создаём документ
	doc := NewVectorDocument(
		"article-123",
		"Introduction to Go programming language",
		ContentType("article"),
	).
		WithCategory("programming").
		WithTags("go", "tutorial", "beginner")

	// Добавляем метаданные
	doc.SetMetadata("author", "John Doe")
	doc.SetMetadata("views", 5000)
	doc.SetMetadata("published", true)

	// Проверяем все данные
	assert.Equal(t, "article-123", doc.ContentID)
	assert.Equal(t, "Introduction to Go programming language", doc.Content)
	assert.Equal(t, ContentType("article"), doc.ContentType)
	assert.True(t, doc.ContentType.IsValid())
	assert.Equal(t, "programming", doc.Category)
	assert.Len(t, doc.Tags, 3)
	assert.True(t, doc.HasTag("go"))
	assert.True(t, doc.HasTag("tutorial"))
	assert.False(t, doc.HasTag("python"))

	author, exists := doc.GetMetadata("author")
	assert.True(t, exists)
	assert.Equal(t, "John Doe", author)

	views, exists := doc.GetMetadata("views")
	assert.True(t, exists)
	assert.Equal(t, 5000, views)
}

