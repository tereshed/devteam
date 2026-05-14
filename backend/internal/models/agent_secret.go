package models

import (
	"regexp"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentSecret — зашифрованное значение секрета по ключу (key_name) для агента.
//
// Шифрование: pkg/crypto.AESEncryptor. EncryptedValue — один blob формата
// [version 1b = 0x01][nonce 12b][sealed]. Nonce НЕ хранится отдельно.
// AAD при Encrypt/Decrypt — []byte(secret.ID.String()) (паттерн как user_llm_credentials).
//
// Секрет ссылается из agent.code_backend_settings JSONB по key_name:
//
//	{ "env_secret_keys": ["GITHUB_TOKEN", "ANTHROPIC_API_KEY"] }
//
// SandboxAgentExecutor резолвит ссылки через AgentSecretRepository.GetByName,
// дешифрует, помещает в ExecutionInput.EnvSecrets (там значения уже маскируются
// в логах — см. ExecutionInput.String()).
type AgentSecret struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AgentID        uuid.UUID `gorm:"type:uuid;not null" json:"agent_id"`
	Agent          *Agent    `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	KeyName        string    `gorm:"type:varchar(128);not null" json:"key_name"`
	EncryptedValue []byte    `gorm:"type:bytea;not null" json:"-"` // никогда не сериализуем
	CreatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (AgentSecret) TableName() string {
	return "agent_secrets"
}

// BeforeCreate генерирует UUID если не задан (нужно при использовании AAD = id.String()).
func (s *AgentSecret) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// agentSecretKeyNameRe дублирует CHECK chk_agent_secrets_key_name_format из миграции 032
// для ранней Go-валидации. ENV-style: заглавная буква + alphanum/_, 1-128 chars.
var agentSecretKeyNameRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

// ValidateKeyName проверяет имя ключа на соответствие БД-формату.
// Возвращает true если строка пригодна для хранения, иначе false (Repository
// должен вернуть пользователю ошибку валидации, не доводя до DB-CHECK).
func ValidateAgentSecretKeyName(name string) bool {
	return agentSecretKeyNameRe.MatchString(name)
}
