package indexer

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrQueryTooLong  = errors.New("search query is too long")
	ErrIndexNotReady = errors.New("code index is not ready")
)

// IndexingRequest запрос на индексацию проекта
type IndexingRequest struct {
	ProjectID uuid.UUID
	RepoPath  string // Абсолютный путь к локальному клону репозитория
	// PathPrefix — мульти-репо: префикс репозитория (slug), которым префиксуются
	// относительные пути файлов в пределах одного project-namespace (например
	// "core/cmd/main.go"). Пусто — индексация без префикса (одно-репо/legacy).
	// Cleanup удалённых файлов при непустом префиксе ограничивается этим префиксом,
	// чтобы переиндексация одного репо не сносила записи соседних.
	PathPrefix string
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

	// PruneToPrefixes удаляет из индекса (vector + sync state) файлы проекта, чьи пути НЕ
	// лежат ни под одним из переданных репо-префиксов (`<slug>/`). Используется при
	// переходе проекта на мульти-репо: вычищает legacy не-префиксованные записи, оставшиеся
	// от индексации до перехода. Пустой список префиксов — no-op (защита от полного сноса).
	PruneToPrefixes(ctx context.Context, projectID uuid.UUID, prefixes []string) error
}
