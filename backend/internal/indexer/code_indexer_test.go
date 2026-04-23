package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/parser"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSyncRepo is a mock of SyncStateRepository
type MockSyncRepo struct {
	mock.Mock
}

func (m *MockSyncRepo) GetByPath(ctx context.Context, projectID uuid.UUID, filePath string) (*repository.FileSyncState, error) {
	args := m.Called(ctx, projectID, filePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.FileSyncState), args.Error(1)
}

func (m *MockSyncRepo) Upsert(ctx context.Context, state *repository.FileSyncState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockSyncRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*repository.FileSyncState, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).([]*repository.FileSyncState), args.Error(1)
}

func (m *MockSyncRepo) Delete(ctx context.Context, projectID uuid.UUID, filePath string) error {
	args := m.Called(ctx, projectID, filePath)
	return args.Error(0)
}

// MockVectorRepo is a mock of VectorRepository
type MockVectorRepo struct {
	mock.Mock
}

func (m *MockVectorRepo) Create(ctx context.Context, projectID string, doc *models.VectorDocument) (string, error) {
	args := m.Called(ctx, projectID, doc)
	return args.String(0), args.Error(1)
}

func (m *MockVectorRepo) BatchCreate(ctx context.Context, projectID string, docs []*models.VectorDocument) (*vectordb.IndexStats, error) {
	args := m.Called(ctx, projectID, docs)
	return args.Get(0).(*vectordb.IndexStats), args.Error(1)
}

func (m *MockVectorRepo) DeleteByContentID(ctx context.Context, projectID string, contentID string) error {
	args := m.Called(ctx, projectID, contentID)
	return args.Error(0)
}

func (m *MockVectorRepo) Get(ctx context.Context, projectID string, id string) (*models.VectorDocument, error) {
	args := m.Called(ctx, projectID, id)
	return args.Get(0).(*models.VectorDocument), args.Error(1)
}

func (m *MockVectorRepo) Update(ctx context.Context, projectID string, id string, doc *models.VectorDocument) error {
	args := m.Called(ctx, projectID, id, doc)
	return args.Error(0)
}

func (m *MockVectorRepo) Delete(ctx context.Context, projectID string, id string) error {
	args := m.Called(ctx, projectID, id)
	return args.Error(0)
}

func (m *MockVectorRepo) DeleteByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) error {
	args := m.Called(ctx, projectID, contentType, category)
	return args.Error(0)
}

func (m *MockVectorRepo) Search(ctx context.Context, projectID string, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, params)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) SemanticSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, query, category, limit)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) KeywordSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, query, category, limit)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) CountByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) (int64, error) {
	args := m.Called(ctx, projectID, contentType, category)
	return args.Get(0).(int64), args.Error(1)
}

// MockParser is a mock of CodeParser
type MockParser struct {
	mock.Mock
}

func (m *MockParser) Parse(ctx context.Context, language string, content []byte) ([]parser.Node, error) {
	args := m.Called(ctx, language, content)
	var nodes []parser.Node
	if args.Get(0) != nil {
		nodes = args.Get(0).([]parser.Node)
	}
	return nodes, args.Error(1)
}

func (m *MockParser) GetLanguageByExtension(ext string) string {
	args := m.Called(ext)
	return args.String(0)
}

func (m *MockParser) Reset() {
	m.Called()
}

func TestCodeIndexer_MaskSecrets(t *testing.T) {
	idx := &codeIndexer{}
	ctx := context.Background()
	
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			"API Key",
			`apiKey = "abcdef1234567890"`,
			`[MASKED_SECRET]` + "\n",
		},
		{
			"Bearer Token",
			`Authorization: Bearer abcdef12345678901234567890`,
			`Authorization: [MASKED_SECRET]` + "\n",
		},
		{
			"GitHub Token",
			`ghp_1234567890abcdef1234567890abcdef1234`,
			`[MASKED_SECRET]` + "\n",
		},
		{
			"Slack Token",
			`xoxb-dummy-token-for-test-1234567890`,
			`[MASKED_SECRET]` + "\n",
		},
		{
			"API Key with spaces",
			`apiKey: "abcdef1234567890"`,
			`[MASKED_SECRET]` + "\n",
		},
		{
			"Secret with spaces around colon",
			`secret : "abcdef1234567890"`,
			`[MASKED_SECRET]` + "\n",
		},
		{
			"No Secret",
			`Hello world`,
			`Hello world` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := idx.maskSecrets(ctx, strings.NewReader(tt.content))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestCodeIndexer_ProcessFile_LongLines(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "indexer-test")
	defer os.RemoveAll(tmpDir)

	idx := &codeIndexer{}
	
	longLine := strings.Repeat("a", MaxLineLength + 1)
	filePath := filepath.Join(tmpDir, "long_line.txt")
	os.WriteFile(filePath, []byte(longLine), 0644)

	task := FileTask{
		AbsolutePath: filePath,
		RelativePath: "long_line.txt",
	}

	res, err := idx.processFile(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "abnormally long line")
	assert.Empty(t, res.Chunks)
}

