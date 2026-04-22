package indexer

import (
	"context"

	"github.com/google/uuid"
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
	FileHash  string // Хеш всего файла для обновления SyncState
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
}

// ConversationIndexer интерфейс для индексации сообщений чатов
type ConversationIndexer interface {
	// Start запускает прослушивание событий
	Start(ctx context.Context) error
	// IndexMessage индексирует одно сообщение
	IndexMessage(ctx context.Context, projectID, conversationID, messageID uuid.UUID) error
	// DeleteMessage удаляет сообщение из индекса
	DeleteMessage(ctx context.Context, projectID, messageID uuid.UUID) error
	// DeleteConversation удаляет весь чат из индекса
	DeleteConversation(ctx context.Context, projectID, conversationID uuid.UUID) error
	// IndexProjectConversations индексирует все чаты проекта
	IndexProjectConversations(ctx context.Context, projectID uuid.UUID) error
	// Stop останавливает прослушивание событий и дожидается завершения текущих задач
	Stop()
}
