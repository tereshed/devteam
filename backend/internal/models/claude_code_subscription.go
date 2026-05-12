package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClaudeCodeSubscription — OAuth-сессия Claude Code (Sprint 15.2).
// Все токены шифруются AES-256-GCM (см. backend/pkg/crypto). Plaintext-токены никогда не попадают в БД и логи.
// AAD (associatedData) при шифровании рекомендуется делать равным
// []byte("claude_code_subscription:" + user_id.String()), чтобы blob нельзя было перенести на другого пользователя.
type ClaudeCodeSubscription struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID               uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_claude_code_subscriptions_user" json:"user_id"`
	OAuthAccessTokenEnc  []byte     `gorm:"type:bytea;not null" json:"-"`
	OAuthRefreshTokenEnc []byte     `gorm:"type:bytea" json:"-"`
	TokenType            string     `gorm:"type:varchar(32);not null;default:'Bearer'" json:"token_type"`
	Scopes               string     `gorm:"type:text;not null;default:''" json:"scopes"`
	ExpiresAt            *time.Time `gorm:"type:timestamp with time zone" json:"expires_at"`
	LastRefreshedAt      *time.Time `gorm:"type:timestamp with time zone" json:"last_refreshed_at"`
	CreatedAt            time.Time  `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (ClaudeCodeSubscription) TableName() string {
	return "claude_code_subscriptions"
}

func (s *ClaudeCodeSubscription) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// IsExpired сообщает, истёк ли access-токен (с запасом skew).
// Используется фоновым воркером claude_code_token_refresher (15.13).
func (s *ClaudeCodeSubscription) IsExpired(now time.Time, skew time.Duration) bool {
	if s.ExpiresAt == nil {
		return false
	}
	return !s.ExpiresAt.After(now.Add(skew))
}