func TestCodeIndexer_SplitByTokens(t *testing.T) {
	// Инициализируем реальный тиктокена для теста
	indexerObj, _ := NewCodeIndexer(nil, nil, func() parser.CodeParser { return nil }, 1, nil)
	idx := indexerObj.(*codeIndexer)
	
	content := "This is a test content that should be small enough."
	chunks := idx.splitByTokens(context.Background(), nil, content, "test.go", "go", 1, "main", 0)
	
	assert.Len(t, chunks, 1)
	assert.Equal(t, content, chunks[0].Content)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, "main", chunks[0].Symbol)
	assert.NotEmpty(t, chunks[0].Hash)

	// Тест на большой контент (разбиение по токенам)
	contentLarge := strings.Repeat("test ", 1000)
	chunksLarge := idx.splitByTokens(context.Background(), nil, contentLarge, "test.go", "go", 1, "main", 0)
	assert.True(t, len(chunksLarge) > 1)
	for _, c := range chunksLarge {
		assert.NotEmpty(t, c.Hash)
	}
}

func TestCodeIndexer_RecursiveChunking(t *testing.T) {
	mockParser := new(MockParser)
	indexerObj, _ := NewCodeIndexer(nil, nil, func() parser.CodeParser { return mockParser }, 1, nil)
	idx := indexerObj.(*codeIndexer)

	// Создадим контент, который точно больше 512 токенов
	largeContent := "func main() {\n" + strings.Repeat("  fmt.Println(\"hello\")\n", 300) + "}"
	part1 := "func main() {\n" + strings.Repeat("  fmt.Println(\"hello\")\n", 150)
	part2 := strings.Repeat("  fmt.Println(\"hello\")\n", 150) + "}"
	
	// Ожидаем, что парсер вернет этот блок
	mockParser.On("Parse", mock.Anything, "go", mock.Anything).Return([]parser.Node{
		{Content: largeContent, StartLine: 1, Symbol: "main"},
	}, nil).Once()
	
	// При рекурсивном вызове внутри splitByTokens (depth=0 -> depth=1)
	mockParser.On("Parse", mock.Anything, "go", mock.Anything).Return([]parser.Node{
		{Content: part1, StartLine: 1, Symbol: "main_part1"},
		{Content: part2, StartLine: 151, Symbol: "main_part2"},
	}, nil).Once()

	// Для part1 и part2
	mockParser.On("Parse", mock.Anything, "go", mock.Anything).Return([]parser.Node{
		{Content: part1, StartLine: 1, Symbol: "main_part1"},
	}, nil)
	mockParser.On("Parse", mock.Anything, "go", mock.Anything).Return([]parser.Node{
		{Content: part2, StartLine: 1, Symbol: "main_part2"},
	}, nil)

	mockParser.On("Reset").Return()

	chunks := idx.splitByTokens(context.Background(), mockParser, largeContent, "main.go", "go", 1, "main", 0)
	
	assert.True(t, len(chunks) >= 2)
}

