package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)

// RefreshTokenRepository определяет интерфейс для работы с refresh токенами
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, tokenID uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// refreshTokenRepository реализация RefreshTokenRepository
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository создает новый репозиторий refresh токенов
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

// Create создает новый refresh токен
func (r *refreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetByTokenHash получает refresh токен по хешу
func (r *refreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// Revoke отзывает refresh токен
func (r *refreshTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("id = ?", tokenID).
		Update("revoked_at", now).Error
}

// RevokeAllForUser отзывает все refresh токены пользователя
func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// DeleteExpired удаляет истекшие токены
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.RefreshToken{}).Error
}
