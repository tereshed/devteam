package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TeamTypeModel represents a dynamic team type
type TeamTypeModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Code      string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"code"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	IsSystem  bool      `gorm:"type:boolean;not null;default:false" json:"is_system"`
	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName returns table name
func (TeamTypeModel) TableName() string {
	return "team_types"
}

// BeforeCreate sets UUID
func (tt *TeamTypeModel) BeforeCreate(tx *gorm.DB) error {
	if tt.ID == uuid.Nil {
		tt.ID = uuid.New()
	}
	return nil
}
