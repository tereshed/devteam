package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// LLMProviderKind — тип LLM-провайдера в каталоге llm_providers (Sprint 15.1).
// Значение должно совпадать с CHECK-ом chk_llm_providers_kind в миграции 023.
type LLMProviderKind string

const (
	LLMProviderKindAnthropic       LLMProviderKind = "anthropic"
	LLMProviderKindAnthropicOAuth  LLMProviderKind = "anthropic_oauth"
	LLMProviderKindOpenAI          LLMProviderKind = "openai"
	LLMProviderKindGemini          LLMProviderKind = "gemini"
	LLMProviderKindDeepSeek        LLMProviderKind = "deepseek"
	LLMProviderKindQwen            LLMProviderKind = "qwen"
	LLMProviderKindOpenRouter      LLMProviderKind = "openrouter"
	LLMProviderKindMoonshot        LLMProviderKind = "moonshot"
	LLMProviderKindOllama          LLMProviderKind = "ollama"
	LLMProviderKindZhipu           LLMProviderKind = "zhipu"
	LLMProviderKindFreeClaudeProxy LLMProviderKind = "free_claude_proxy"
)

// IsValid проверяет, что kind является поддерживаемым.
func (k LLMProviderKind) IsValid() bool {
	switch k {
	case LLMProviderKindAnthropic, LLMProviderKindAnthropicOAuth,
		LLMProviderKindOpenAI, LLMProviderKindGemini, LLMProviderKindDeepSeek,
		LLMProviderKindQwen, LLMProviderKindOpenRouter, LLMProviderKindMoonshot,
		LLMProviderKindOllama, LLMProviderKindZhipu, LLMProviderKindFreeClaudeProxy:
		return true
	default:
		return false
	}
}

// LLMProviderAuthType — способ аутентификации к LLM-провайдеру.
type LLMProviderAuthType string

const (
	LLMProviderAuthAPIKey LLMProviderAuthType = "api_key"
	LLMProviderAuthOAuth  LLMProviderAuthType = "oauth"
	LLMProviderAuthBearer LLMProviderAuthType = "bearer"
	LLMProviderAuthNone   LLMProviderAuthType = "none"
)

// IsValid проверяет, что auth_type — поддерживаемое значение.
func (a LLMProviderAuthType) IsValid() bool {
	switch a {
	case LLMProviderAuthAPIKey, LLMProviderAuthOAuth, LLMProviderAuthBearer, LLMProviderAuthNone:
		return true
	default:
		return false
	}
}

// LLMProvider — запись каталога llm_providers.
// CredentialsEncrypted — AES-256-GCM blob (см. backend/pkg/crypto). Никогда не сериализуется в JSON.
type LLMProvider struct {
	ID                   uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name                 string              `gorm:"type:varchar(255);not null;uniqueIndex:uq_llm_providers_name" json:"name"`
	Kind                 LLMProviderKind     `gorm:"type:varchar(32);not null" json:"kind"`
	BaseURL              string              `gorm:"type:varchar(1024);not null;default:''" json:"base_url"`
	AuthType             LLMProviderAuthType `gorm:"type:varchar(32);not null;default:'api_key'" json:"auth_type"`
	CredentialsEncrypted []byte              `gorm:"type:bytea" json:"-"`
	DefaultModel         string              `gorm:"type:varchar(255);not null;default:''" json:"default_model"`
	Settings             datatypes.JSON      `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	Enabled              bool                `gorm:"not null;default:true" json:"enabled"`
	CreatedAt            time.Time           `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time           `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (LLMProvider) TableName() string {
	return "llm_providers"
}

func (p *LLMProvider) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
