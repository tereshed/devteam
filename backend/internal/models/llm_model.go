package models

import (
	"time"

	"gorm.io/datatypes"
)

// LLMModel представляет модель из OpenRouter (или другую)
type LLMModel struct {
	ID          string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name        string         `gorm:"not null;type:varchar(255)" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	ContextLength int          `gorm:"default:0" json:"context_length"`
	Architecture  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"architecture" swaggertype:"object"`

	// Pricing
	PricingPrompt     float64 `gorm:"type:numeric(20,10);default:0" json:"pricing_prompt"`
	PricingCompletion float64 `gorm:"type:numeric(20,10);default:0" json:"pricing_completion"`
	PricingRequest    float64 `gorm:"type:numeric(20,10);default:0" json:"pricing_request"`
	PricingImage      float64 `gorm:"type:numeric(20,10);default:0" json:"pricing_image"`

	IsActive bool `gorm:"default:true" json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

