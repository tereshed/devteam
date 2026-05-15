package apierror

import (
	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a structured API error
type ErrorResponse struct {
	Error   string `json:"error"`   // Stable error code (e.g., "invalid_credentials")
	Message string `json:"message"` // Human-readable message
}

// Standard Error Codes
const (
	ErrInvalidCredentials  = "invalid_credentials"
	ErrUserNotFound        = "user_not_found"
	ErrUserAlreadyExists   = "user_already_exists"
	ErrInvalidToken        = "invalid_token"
	ErrTokenExpired        = "token_expired"
	ErrTokenRequired       = "token_required"
	ErrInvalidAuthHeader   = "invalid_auth_header"
	ErrAccessDenied        = "access_denied"
	ErrUnauthorized        = "unauthorized"
	ErrInternalServerError = "internal_server_error"
	ErrBadRequest          = "bad_request"
	ErrForbidden           = "forbidden"
	ErrNotFound            = "not_found"
	ErrAlreadyExists       = "already_exists"
	ErrConflict            = "conflict"
	ErrUnprocessable       = "unprocessable"
	ErrTooManyRequests     = "too_many_requests"
	// ErrUnsupportedMediaType — неверный Content-Type (например PATCH не application/json).
	ErrUnsupportedMediaType = "unsupported_media_type"
	// ErrRequestEntityTooLarge — тело запроса превысило лимит (например 8 KiB для LLM credentials PATCH).
	ErrRequestEntityTooLarge = "request_entity_too_large"
	// ErrExternalService — сбой внешнего сервиса (GitHub API, git remote и т.д.), HTTP 502.
	ErrExternalService = "external_service_error"
	// ErrTaskAlreadyTerminal — race condition при Cancel: задача уже завершена.
	// HTTP 409, фронт обрабатывает тихо (invalidate + info-toast вместо красного snack).
	ErrTaskAlreadyTerminal = "task_already_terminal"
)

// JSON sends a structured error response
func JSON(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}

// AbortJSON sends a structured error response and aborts the request
func AbortJSON(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}
