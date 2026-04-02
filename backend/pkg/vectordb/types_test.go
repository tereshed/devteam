package vectordb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

// ========================================
// SearchParams Tests
// ========================================

func TestSearchParams_Defaults(t *testing.T) {
	params := SearchParams{}

	assert.Empty(t, params.Query)
	assert.Nil(t, params.ContentTypes)
	assert.Empty(t, params.Category)
	assert.Nil(t, params.Tags)
	assert.Nil(t, params.ContentIDs)
	assert.Equal(t, 0, params.Limit)
	assert.Equal(t, float32(0), params.Alpha)
}

func TestSearchParams_FullConfig(t *testing.T) {
	params := SearchParams{
		Query:        "search query",
		ContentTypes: []models.ContentType{"article", "product"},
		Category:     "tech",
		Tags:         []string{"go", "testing"},
		ContentIDs:   []string{"id1", "id2"},
		Limit:        20,
		Alpha:        0.7,
	}

	assert.Equal(t, "search query", params.Query)
	assert.Len(t, params.ContentTypes, 2)
	assert.Equal(t, "tech", params.Category)
	assert.Contains(t, params.Tags, "go")
	assert.Equal(t, 20, params.Limit)
	assert.Equal(t, float32(0.7), params.Alpha)
}

// ========================================
// SearchResult Tests
// ========================================

func TestSearchResult_Defaults(t *testing.T) {
	result := SearchResult{}

	assert.Empty(t, result.VectorID)
	assert.Empty(t, result.ContentID)
	assert.Empty(t, result.ContentType)
	assert.Empty(t, result.Category)
	assert.Empty(t, result.Content)
	assert.Equal(t, float32(0), result.Score)
	assert.Equal(t, float32(0), result.Distance)
	assert.Nil(t, result.Metadata)
}

func TestSearchResult_FullData(t *testing.T) {
	result := SearchResult{
		VectorID:    "vec-123",
		ContentID:   "content-456",
		ContentType: models.ContentType("article"),
		Category:    "programming",
		Content:     "Sample content",
		Score:       0.95,
		Distance:    0.05,
		Metadata: map[string]interface{}{
			"author": "John",
			"tags":   []string{"go", "testing"},
		},
	}

	assert.Equal(t, "vec-123", result.VectorID)
	assert.Equal(t, "content-456", result.ContentID)
	assert.Equal(t, models.ContentType("article"), result.ContentType)
	assert.Equal(t, "programming", result.Category)
	assert.Equal(t, "Sample content", result.Content)
	assert.Equal(t, float32(0.95), result.Score)
	assert.Equal(t, float32(0.05), result.Distance)
	assert.Equal(t, "John", result.Metadata["author"])
}

// ========================================
// IndexStats Tests
// ========================================

func TestIndexStats_Defaults(t *testing.T) {
	stats := IndexStats{}

	assert.Equal(t, 0, stats.TotalProcessed)
	assert.Equal(t, 0, stats.Succeeded)
	assert.Equal(t, 0, stats.Failed)
	assert.Nil(t, stats.Errors)
}

func TestIndexStats_FullData(t *testing.T) {
	stats := IndexStats{
		TotalProcessed: 100,
		Succeeded:      95,
		Failed:         5,
		Errors:         []string{"error 1", "error 2"},
	}

	assert.Equal(t, 100, stats.TotalProcessed)
	assert.Equal(t, 95, stats.Succeeded)
	assert.Equal(t, 5, stats.Failed)
	assert.Len(t, stats.Errors, 2)
}

// ========================================
// Config Tests
// ========================================

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}

	assert.Empty(t, cfg.Host)
	assert.Empty(t, cfg.Scheme)
}

func TestConfig_FullData(t *testing.T) {
	cfg := Config{
		Host:   "weaviate:8080",
		Scheme: "https",
	}

	assert.Equal(t, "weaviate:8080", cfg.Host)
	assert.Equal(t, "https", cfg.Scheme)
}

// ========================================
// ClassName Tests
// ========================================

func TestClassName_Value(t *testing.T) {
	assert.Equal(t, "Document", ClassName)
}

