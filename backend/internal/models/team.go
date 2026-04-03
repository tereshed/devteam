package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TeamType тип команды
type TeamType string

const (
	TeamTypeDevelopment TeamType = "development"
)

// IsValid проверяет валидность типа команды
func (tt TeamType) IsValid() bool {
	switch tt {
	case TeamTypeDevelopment:
		return true
	default:
		return false
	}
}

// Team связывает проект с набором AI-агентов (1 проект = 1 команда)
type Team struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"project_id"`
	Project   *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Type      TeamType  `gorm:"type:varchar(50);not null;default:'development'" json:"type"`
	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (Team) TableName() string {
	return "teams"
}

// BeforeCreate генерирует UUID если не задан
func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
