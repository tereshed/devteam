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
