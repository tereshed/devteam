package dto

import "time"

// ClaudeCodeAuthInitResponse — ответ POST /claude-code/auth/init (Sprint 15.12).
type ClaudeCodeAuthInitResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	// IntervalSeconds — минимальный интервал между poll-запросами /callback.
	IntervalSeconds int `json:"interval_seconds"`
	// ExpiresInSeconds — через сколько device_code станет невалидным.
	ExpiresInSeconds int `json:"expires_in_seconds"`
}

// ClaudeCodeAuthCallbackRequest — тело POST /claude-code/auth/callback.
type ClaudeCodeAuthCallbackRequest struct {
	DeviceCode string `json:"device_code" binding:"required"`
}

// ClaudeCodeAuthStatusResponse — статус подписки текущего пользователя.
type ClaudeCodeAuthStatusResponse struct {
	Connected       bool       `json:"connected"`
	TokenType       string     `json:"token_type,omitempty"`
	Scopes          string     `json:"scopes,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at,omitempty"`
}
