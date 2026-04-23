package indexer

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrQueryTooLong   = errors.New("search query is too long")
	ErrIndexNotReady  = errors.New("code index is not ready")
)

// IndexingRequest запрос на индексацию проекта
type IndexingRequest struct {
	ProjectID uuid.UUID
	RepoPath  string // Абсолютный путь к локальному клону репозитория
}

// FileTask задача на обработку файла в Pipeline
type FileTask struct {
	ProjectID    uuid.UUID
	RelativePath string
	AbsolutePath string
	Language     string
	Size         int64
}

// Chunk результат разбиения файла
type Chunk struct {
	Content   string
	FilePath  string
	Language  string
	StartLine int
	EndLine   int
	Symbol    string
	Hash      string
	FileHash  string  // Хеш всего файла для обновления SyncState
	Score     float32 // Релевантность чанка (0.0 - 1.0)
}

// FileResult результат обработки файла воркером
type FileResult struct {
	ProjectID    uuid.UUID
	RelativePath string
	ContentHash  string
	Chunks       []Chunk
	Unchanged    bool
	Error        error
}

// CodeIndexer интерфейс основного компонента индексации
type CodeIndexer interface {
	// IndexProject запускает процесс индексации всего проекта
	IndexProject(ctx context.Context, req IndexingRequest) error

	// SearchContext выполняет контекстный поиск по проиндексированному коду проекта
	SearchContext(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]Chunk, error)
}
