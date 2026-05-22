package dto

import "time"

// AntigravityAuthInitResponse — ответ POST /antigravity/auth/init.
type AntigravityAuthInitResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	// IntervalSeconds — минимальный интервал между poll-запросами /callback.
	IntervalSeconds int `json:"interval_seconds"`
	// ExpiresInSeconds — через сколько device_code станет невалидным.
	ExpiresInSeconds int `json:"expires_in_seconds"`
}

// AntigravityAuthCallbackRequest — тело POST /antigravity/auth/callback.
type AntigravityAuthCallbackRequest struct {
	DeviceCode string `json:"device_code" binding:"required"`
}

// AntigravityAuthManualTokenRequest — тело PUT /antigravity/auth/manual-token.
type AntigravityAuthManualTokenRequest struct {
	AccessToken  string     `json:"access_token" binding:"required"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	TokenType    string     `json:"token_type,omitempty"`
	Scopes       string     `json:"scopes,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// AntigravityAuthStatusResponse — статус подписки текущего пользователя.
type AntigravityAuthStatusResponse struct {
	Connected       bool       `json:"connected"`
	TokenType       string     `json:"token_type,omitempty"`
	Scopes          string     `json:"scopes,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at,omitempty"`
}
