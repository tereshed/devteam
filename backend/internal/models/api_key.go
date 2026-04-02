package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApiKey представляет долгосрочный API-ключ пользователя
type ApiKey struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_api_keys_user_id" json:"user_id"`
	Name      string     `gorm:"type:varchar(255);not null" json:"name"`
	KeyHash   string     `gorm:"type:varchar(255);uniqueIndex:idx_api_keys_key_hash;not null" json:"-"`
	KeyPrefix string     `gorm:"type:varchar(12);not null;index:idx_api_keys_key_prefix" json:"key_prefix"`
	Scopes    string     `gorm:"type:text;not null;default:'*'" json:"scopes"`
	ExpiresAt *time.Time `gorm:"type:timestamp with time zone;null" json:"expires_at"`
	LastUsedAt *time.Time `gorm:"type:timestamp with time zone;null" json:"last_used_at"`
	CreatedAt time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	RevokedAt *time.Time `gorm:"type:timestamp with time zone;null" json:"revoked_at"`

	// Связь с пользователем
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName возвращает имя таблицы для модели ApiKey
func (ApiKey) TableName() string {
	return "api_keys"
}

// BeforeCreate вызывается перед созданием записи
func (a *ApiKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// IsExpired проверяет, истек ли ключ
func (a *ApiKey) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false // Без срока действия
	}
	return time.Now().After(*a.ExpiresAt)
}

// IsRevoked проверяет, отозван ли ключ
func (a *ApiKey) IsRevoked() bool {
	return a.RevokedAt != nil
}

// IsValid проверяет, валиден ли ключ (не истек и не отозван)
func (a *ApiKey) IsValid() bool {
	return !a.IsExpired() && !a.IsRevoked()
}
