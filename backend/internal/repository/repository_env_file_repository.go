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

// repositoryEnvFileMetaColumns — без encrypted_content (write-only: содержимое не
// возвращается в листинге/метаданных, читается только для инъекции в sandbox).
const repositoryEnvFileMetaColumns = "id, project_repository_id, file_name, target_dir, created_at, updated_at"

type RepositoryEnvFileRepository interface {
	Create(ctx context.Context, f *models.RepositoryEnvFile) error
	Update(ctx context.Context, f *models.RepositoryEnvFile) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.RepositoryEnvFile, error)
	// ListByRepo — метаданные всех файлов репо (без encrypted_content).
	ListByRepo(ctx context.Context, repoID uuid.UUID) ([]models.RepositoryEnvFile, error)
	// ListByRepoWithContent — все файлы репо С encrypted_content (для инъекции в sandbox).
	ListByRepoWithContent(ctx context.Context, repoID uuid.UUID) ([]models.RepositoryEnvFile, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type repositoryEnvFileRepository struct {
	db *gorm.DB
}

func NewRepositoryEnvFileRepository(db *gorm.DB) RepositoryEnvFileRepository {
	return &repositoryEnvFileRepository{db: db}
}

func (r *repositoryEnvFileRepository) Create(ctx context.Context, f *models.RepositoryEnvFile) error {
	if len(f.EncryptedContent) < 29 {
		return fmt.Errorf("encrypted_content too short (%d bytes), refusing to write — looks unencrypted", len(f.EncryptedContent))
	}
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(f).Error; err != nil {
		return fmt.Errorf("failed to create repository env file: %w", err)
	}
	return nil
}

func (r *repositoryEnvFileRepository) Update(ctx context.Context, f *models.RepositoryEnvFile) error {
	if len(f.EncryptedContent) < 29 {
		return fmt.Errorf("encrypted_content too short (%d bytes), refusing to write — looks unencrypted", len(f.EncryptedContent))
	}
	result := gormDB(ctx, r.db).WithContext(ctx).
		Model(f).
		Select("file_name", "target_dir", "encrypted_content", "updated_at").
		Updates(f)
	if result.Error != nil {
		return fmt.Errorf("failed to update repository env file %s: %w", f.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRepositoryEnvFileNotFound
	}
	return nil
}

func (r *repositoryEnvFileRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RepositoryEnvFile, error) {
	var f models.RepositoryEnvFile
	err := gormDB(ctx, r.db).WithContext(ctx).
		Select(repositoryEnvFileMetaColumns).
		Where("id = ?", id).
		First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRepositoryEnvFileNotFound
		}
		return nil, fmt.Errorf("failed to get repository env file %s: %w", id, err)
	}
	return &f, nil
}

func (r *repositoryEnvFileRepository) ListByRepo(ctx context.Context, repoID uuid.UUID) ([]models.RepositoryEnvFile, error) {
	var files []models.RepositoryEnvFile
	err := gormDB(ctx, r.db).WithContext(ctx).
		Select(repositoryEnvFileMetaColumns).
		Where("project_repository_id = ?", repoID).
		Order("target_dir ASC, file_name ASC").
		Find(&files).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list repository env files for repo %s: %w", repoID, err)
	}
	return files, nil
}

func (r *repositoryEnvFileRepository) ListByRepoWithContent(ctx context.Context, repoID uuid.UUID) ([]models.RepositoryEnvFile, error) {
	var files []models.RepositoryEnvFile
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("project_repository_id = ?", repoID).
		Order("target_dir ASC, file_name ASC").
		Find(&files).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list repository env files (with content) for repo %s: %w", repoID, err)
	}
	return files, nil
}

func (r *repositoryEnvFileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := gormDB(ctx, r.db).WithContext(ctx).Where("id = ?", id).Delete(&models.RepositoryEnvFile{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete repository env file %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRepositoryEnvFileNotFound
	}
	return nil
}
