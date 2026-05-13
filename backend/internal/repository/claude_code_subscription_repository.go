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

// ClaudeCodeSubscriptionRepository — CRUD по claude_code_subscriptions (Sprint 15.12).
// На одного user_id допускается ровно одна запись (UNIQUE-ограничение в миграции 024).
type ClaudeCodeSubscriptionRepository interface {
	Upsert(ctx context.Context, sub *models.ClaudeCodeSubscription) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.ClaudeCodeSubscription, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	// ListExpiring возвращает подписки, у которых expires_at <= now+within (для refresh-воркера 15.13).
	ListExpiring(ctx context.Context, now time.Time, within time.Duration) ([]models.ClaudeCodeSubscription, error)
}

type claudeCodeSubscriptionRepository struct {
	db *gorm.DB
}

func NewClaudeCodeSubscriptionRepository(db *gorm.DB) ClaudeCodeSubscriptionRepository {
	return &claudeCodeSubscriptionRepository{db: db}
}

// Upsert атомарно создаёт или обновляет запись по user_id.
// Sprint 15.B (B3): использует ON CONFLICT (user_id) DO UPDATE — окно race condition
// между двумя параллельными RefreshOne закрывается транзакционно в БД.
func (r *claudeCodeSubscriptionRepository) Upsert(ctx context.Context, sub *models.ClaudeCodeSubscription) error {
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

func (r *claudeCodeSubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.ClaudeCodeSubscription, error) {
	var sub models.ClaudeCodeSubscription
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrClaudeCodeSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *claudeCodeSubscriptionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.ClaudeCodeSubscription{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrClaudeCodeSubscriptionNotFound
	}
	return nil
}

func (r *claudeCodeSubscriptionRepository) ListExpiring(ctx context.Context, now time.Time, within time.Duration) ([]models.ClaudeCodeSubscription, error) {
	threshold := now.Add(within)
	var items []models.ClaudeCodeSubscription
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", threshold).
		Order("expires_at ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
