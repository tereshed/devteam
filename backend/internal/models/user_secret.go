package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserSecret struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	KeyName        string    `gorm:"type:varchar(128);not null" json:"key_name"`
	EncryptedValue []byte    `gorm:"type:bytea;not null" json:"-"`
	CreatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (UserSecret) TableName() string {
	return "user_secrets"
}

func (s *UserSecret) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
