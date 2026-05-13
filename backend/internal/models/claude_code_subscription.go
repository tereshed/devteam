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
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_claude_code_subscriptions_user" json:"user_id"`
	// Sprint 15.e2e ревью #6: явный `column:` обязателен.
	// GORM naming strategy конвертирует `OAuthAccessTokenEnc` в `o_auth_access_token_enc`
	// (capital `O` + capital `A` → дополнительный split), а миграция 024
	// называет колонку `oauth_access_token_enc`. До этого фикса INSERT через
	// репозиторий падал с SQLSTATE 42703 "column o_auth_access_token_enc does not
	// exist" — поломка отсиделась незамеченной, потому что device-flow не было
	// фактических вызывателей до e2e_smoke.sh. НЕ удалять column-теги.
	OAuthAccessTokenEnc  []byte     `gorm:"column:oauth_access_token_enc;type:bytea;not null" json:"-"`
	OAuthRefreshTokenEnc []byte     `gorm:"column:oauth_refresh_token_enc;type:bytea" json:"-"`
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
