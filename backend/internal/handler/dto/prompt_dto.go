package dto

import (
	"time"

	"gorm.io/datatypes"
)

// CreatePromptRequest запрос на создание промпта
type CreatePromptRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Template    string         `json:"template" binding:"required"`
	JSONSchema  datatypes.JSON `json:"json_schema" swaggertype:"string"`
	IsActive    *bool          `json:"is_active"` // Используем указатель чтобы отличить false от zero value
}

// UpdatePromptRequest запрос на обновление промпта
type UpdatePromptRequest struct {
	Description string         `json:"description"`
	Template    string         `json:"template"`
	JSONSchema  datatypes.JSON `json:"json_schema" swaggertype:"string"`
	IsActive    *bool          `json:"is_active"`
}

// PromptResponse ответ с данными промпта
type PromptResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Template    string         `json:"template"`
	JSONSchema  datatypes.JSON `json:"json_schema" swaggertype:"string"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

