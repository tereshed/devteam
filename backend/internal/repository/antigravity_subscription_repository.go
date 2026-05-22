package repository

import (
	"context"
	"errors"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AntigravitySubscriptionRepository — CRUD по antigravity_subscriptions.
// На одного user_id допускается ровно одна запись.
type AntigravitySubscriptionRepository interface {
	Upsert(ctx context.Context, sub *models.AntigravitySubscription) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AntigravitySubscription, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	ListExpiring(ctx context.Context, now time.Time, within time.Duration) ([]models.AntigravitySubscription, error)
}

type antigravitySubscriptionRepository struct {
	db *gorm.DB
}

func NewAntigravitySubscriptionRepository(db *gorm.DB) AntigravitySubscriptionRepository {
	return &antigravitySubscriptionRepository{db: db}
}

// Upsert атомарно создаёт или обновляет запись по user_id.
func (r *antigravitySubscriptionRepository) Upsert(ctx context.Context, sub *models.AntigravitySubscription) error {
	if sub.UserID == uuid.Nil {
		return ErrInvalidInput
	}
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"oauth_access_token_enc",
			"oauth_refresh_token_enc",
			"token_type",
			"scopes",
			"expires_at",
			"last_refreshed_at",
			"updated_at",
		}),
	}).Create(sub).Error
}

func (r *antigravitySubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AntigravitySubscription, error) {
	var sub models.AntigravitySubscription
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAntigravitySubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *antigravitySubscriptionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.AntigravitySubscription{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrAntigravitySubscriptionNotFound
	}
	return nil
}

func (r *antigravitySubscriptionRepository) ListExpiring(ctx context.Context, now time.Time, within time.Duration) ([]models.AntigravitySubscription, error) {
	threshold := now.Add(within)
	var items []models.AntigravitySubscription
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", threshold).
		Order("expires_at ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
