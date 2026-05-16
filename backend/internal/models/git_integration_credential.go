package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GitIntegrationProvider — провайдер git-интеграции (OAuth).
type GitIntegrationProvider string

const (
	// GitIntegrationProviderGitHub — github.com OAuth App (shared backend credentials).
	GitIntegrationProviderGitHub GitIntegrationProvider = "github"
	// GitIntegrationProviderGitLab — gitlab.com OR self-hosted GitLab (BYO),
	// различается по непустому полю Host.
	GitIntegrationProviderGitLab GitIntegrationProvider = "gitlab"
)

// IsValid проверяет, что значение — допустимый провайдер git-интеграции.
func (p GitIntegrationProvider) IsValid() bool {
	switch p {
	case GitIntegrationProviderGitHub, GitIntegrationProviderGitLab:
		return true
	default:
		return false
	}
}

// GitIntegrationCredential — OAuth-сессия для git-провайдера.
//
// Шифрование:
//   - AccessTokenEnc / RefreshTokenEnc / ByoClientSecretEnc — AES-256-GCM blob,
//     AAD = id записи (см. docs/rules/main.md §2.3 п.5). Защита от cross-row
//     substitution: подмена blob'а из чужой строки → расшифровка падает.
//   - ByoClientID — plain VARCHAR (public значение OAuth 2.0).
//
// Host пустой = shared backend creds (github.com или gitlab.com).
// Host непустой = self-hosted GitLab (BYO), валидируется через
// GitProviderHostValidator перед каждым outbound HTTP (анти-DNS-rebinding).
type GitIntegrationCredential struct {
	ID                  uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID              `gorm:"type:uuid;not null;uniqueIndex:uq_git_integration_credentials_user_provider,priority:1" json:"user_id"`
	Provider            GitIntegrationProvider `gorm:"type:varchar(32);not null;uniqueIndex:uq_git_integration_credentials_user_provider,priority:2" json:"provider"`
	Host                string                 `gorm:"type:varchar(255);not null;default:''" json:"host"`
	ByoClientID         string                 `gorm:"column:byo_client_id;type:varchar(255);not null;default:''" json:"byo_client_id"`
	ByoClientSecretEnc  []byte                 `gorm:"column:byo_client_secret_enc;type:bytea" json:"-"`
	AccessTokenEnc      []byte                 `gorm:"column:access_token_enc;type:bytea;not null" json:"-"`
	RefreshTokenEnc     []byte                 `gorm:"column:refresh_token_enc;type:bytea" json:"-"`
	TokenType           string                 `gorm:"type:varchar(32);not null;default:'Bearer'" json:"token_type"`
	Scopes              string                 `gorm:"type:text;not null;default:''" json:"scopes"`
	AccountLogin        string                 `gorm:"type:varchar(255);not null;default:''" json:"account_login"`
	ExpiresAt           *time.Time             `gorm:"type:timestamp with time zone" json:"expires_at"`
	LastRefreshedAt     *time.Time             `gorm:"type:timestamp with time zone" json:"last_refreshed_at"`
	CreatedAt           time.Time              `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time              `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

// TableName — имя таблицы.
func (GitIntegrationCredential) TableName() string {
	return "git_integration_credentials"
}

// BeforeCreate — генерируем UUID до INSERT, чтобы ID был известен ещё до записи.
// Это критично: AAD при шифровании AccessToken/RefreshToken/ByoClientSecret = id записи.
func (c *GitIntegrationCredential) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// IsExpired сообщает, истёк ли access-токен (с запасом skew).
func (c *GitIntegrationCredential) IsExpired(now time.Time, skew time.Duration) bool {
	if c.ExpiresAt == nil {
		return false
	}
	return !c.ExpiresAt.After(now.Add(skew))
}

// IsBYO — true, если Host непустой (self-hosted GitLab BYO).
func (c *GitIntegrationCredential) IsBYO() bool {
	return c.Host != ""
}