func TestCodeIndexer_SearchContext(t *testing.T) {
	mockVectorRepo := new(MockVectorRepo)
	indexerObj, _ := NewCodeIndexer(nil, mockVectorRepo, func() parser.CodeParser { return nil }, 1, nil)
	idx := indexerObj.(*codeIndexer)
	ctx := context.Background()
	projectID := uuid.New()

	t.Run("Empty query", func(t *testing.T) {
		chunks, err := idx.SearchContext(ctx, projectID, "", 10)
		assert.NoError(t, err)
		assert.Empty(t, chunks)
	})

	t.Run("Query too long", func(t *testing.T) {
		longQuery := strings.Repeat("a", MaxSearchQueryLen+1)
		chunks, err := idx.SearchContext(ctx, projectID, longQuery, 10)
		assert.ErrorIs(t, err, ErrQueryTooLong)
		assert.Nil(t, chunks)
	})

	t.Run("Successful search", func(t *testing.T) {
		query := "how to use this"
		limit := 5

		mockResults := []*vectordb.SearchResult{
			{
				Content:  "chunk 1",
				Distance: 0.1, // Certainty 0.95
				Metadata: map[string]interface{}{
					"file_path":    "test.go",
					"language":     "go",
					"symbol":       "Func1",
					"content_hash": "hash1",
					"start_line":   10.0,
					"end_line":     20.0,
				},
			},
			{
				Content:  "chunk 2",
				Distance: 0.5, // Certainty 0.75
				Metadata: map[string]interface{}{
					"file_path":    "other.go",
					"language":     "go",
					"symbol":       "Func2",
					"content_hash": "hash2",
					"start_line":   30.0,
					"end_line":     40.0,
				},
			},
			{
				Content:  "noise",
				Distance: 0.8, // Certainty 0.6 - should be filtered out
				Metadata: map[string]interface{}{
					"file_path":    "noise.go",
					"language":     "go",
					"symbol":       "Noise",
					"content_hash": "hash3",
					"start_line":   1.0,
					"end_line":     5.0,
				},
			},
		}

		mockVectorRepo.On("Search", ctx, projectID.String(), mock.MatchedBy(func(p *vectordb.SearchParams) bool {
			return p.Query == query && p.Limit == limit && p.ProjectID == projectID.String()
		})).Return(mockResults, nil)

		chunks, err := idx.SearchContext(ctx, projectID, query, limit)
		assert.NoError(t, err)
		assert.Len(t, chunks, 2)
		assert.Equal(t, "chunk 1", chunks[0].Content)
		assert.Equal(t, "test.go", chunks[0].FilePath)
		assert.Equal(t, 10, chunks[0].StartLine)
		assert.Equal(t, "chunk 2", chunks[1].Content)
		assert.Equal(t, 30, chunks[1].StartLine)
	})

	t.Run("Missing metadata fields", func(t *testing.T) {
		query := "missing metadata"
		mockResults := []*vectordb.SearchResult{
			{
				Content:  "partial chunk",
				Distance: 0.1,
				Metadata: map[string]interface{}{
					"file_path": "test.go",
					// language, symbol, hash missing
				},
			},
			{
				Content:  "no metadata",
				Distance: 0.1,
				Metadata: nil,
			},
		}

		mockVectorRepo.On("Search", ctx, projectID.String(), mock.Anything).Return(mockResults, nil).Once()

		chunks, err := idx.SearchContext(ctx, projectID, query, 10)
		assert.NoError(t, err)
		assert.Len(t, chunks, 2)
		assert.Equal(t, "test.go", chunks[0].FilePath)
		assert.Equal(t, "", chunks[0].Language)
		assert.Equal(t, "", chunks[1].FilePath)
	})

	t.Run("Vector DB error", func(t *testing.T) {
		mockVectorRepo.On("Search", ctx, projectID.String(), mock.Anything).
			Return([]*vectordb.SearchResult{}, fmt.Errorf("db error")).Once()

		chunks, err := idx.SearchContext(ctx, projectID, "query", 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector search failed")
		assert.Nil(t, chunks)
	})
}

func TestCodeIndexer_MaskSecrets_ReDoS(t *testing.T) {
	idx := &codeIndexer{}
	
	// Очень длинная строка для проверки построчной обработки
	longLine := "apiKey = \"abcdef1234567890\" " + strings.Repeat("a", 10000)
	content := longLine + "\n" + "normal line"
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	masked, err := idx.maskSecrets(ctx, strings.NewReader(content))
	assert.NoError(t, err)
	assert.Contains(t, masked, "[MASKED_SECRET]")
	assert.Contains(t, masked, "normal line")
}
