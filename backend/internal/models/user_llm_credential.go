package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserLLMProvider — идентификатор LLM-провайдера для пользовательских ключей (SSOT: docs/tasks/13.5-backend-user-llm-credentials.md).
type UserLLMProvider string

const (
	UserLLMProviderOpenAI     UserLLMProvider = "openai"
	UserLLMProviderAnthropic  UserLLMProvider = "anthropic"
	UserLLMProviderGemini     UserLLMProvider = "gemini"
	UserLLMProviderDeepSeek   UserLLMProvider = "deepseek"
	UserLLMProviderQwen       UserLLMProvider = "qwen"
	UserLLMProviderOpenRouter UserLLMProvider = "openrouter"
)

// UserLLMProvidersOrdered — фиксированный порядок для ответа GET/PATCH (ровно шесть ключей).
var UserLLMProvidersOrdered = []UserLLMProvider{
	UserLLMProviderOpenAI,
	UserLLMProviderAnthropic,
	UserLLMProviderGemini,
	UserLLMProviderDeepSeek,
	UserLLMProviderQwen,
	UserLLMProviderOpenRouter,
}

// IsValidUserLLMProvider проверяет значение provider.
func IsValidUserLLMProvider(p string) bool {
	switch UserLLMProvider(p) {
	case UserLLMProviderOpenAI, UserLLMProviderAnthropic, UserLLMProviderGemini,
		UserLLMProviderDeepSeek, UserLLMProviderQwen, UserLLMProviderOpenRouter:
		return true
	default:
		return false
	}
}

// UserLlmCredential — зашифрованный API-ключ пользователя для одного провайдера.
type UserLlmCredential struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID       `gorm:"type:uuid;not null;index:idx_user_llm_credentials_user_id" json:"user_id"`
	Provider     UserLLMProvider `gorm:"type:varchar(32);not null" json:"provider"`
	EncryptedKey []byte          `gorm:"type:bytea;not null" json:"-"`
	CreatedAt    time.Time       `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time       `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (UserLlmCredential) TableName() string {
	return "user_llm_credentials"
}

func (c *UserLlmCredential) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// UserLlmCredentialAuditAction — тип записи аудита.
type UserLlmCredentialAuditAction string

const (
	UserLlmCredentialAuditSet   UserLlmCredentialAuditAction = "set"
	UserLlmCredentialAuditClear UserLlmCredentialAuditAction = "clear"
)

// UserLlmCredentialAudit — аудит изменений ключей (без секрета и маски).
type UserLlmCredentialAudit struct {
	ID        uuid.UUID                    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID                    `gorm:"type:uuid;not null" json:"user_id"`
	Provider  UserLLMProvider              `gorm:"type:varchar(32);not null" json:"provider"`
	Action    UserLlmCredentialAuditAction `gorm:"type:varchar(16);not null" json:"action"`
	CreatedAt time.Time                    `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	IP        string                       `gorm:"type:varchar(64);not null" json:"ip"`
	UserAgent string                       `gorm:"type:text;not null" json:"user_agent"`
}

func (UserLlmCredentialAudit) TableName() string {
	return "user_llm_credential_audit"
}

func (a *UserLlmCredentialAudit) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
