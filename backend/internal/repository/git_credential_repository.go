package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrGitCredentialNotFound = errors.New("git credential not found")

const gitCredentialListColumns = "id, user_id, provider, auth_type, label, created_at, updated_at"

// GitCredentialRepository CRUD по зашифрованным Git credentials (bytea на диске — уже зашифровано сервисом).
type GitCredentialRepository interface {
	Create(ctx context.Context, cred *models.GitCredential) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.GitCredential, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.GitCredential, error)
	ListByUserIDAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitCredentialProvider) ([]models.GitCredential, error)
	Update(ctx context.Context, cred *models.GitCredential) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type gitCredentialRepository struct {
	db *gorm.DB
}

// NewGitCredentialRepository создаёт репозиторий Git credentials.
func NewGitCredentialRepository(db *gorm.DB) GitCredentialRepository {
	return &gitCredentialRepository{db: db}
}

func (r *gitCredentialRepository) Create(ctx context.Context, cred *models.GitCredential) error {
	if err := r.db.WithContext(ctx).Create(cred).Error; err != nil {
		return fmt.Errorf("failed to create git credential: %w", err)
	}
	return nil
}

func (r *gitCredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GitCredential, error) {
	var cred models.GitCredential
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cred).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGitCredentialNotFound
		}
		return nil, fmt.Errorf("failed to get git credential by id: %w", err)
	}
	return &cred, nil
}

func (r *gitCredentialRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.GitCredential, error) {
	var creds []models.GitCredential
	err := r.db.WithContext(ctx).
		Select(gitCredentialListColumns).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&creds).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list git credentials by user id: %w", err)
	}
	return creds, nil
}

func (r *gitCredentialRepository) ListByUserIDAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitCredentialProvider) ([]models.GitCredential, error) {
	var creds []models.GitCredential
	err := r.db.WithContext(ctx).
		Select(gitCredentialListColumns).
		Where("user_id = ? AND provider = ?", userID, string(provider)).
		Order("created_at DESC").
		Find(&creds).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list git credentials by user and provider: %w", err)
	}
	return creds, nil
}

func (r *gitCredentialRepository) Update(ctx context.Context, cred *models.GitCredential) error {
	if err := r.db.WithContext(ctx).Save(cred).Error; err != nil {
		return fmt.Errorf("failed to update git credential: %w", err)
	}
	return nil
}

func (r *gitCredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.GitCredential{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete git credential: %w", err)
	}
	return nil
}
