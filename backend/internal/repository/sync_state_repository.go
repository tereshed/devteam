package repository

import (
	"context"

	"github.com/google/uuid"
)

// FileSyncState представляет состояние синхронизации отдельного файла
type FileSyncState struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null;index:idx_file_sync_project_path,unique" json:"project_id"`
	FilePath    string    `gorm:"type:varchar(1024);not null;index:idx_file_sync_project_path,unique" json:"file_path"`
	ContentHash string    `gorm:"type:varchar(64);not null" json:"content_hash"`
	LastIndexed int64     `gorm:"type:bigint;not null" json:"last_indexed"` // Unix timestamp
}

// SyncStateRepository определяет интерфейс для хранения состояния индексации
type SyncStateRepository interface {
	// GetByPath получает состояние для конкретного файла
	GetByPath(ctx context.Context, projectID uuid.UUID, filePath string) (*FileSyncState, error)
	
	// Upsert обновляет или создает состояние файла
	Upsert(ctx context.Context, state *FileSyncState) error
	
	// ListByProject возвращает все проиндексированные файлы проекта
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*FileSyncState, error)
	
	// Delete удаляет состояние файла
	Delete(ctx context.Context, projectID uuid.UUID, filePath string) error
}
