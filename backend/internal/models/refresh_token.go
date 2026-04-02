package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshToken представляет refresh токен в системе
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_refresh_tokens_user_id" json:"user_id"`
	TokenHash string     `gorm:"type:varchar(255);uniqueIndex:idx_refresh_tokens_token_hash;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"type:timestamp with time zone;not null;index:idx_refresh_tokens_expires_at" json:"expires_at"`
	CreatedAt time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	RevokedAt *time.Time `gorm:"type:timestamp with time zone;null" json:"revoked_at"`

	// Связь с пользователем
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName возвращает имя таблицы для модели RefreshToken
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// BeforeCreate вызывается перед созданием записи
func (rt *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	return nil
}

// IsExpired проверяет, истек ли токен
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// IsRevoked проверяет, отозван ли токен
func (rt *RefreshToken) IsRevoked() bool {
	return rt.RevokedAt != nil
}

// IsValid проверяет, валиден ли токен (не истек и не отозван)
func (rt *RefreshToken) IsValid() bool {
	return !rt.IsExpired() && !rt.IsRevoked()
}
