package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProjectSyncState хранит общее состояние индексации проекта
type ProjectSyncState struct {
	ProjectID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"project_id"`
	ActiveRunID    string    `gorm:"type:varchar(64)" json:"active_run_id"`
	CurrentState   string    `gorm:"type:varchar(20)" json:"current_state"` // idle | indexing | failed
	Progress       float64   `gorm:"type:double" json:"progress"`
	StartTime      time.Time `json:"start_time"`
	LastError      string    `gorm:"type:text" json:"last_error"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// FailedOperation представляет запись в Dead Letter Queue (DLQ) для повторной обработки
type FailedOperation struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Operation   string    `gorm:"type:varchar(50);not null" json:"operation"` // index_code | delete_code | etc
	EntityID    string    `gorm:"type:varchar(1024);not null" json:"entity_id"` // filePath or taskID
	LastError   string    `gorm:"type:text" json:"last_error"`
	RetryCount  int       `gorm:"type:int;default:0" json:"retry_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

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
	// --- File Sync ---
	GetByPath(ctx context.Context, projectID uuid.UUID, filePath string) (*FileSyncState, error)
	Upsert(ctx context.Context, state *FileSyncState) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*FileSyncState, error)
	Delete(ctx context.Context, projectID uuid.UUID, filePath string) error

	// --- Project Sync State ---
	GetProjectState(ctx context.Context, projectID uuid.UUID) (*ProjectSyncState, error)
	UpsertProjectState(ctx context.Context, state *ProjectSyncState) error

	// --- Failed Operations (DLQ) ---
	AddFailedOperation(ctx context.Context, op *FailedOperation) error
	ListFailedOperations(ctx context.Context, projectID uuid.UUID) ([]*FailedOperation, error)
	DeleteFailedOperation(ctx context.Context, opID uuid.UUID) error
}
