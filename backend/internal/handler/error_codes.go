package handler

// Коды ошибок API
const (
	ErrCodeInvalidRequest     = "invalid_request"
	ErrCodeInvalidCredentials = "invalid_credentials"
	ErrCodeUserNotFound       = "user_not_found"
	ErrCodeUserAlreadyExists  = "user_already_exists"
	ErrCodeTokenExpired       = "token_expired"
	ErrCodeInvalidToken       = "invalid_token"
	ErrCodeUnauthorized       = "unauthorized"
	ErrCodeInternalError      = "internal_error"
	ErrCodeInvalidAuthHeader  = "invalid_auth_header"
)
