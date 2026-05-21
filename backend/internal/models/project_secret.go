package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectSecret struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID      uuid.UUID `gorm:"type:uuid;not null" json:"project_id"`
	KeyName        string    `gorm:"type:varchar(128);not null" json:"key_name"`
	EncryptedValue []byte    `gorm:"type:bytea;not null" json:"-"`
	CreatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt      time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (ProjectSecret) TableName() string {
	return "project_secrets"
}

func (s *ProjectSecret) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
