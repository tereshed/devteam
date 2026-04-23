package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type syncStateRepository struct {
	db *gorm.DB
}

func NewSyncStateRepository(db *gorm.DB) SyncStateRepository {
	return &syncStateRepository{db: db}
}

func (r *syncStateRepository) GetByPath(ctx context.Context, projectID uuid.UUID, filePath string) (*FileSyncState, error) {
	var state FileSyncState
	err := r.db.WithContext(ctx).Where("project_id = ? AND file_path = ?", projectID, filePath).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *syncStateRepository) Upsert(ctx context.Context, state *FileSyncState) error {
	return r.db.WithContext(ctx).Save(state).Error
}

func (r *syncStateRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*FileSyncState, error) {
	var states []*FileSyncState
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&states).Error
	return states, err
}

func (r *syncStateRepository) Delete(ctx context.Context, projectID uuid.UUID, filePath string) error {
	return r.db.WithContext(ctx).Where("project_id = ? AND file_path = ?", projectID, filePath).Delete(&FileSyncState{}).Error
}
