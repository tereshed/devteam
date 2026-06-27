package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrRepositoryEnvFileNotFound = errors.New("repository env file not found")

type RepositoryEnvFileRepository interface {
	// Upsert создаёт или обновляет единственный env-файл репозитория (UNIQUE по repo).
	Upsert(ctx context.Context, f *models.RepositoryEnvFile) error
	// GetByRepo возвращает запись С encrypted_content (для дешифровки/инъекции/редактирования).
	GetByRepo(ctx context.Context, repoID uuid.UUID) (*models.RepositoryEnvFile, error)
	DeleteByRepo(ctx context.Context, repoID uuid.UUID) error
}

type repositoryEnvFileRepository struct {
	db *gorm.DB
}

func NewRepositoryEnvFileRepository(db *gorm.DB) RepositoryEnvFileRepository {
	return &repositoryEnvFileRepository{db: db}
}

func (r *repositoryEnvFileRepository) Upsert(ctx context.Context, f *models.RepositoryEnvFile) error {
	if len(f.EncryptedContent) < 29 {
		return fmt.Errorf("encrypted_content too short (%d bytes), refusing to write — looks unencrypted", len(f.EncryptedContent))
	}
	existing, err := r.GetByRepo(ctx, f.ProjectRepositoryID)
	if err != nil && !errors.Is(err, ErrRepositoryEnvFileNotFound) {
		return err
	}
	if existing != nil {
		existing.FileName = f.FileName
		existing.TargetDir = f.TargetDir
		existing.EncryptedContent = f.EncryptedContent
		result := gormDB(ctx, r.db).WithContext(ctx).
			Model(existing).
			Select("file_name", "target_dir", "encrypted_content", "updated_at").
			Updates(existing)
		if result.Error != nil {
			return fmt.Errorf("failed to update repository env file %s: %w", existing.ID, result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrRepositoryEnvFileNotFound
		}
		f.ID = existing.ID
		return nil
	}
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(f).Error; err != nil {
		return fmt.Errorf("failed to create repository env file: %w", err)
	}
	return nil
}

func (r *repositoryEnvFileRepository) GetByRepo(ctx context.Context, repoID uuid.UUID) (*models.RepositoryEnvFile, error) {
	var f models.RepositoryEnvFile
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("project_repository_id = ?", repoID).
		First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRepositoryEnvFileNotFound
		}
		return nil, fmt.Errorf("failed to get repository env file for repo %s: %w", repoID, err)
	}
	return &f, nil
}

func (r *repositoryEnvFileRepository) DeleteByRepo(ctx context.Context, repoID uuid.UUID) error {
	result := gormDB(ctx, r.db).WithContext(ctx).
		Where("project_repository_id = ?", repoID).
		Delete(&models.RepositoryEnvFile{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete repository env file for repo %s: %w", repoID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRepositoryEnvFileNotFound
	}
	return nil
}
