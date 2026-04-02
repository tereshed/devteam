package vectorloader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/pkg/vectordb"
	"github.com/wibe-flutter-gin-template/backend/pkg/vectordb/strategy"
)

// ========================================
// Mocks
// ========================================

// MockVectorRepository мок для VectorRepository
type MockVectorRepository struct {
	mock.Mock
}

func (m *MockVectorRepository) BatchCreate(ctx context.Context, docs []*models.VectorDocument) (*vectordb.IndexStats, error) {
	args := m.Called(ctx, docs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*vectordb.IndexStats), args.Error(1)
}

func (m *MockVectorRepository) CountByContentType(ctx context.Context, contentType models.ContentType, category string) (int64, error) {
	args := m.Called(ctx, contentType, category)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockVectorRepository) DeleteByContentType(ctx context.Context, contentType models.ContentType, category string) error {
	args := m.Called(ctx, contentType, category)
	return args.Error(0)
}

// MockDataSource мок для DataSource
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) GetItems(ctx context.Context, category string) ([]interface{}, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockDataSource) GetItemID(item interface{}) string {
	args := m.Called(item)
	return args.String(0)
}

func (m *MockDataSource) GetItemName(item interface{}) string {
	args := m.Called(item)
	return args.String(0)
}

// ========================================
// Setup Helpers
// ========================================

func setupTest() (*VectorLoader, *MockVectorRepository, *MockDataSource) {
	mockRepo := new(MockVectorRepository)
	mockDataSource := new(MockDataSource)
	loader := NewVectorLoader(mockRepo)

	return loader, mockRepo, mockDataSource
}

func setupTestStrategy(contentType models.ContentType) {
	// Регистрируем тестовую стратегию
	testStrategy := strategy.NewGenericStrategy("content", "id")
	strategy.RegisterStrategy(contentType, testStrategy)
}

// ========================================
// NewVectorLoader Tests
// ========================================

func TestNewVectorLoader(t *testing.T) {
	mockRepo := new(MockVectorRepository)

	loader := NewVectorLoader(mockRepo)

	assert.NotNil(t, loader)
	assert.NotNil(t, loader.vectorRepo)
}

// ========================================
// LoadFromDataSource Tests
// ========================================

func TestLoadFromDataSource_AlreadyIndexed(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("article")

	// Уже проиндексировано 10 документов
	mockRepo.On("CountByContentType", ctx, contentType, "tech").Return(int64(10), nil)

	result, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "tech")

	require.NoError(t, err)
	assert.Equal(t, 10, result.SkippedItems)
	assert.Equal(t, 0, result.IndexedItems)
	mockRepo.AssertExpectations(t)
}

func TestLoadFromDataSource_EmptySource(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("article")

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return([]interface{}{}, nil)

	result, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalItems)
	assert.Equal(t, 0, result.IndexedItems)
	mockRepo.AssertExpectations(t)
	mockDataSource.AssertExpectations(t)
}

func TestLoadFromDataSource_CountError(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("article")

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), errors.New("db error"))

	_, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to count indexed items")
	mockRepo.AssertExpectations(t)
}

func TestLoadFromDataSource_GetItemsError(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("article")

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return(nil, errors.New("source error"))

	_, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get items from data source")
	mockRepo.AssertExpectations(t)
	mockDataSource.AssertExpectations(t)
}

func TestLoadFromDataSource_NoStrategy(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("unknown_type_no_strategy")

	items := []interface{}{
		map[string]interface{}{"id": "1", "content": "test"},
	}

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return(items, nil)

	_, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get strategy")
	mockRepo.AssertExpectations(t)
	mockDataSource.AssertExpectations(t)
}

func TestLoadFromDataSource_Success(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("test_article")

	// Регистрируем стратегию
	setupTestStrategy(contentType)

	items := []interface{}{
		map[string]interface{}{"id": "1", "content": "Article 1"},
		map[string]interface{}{"id": "2", "content": "Article 2"},
	}

	mockRepo.On("CountByContentType", ctx, contentType, "tech").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "tech").Return(items, nil)
	mockDataSource.On("GetItemID", items[0]).Return("1")
	mockDataSource.On("GetItemName", items[0]).Return("Article 1")
	mockDataSource.On("GetItemID", items[1]).Return("2")
	mockDataSource.On("GetItemName", items[1]).Return("Article 2")

	mockRepo.On("BatchCreate", ctx, mock.AnythingOfType("[]*models.VectorDocument")).
		Return(&vectordb.IndexStats{
			TotalProcessed: 2,
			Succeeded:      2,
			Failed:         0,
		}, nil)

	result, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "tech")

	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalItems)
	assert.Equal(t, 2, result.IndexedItems)
	assert.Equal(t, 0, result.FailedItems)
	assert.Equal(t, contentType, result.ContentType)
	assert.Equal(t, "tech", result.Category)
	mockRepo.AssertExpectations(t)
	mockDataSource.AssertExpectations(t)
}

func TestLoadFromDataSource_PartialFailure(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("test_article2")

	setupTestStrategy(contentType)

	items := []interface{}{
		map[string]interface{}{"id": "1", "content": "Valid"},
		map[string]interface{}{"id": "", "content": ""}, // Невалидный
	}

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return(items, nil)
	mockDataSource.On("GetItemID", items[0]).Return("1")
	mockDataSource.On("GetItemName", items[0]).Return("Valid")
	mockDataSource.On("GetItemID", items[1]).Return("")
	mockDataSource.On("GetItemName", items[1]).Return("Invalid")

	mockRepo.On("BatchCreate", ctx, mock.AnythingOfType("[]*models.VectorDocument")).
		Return(&vectordb.IndexStats{
			TotalProcessed: 1,
			Succeeded:      1,
			Failed:         0,
		}, nil)

	result, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalItems)
	assert.Equal(t, 1, result.IndexedItems)
	assert.Equal(t, 1, result.FailedItems) // Один невалидный
	assert.NotEmpty(t, result.Errors)
	mockRepo.AssertExpectations(t)
}

