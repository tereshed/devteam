package dto

import (
	"time"

	"github.com/google/uuid"
)

// LLMProviderResponse — публичный вид LLMProvider (Sprint 15.B5).
// credentials_encrypted никогда не сериализуется (json:"-" в модели).
type LLMProviderResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	BaseURL      string    `json:"base_url"`
	AuthType     string    `json:"auth_type"`
	DefaultModel string    `json:"default_model"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateLLMProviderRequest — POST /llm-providers.
type CreateLLMProviderRequest struct {
	Name         string `json:"name" binding:"required"`
	Kind         string `json:"kind" binding:"required"`
	BaseURL      string `json:"base_url"`
	AuthType     string `json:"auth_type"`
	Credential   string `json:"credential"`
	DefaultModel string `json:"default_model"`
	Enabled      bool   `json:"enabled"`
}

// UpdateLLMProviderRequest — PUT /llm-providers/:id (full update).
// Пустой credential оставляет существующий blob нетронутым (см. сервис).
type UpdateLLMProviderRequest = CreateLLMProviderRequest

// TestLLMProviderConnectionRequest — POST /llm-providers/test-connection.
type TestLLMProviderConnectionRequest = CreateLLMProviderRequest
