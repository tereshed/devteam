package dto

// ErrorResponse представляет стандартный формат ошибки API
type ErrorResponse struct {
	Error   string `json:"error"`             // Код ошибки (snake_case)
	Message string `json:"message"`           // Читаемое сообщение
	Details string `json:"details,omitempty"` // Дополнительные детали (опционально)
}
