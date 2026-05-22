package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AntigravitySubscription — OAuth-сессия Antigravity.
// Все токены шифруются AES-256-GCM (см. backend/pkg/crypto). Plaintext-токены никогда не попадают в БД и логи.
type AntigravitySubscription struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID               uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_antigravity_subscriptions_user" json:"user_id"`
	OAuthAccessTokenEnc  []byte     `gorm:"column:oauth_access_token_enc;type:bytea;not null" json:"-"`
	OAuthRefreshTokenEnc []byte     `gorm:"column:oauth_refresh_token_enc;type:bytea" json:"-"`
	TokenType            string     `gorm:"type:varchar(32);not null;default:'Bearer'" json:"token_type"`
	Scopes               string     `gorm:"type:text;not null;default:''" json:"scopes"`
	ExpiresAt            *time.Time `gorm:"type:timestamp with time zone" json:"expires_at"`
	LastRefreshedAt      *time.Time `gorm:"type:timestamp with time zone" json:"last_refreshed_at"`
	CreatedAt            time.Time  `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (AntigravitySubscription) TableName() string {
	return "antigravity_subscriptions"
}

func (s *AntigravitySubscription) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// IsExpired сообщает, истёк ли access-токен (с запасом skew).
func (s *AntigravitySubscription) IsExpired(now time.Time, skew time.Duration) bool {
	if s.ExpiresAt == nil {
		return false
	}
	return !s.ExpiresAt.After(now.Add(skew))
}
