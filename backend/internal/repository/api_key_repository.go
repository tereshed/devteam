package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrApiKeyNotFound = errors.New("api key not found")
)

// ApiKeyRepository определяет интерфейс для работы с API-ключами
type ApiKeyRepository interface {
	Create(ctx context.Context, apiKey *models.ApiKey) error
	GetByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ApiKey, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// apiKeyRepository реализация ApiKeyRepository
type apiKeyRepository struct {
	db *gorm.DB
}

// NewApiKeyRepository создает новый репозиторий API-ключей
func NewApiKeyRepository(db *gorm.DB) ApiKeyRepository {
	return &apiKeyRepository{db: db}
}

// Create создает новый API-ключ
func (r *apiKeyRepository) Create(ctx context.Context, apiKey *models.ApiKey) error {
	return r.db.WithContext(ctx).Create(apiKey).Error
}

// GetByKeyHash получает API-ключ по хешу ключа
func (r *apiKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error) {
	var apiKey models.ApiKey
	if err := r.db.WithContext(ctx).Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApiKeyNotFound
		}
		return nil, err
	}
	return &apiKey, nil
}

// GetByID получает API-ключ по ID
func (r *apiKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ApiKey, error) {
	var apiKey models.ApiKey
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApiKeyNotFound
		}
		return nil, err
	}
	return &apiKey, nil
}

// ListByUserID получает все API-ключи пользователя (неотозванные)
func (r *apiKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	var apiKeys []models.ApiKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").
		Find(&apiKeys).Error; err != nil {
		return nil, err
	}
	return apiKeys, nil
}

// Revoke отзывает API-ключ
func (r *apiKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&models.ApiKey{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrApiKeyNotFound
	}
	return nil
}

// RevokeAllForUser отзывает все API-ключи пользователя
func (r *apiKeyRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.ApiKey{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// UpdateLastUsed обновляет время последнего использования
func (r *apiKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_at", now).Error
}

// Delete удаляет API-ключ из базы данных
func (r *apiKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.ApiKey{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrApiKeyNotFound
	}
	return nil
}
