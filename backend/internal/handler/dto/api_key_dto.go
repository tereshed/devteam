package dto

import "time"

// CreateApiKeyRequest запрос на создание API-ключа
type CreateApiKeyRequest struct {
	Name      string  `json:"name" binding:"required,min=1,max=255"`
	Scopes    string  `json:"scopes"`                                     // "*" — полный доступ, или список через запятую
	ExpiresIn *int64  `json:"expires_in"`                                  // Время жизни в секундах (nil = бессрочный)
}

// ApiKeyResponse ответ с данными API-ключа
type ApiKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     string     `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ApiKeyCreatedResponse ответ при создании — содержит сырой ключ (показывается один раз)
type ApiKeyCreatedResponse struct {
	ApiKeyResponse
	RawKey string `json:"raw_key"` // Показывается только при создании!
}

// MCPConfigResponse готовый JSON-конфиг для подключения к MCP-серверу
type MCPConfigResponse struct {
	Config       map[string]interface{} `json:"config"`        // Готовый конфиг для mcpServers
	Instructions string                 `json:"instructions"`  // Инструкция для пользователя
	ServerURL    string                 `json:"server_url"`    // URL MCP-сервера
}
