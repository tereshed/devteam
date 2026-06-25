package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrProjectSecretNotFound = errors.New("project secret not found")

const projectSecretListColumns = "id, project_id, key_name, inject_as_env, description, created_at, updated_at"

type ProjectSecretRepository interface {
	Create(ctx context.Context, secret *models.ProjectSecret) error
	Update(ctx context.Context, secret *models.ProjectSecret) error
	GetByName(ctx context.Context, projectID uuid.UUID, keyName string) (*models.ProjectSecret, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetAllDecrypted(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error)
}

type projectSecretRepository struct {
	db *gorm.DB
}

func NewProjectSecretRepository(db *gorm.DB) ProjectSecretRepository {
	return &projectSecretRepository{db: db}
}

func (r *projectSecretRepository) Create(ctx context.Context, secret *models.ProjectSecret) error {
	if !models.ValidateAgentSecretKeyName(secret.KeyName) {
		return fmt.Errorf("invalid project secret key_name: %q", secret.KeyName)
	}
	if len(secret.EncryptedValue) < 29 {
		return fmt.Errorf("encrypted_value too short (%d bytes), refusing to write — looks unencrypted", len(secret.EncryptedValue))
	}
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(secret).Error; err != nil {
		return fmt.Errorf("failed to create project secret: %w", err)
	}
	return nil
}

func (r *projectSecretRepository) Update(ctx context.Context, secret *models.ProjectSecret) error {
	if len(secret.EncryptedValue) < 29 {
		return fmt.Errorf("encrypted_value too short (%d bytes), refusing to write — looks unencrypted", len(secret.EncryptedValue))
	}
	result := gormDB(ctx, r.db).WithContext(ctx).
		Model(secret).
		Select("encrypted_value", "inject_as_env", "description", "updated_at").
		Updates(secret)
	if result.Error != nil {
		return fmt.Errorf("failed to update project secret %s: %w", secret.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrProjectSecretNotFound
	}
	return nil
}

func (r *projectSecretRepository) GetByName(ctx context.Context, projectID uuid.UUID, keyName string) (*models.ProjectSecret, error) {
	var s models.ProjectSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("project_id = ? AND key_name = ?", projectID, keyName).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectSecretNotFound
		}
		return nil, fmt.Errorf("failed to get project secret %s/%s: %w", projectID, keyName, err)
	}
	return &s, nil
}

func (r *projectSecretRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error) {
	var secrets []models.ProjectSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Select(projectSecretListColumns).
		Where("project_id = ?", projectID).
		Order("key_name ASC").
		Find(&secrets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list project secrets for project %s: %w", projectID, err)
	}
	return secrets, nil
}

func (r *projectSecretRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := gormDB(ctx, r.db).WithContext(ctx).Where("id = ?", id).Delete(&models.ProjectSecret{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete project secret %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrProjectSecretNotFound
	}
	return nil
}

// GetAllDecrypted returns all secrets WITH encrypted_value for bulk decryption.
func (r *projectSecretRepository) GetAllDecrypted(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error) {
	var secrets []models.ProjectSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("key_name ASC").
		Find(&secrets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all project secrets for project %s: %w", projectID, err)
	}
	return secrets, nil
}
