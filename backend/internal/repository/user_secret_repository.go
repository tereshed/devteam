package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUserSecretNotFound = errors.New("user secret not found")

const userSecretListColumns = "id, user_id, key_name, created_at, updated_at"

type UserSecretRepository interface {
	Create(ctx context.Context, secret *models.UserSecret) error
	Update(ctx context.Context, secret *models.UserSecret) error
	GetByName(ctx context.Context, userID uuid.UUID, keyName string) (*models.UserSecret, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.UserSecret, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetAllDecrypted(ctx context.Context, userID uuid.UUID) ([]models.UserSecret, error)
}

type userSecretRepository struct {
	db *gorm.DB
}

func NewUserSecretRepository(db *gorm.DB) UserSecretRepository {
	return &userSecretRepository{db: db}
}

func (r *userSecretRepository) Create(ctx context.Context, secret *models.UserSecret) error {
	if !models.ValidateAgentSecretKeyName(secret.KeyName) {
		return fmt.Errorf("invalid user secret key_name: %q", secret.KeyName)
	}
	if len(secret.EncryptedValue) < 29 {
		return fmt.Errorf("encrypted_value too short (%d bytes), refusing to write — looks unencrypted", len(secret.EncryptedValue))
	}
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(secret).Error; err != nil {
		return fmt.Errorf("failed to create user secret: %w", err)
	}
	return nil
}

func (r *userSecretRepository) Update(ctx context.Context, secret *models.UserSecret) error {
	if len(secret.EncryptedValue) < 29 {
		return fmt.Errorf("encrypted_value too short (%d bytes), refusing to write — looks unencrypted", len(secret.EncryptedValue))
	}
	result := gormDB(ctx, r.db).WithContext(ctx).
		Model(secret).
		Select("encrypted_value", "updated_at").
		Updates(secret)
	if result.Error != nil {
		return fmt.Errorf("failed to update user secret %s: %w", secret.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserSecretNotFound
	}
	return nil
}

func (r *userSecretRepository) GetByName(ctx context.Context, userID uuid.UUID, keyName string) (*models.UserSecret, error) {
	var s models.UserSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND key_name = ?", userID, keyName).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserSecretNotFound
		}
		return nil, fmt.Errorf("failed to get user secret %s/%s: %w", userID, keyName, err)
	}
	return &s, nil
}

func (r *userSecretRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.UserSecret, error) {
	var secrets []models.UserSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Select(userSecretListColumns).
		Where("user_id = ?", userID).
		Order("key_name ASC").
		Find(&secrets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list user secrets for user %s: %w", userID, err)
	}
	return secrets, nil
}

func (r *userSecretRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := gormDB(ctx, r.db).WithContext(ctx).Where("id = ?", id).Delete(&models.UserSecret{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete user secret %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserSecretNotFound
	}
	return nil
}

func (r *userSecretRepository) GetAllDecrypted(ctx context.Context, userID uuid.UUID) ([]models.UserSecret, error) {
	var secrets []models.UserSecret
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("user_id = ?", userID).
		Order("key_name ASC").
		Find(&secrets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all user secrets for user %s: %w", userID, err)
	}
	return secrets, nil
}
