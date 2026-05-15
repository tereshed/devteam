package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Prompt представляет шаблон промпта для LLM
type Prompt struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string         `gorm:"type:varchar(255);uniqueIndex;not null"`
	Description string         `gorm:"type:text"`
	Template    string         `gorm:"type:text;not null"`
	JSONSchema  datatypes.JSON `gorm:"type:jsonb" swaggertype:"object"` // Схема для валидации ответа (опционально)
	IsActive    bool           `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