func TestLoadFromDataSource_BatchCreateError(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("test_article3")

	setupTestStrategy(contentType)

	items := []interface{}{
		map[string]interface{}{"id": "1", "content": "Test"},
	}

	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return(items, nil)
	mockDataSource.On("GetItemID", items[0]).Return("1")
	mockDataSource.On("GetItemName", items[0]).Return("Test")

	mockRepo.On("BatchCreate", ctx, mock.AnythingOfType("[]*models.VectorDocument")).
		Return(nil, errors.New("batch error"))

	_, err := loader.LoadFromDataSource(ctx, mockDataSource, contentType, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch indexing failed")
	mockRepo.AssertExpectations(t)
}

// ========================================
// RebuildIndex Tests
// ========================================

func TestRebuildIndex_Success(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("test_rebuild")

	setupTestStrategy(contentType)

	items := []interface{}{
		map[string]interface{}{"id": "1", "content": "Rebuilt"},
	}

	// Удаление
	mockRepo.On("DeleteByContentType", ctx, contentType, "").Return(nil)

	// Загрузка заново
	mockRepo.On("CountByContentType", ctx, contentType, "").Return(int64(0), nil)
	mockDataSource.On("GetItems", ctx, "").Return(items, nil)
	mockDataSource.On("GetItemID", items[0]).Return("1")
	mockDataSource.On("GetItemName", items[0]).Return("Rebuilt")
	mockRepo.On("BatchCreate", ctx, mock.AnythingOfType("[]*models.VectorDocument")).
		Return(&vectordb.IndexStats{Succeeded: 1}, nil)

	result, err := loader.RebuildIndex(ctx, mockDataSource, contentType, "")

	require.NoError(t, err)
	assert.Equal(t, 1, result.IndexedItems)
	mockRepo.AssertExpectations(t)
}

func TestRebuildIndex_DeleteError(t *testing.T) {
	loader, mockRepo, mockDataSource := setupTest()
	ctx := context.Background()
	contentType := models.ContentType("article")

	mockRepo.On("DeleteByContentType", ctx, contentType, "").Return(errors.New("delete error"))

	_, err := loader.RebuildIndex(ctx, mockDataSource, contentType, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete existing documents")
	mockRepo.AssertExpectations(t)
}

// ========================================
// GetIndexStatistics Tests
// ========================================

func TestGetIndexStatistics_Success(t *testing.T) {
	loader, mockRepo, _ := setupTest()
	ctx := context.Background()

	contentTypes := []models.ContentType{"article", "product"}

	mockRepo.On("CountByContentType", ctx, models.ContentType("article"), "").Return(int64(100), nil)
	mockRepo.On("CountByContentType", ctx, models.ContentType("product"), "").Return(int64(50), nil)

	stats, err := loader.GetIndexStatistics(ctx, contentTypes)

	require.NoError(t, err)
	assert.Equal(t, int64(100), stats["indexed_article"])
	assert.Equal(t, int64(50), stats["indexed_product"])
	mockRepo.AssertExpectations(t)
}

func TestGetIndexStatistics_Error(t *testing.T) {
	loader, mockRepo, _ := setupTest()
	ctx := context.Background()

	contentTypes := []models.ContentType{"article"}

	mockRepo.On("CountByContentType", ctx, models.ContentType("article"), "").Return(int64(0), errors.New("db error"))

	_, err := loader.GetIndexStatistics(ctx, contentTypes)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to count article")
	mockRepo.AssertExpectations(t)
}

func TestGetIndexStatistics_Empty(t *testing.T) {
	loader, _, _ := setupTest()
	ctx := context.Background()

	stats, err := loader.GetIndexStatistics(ctx, []models.ContentType{})

	require.NoError(t, err)
	assert.Empty(t, stats)
}

// ========================================
// GetTotalCount Tests
// ========================================

func TestGetTotalCount_Success(t *testing.T) {
	loader, mockRepo, _ := setupTest()
	ctx := context.Background()

	mockRepo.On("CountByContentType", ctx, models.ContentType(""), "").Return(int64(500), nil)

	count, err := loader.GetTotalCount(ctx)

	require.NoError(t, err)
	assert.Equal(t, int64(500), count)
	mockRepo.AssertExpectations(t)
}

func TestGetTotalCount_Error(t *testing.T) {
	loader, mockRepo, _ := setupTest()
	ctx := context.Background()

	mockRepo.On("CountByContentType", ctx, models.ContentType(""), "").Return(int64(0), errors.New("db error"))

	_, err := loader.GetTotalCount(ctx)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// ========================================
// LoadResult Tests
// ========================================

func TestLoadResult_Fields(t *testing.T) {
	result := LoadResult{
		ContentType:  models.ContentType("article"),
		Category:     "tech",
		TotalItems:   100,
		IndexedItems: 95,
		SkippedItems: 0,
		FailedItems:  5,
		Duration:     5 * time.Second,
		Errors:       []string{"error 1", "error 2"},
	}

	assert.Equal(t, models.ContentType("article"), result.ContentType)
	assert.Equal(t, "tech", result.Category)
	assert.Equal(t, 100, result.TotalItems)
	assert.Equal(t, 95, result.IndexedItems)
	assert.Equal(t, 5, result.FailedItems)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Len(t, result.Errors, 2)
}

