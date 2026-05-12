package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClaudeCodeOAuthProvider — абстракция OAuth2 device-flow для Claude Code (Sprint 15.12).
//
// Реализация по умолчанию (anthropicDeviceFlow) ходит на конфигурируемые URL
// https://console.anthropic.com/v1/oauth/device + /v1/oauth/token. Конкретные пути могут отличаться;
// конкретные значения берутся из ClaudeCodeOAuthConfig (env CLAUDE_CODE_OAUTH_*).
type ClaudeCodeOAuthProvider interface {
	// InitDeviceCode инициирует device-code flow, возвращая user-facing URL и интервал поллинга.
	InitDeviceCode(ctx context.Context) (*ClaudeCodeDeviceInit, error)
	// PollDeviceToken единичный poll: возвращает токены, если пользователь подтвердил доступ;
	// возвращает ErrAuthorizationPending, пока пользователь ещё не подтвердил.
	PollDeviceToken(ctx context.Context, deviceCode string) (*ClaudeCodeOAuthToken, error)
	// RefreshToken обменивает refresh_token на новую пару токенов.
	RefreshToken(ctx context.Context, refreshToken string) (*ClaudeCodeOAuthToken, error)
	// Revoke отзывает токен у провайдера (best-effort; даже если вернёт ошибку, локально считаем revoked).
	Revoke(ctx context.Context, token string) error
}

// ClaudeCodeDeviceInit — данные device-flow для отображения пользователю.
type ClaudeCodeDeviceInit struct {
	DeviceCode      string        `json:"device_code"`
	UserCode        string        `json:"user_code"`
	VerificationURI string        `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn       time.Duration `json:"expires_in"`
	Interval        time.Duration `json:"interval"`
}

// ClaudeCodeOAuthToken — пара токенов после успешного обмена.
type ClaudeCodeOAuthToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scopes       string
	ExpiresAt    *time.Time
}

// Стандартные ошибки device-flow (RFC 8628, §3.5).
var (
	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrExpiredToken         = errors.New("expired_token")
	ErrAccessDenied         = errors.New("access_denied")
	ErrOAuthInvalidGrant    = errors.New("invalid_grant")
)

// ClaudeCodeOAuthConfig — настройки OAuth-провайдера (берутся из env / config.LLM).
type ClaudeCodeOAuthConfig struct {
	ClientID       string
	DeviceCodeURL  string // напр. https://console.anthropic.com/v1/oauth/device
	TokenURL       string // напр. https://console.anthropic.com/v1/oauth/token
	RevokeURL      string // опционально
	Scopes         string
	DefaultInterval time.Duration // дефолт, если провайдер не вернул interval
	DefaultExpires  time.Duration // дефолт expires_in
	HTTPClient     *http.Client
}

// NewClaudeCodeOAuthProvider — реализация по умолчанию (Anthropic device-flow).
// Если ClientID == "" — возвращает заглушку, всегда отвечающую ErrOAuthNotConfigured.
func NewClaudeCodeOAuthProvider(cfg ClaudeCodeOAuthConfig) ClaudeCodeOAuthProvider {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.DefaultInterval <= 0 {
		cfg.DefaultInterval = 5 * time.Second
	}
	if cfg.DefaultExpires <= 0 {
		cfg.DefaultExpires = 15 * time.Minute
	}
	if cfg.ClientID == "" || cfg.DeviceCodeURL == "" || cfg.TokenURL == "" {
		return &unconfiguredOAuth{}
	}
	return &anthropicDeviceFlow{cfg: cfg}
}

// ErrOAuthNotConfigured — backend запущен без CLAUDE_CODE_OAUTH_* env.
var ErrOAuthNotConfigured = errors.New("claude code oauth provider is not configured")

type unconfiguredOAuth struct{}

func (unconfiguredOAuth) InitDeviceCode(context.Context) (*ClaudeCodeDeviceInit, error) {
	return nil, ErrOAuthNotConfigured
}
func (unconfiguredOAuth) PollDeviceToken(context.Context, string) (*ClaudeCodeOAuthToken, error) {
	return nil, ErrOAuthNotConfigured
}
func (unconfiguredOAuth) RefreshToken(context.Context, string) (*ClaudeCodeOAuthToken, error) {
	return nil, ErrOAuthNotConfigured
}
func (unconfiguredOAuth) Revoke(context.Context, string) error { return ErrOAuthNotConfigured }

// === Anthropic device-flow ===

type anthropicDeviceFlow struct{ cfg ClaudeCodeOAuthConfig }

type deviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func (p *anthropicDeviceFlow) InitDeviceCode(ctx context.Context) (*ClaudeCodeDeviceInit, error) {
	form := url.Values{
		"client_id": {p.cfg.ClientID},
		"scope":     {p.cfg.Scopes},
	}
	body, err := p.postForm(ctx, p.cfg.DeviceCodeURL, form)
	if err != nil {
		return nil, err
	}
	var dr deviceResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, fmt.Errorf("claude-code oauth: decode device response: %w", err)
	}
	if dr.DeviceCode == "" || dr.UserCode == "" || dr.VerificationURI == "" {
		return nil, fmt.Errorf("claude-code oauth: incomplete device response")
	}
	interval := time.Duration(dr.Interval) * time.Second
	if interval <= 0 {
		interval = p.cfg.DefaultInterval
	}
	expiresIn := time.Duration(dr.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = p.cfg.DefaultExpires
	}
	return &ClaudeCodeDeviceInit{
		DeviceCode:              dr.DeviceCode,
		UserCode:                dr.UserCode,
		VerificationURI:         dr.VerificationURI,
		VerificationURIComplete: dr.VerificationURIComplete,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

func (p *anthropicDeviceFlow) PollDeviceToken(ctx context.Context, deviceCode string) (*ClaudeCodeOAuthToken, error) {
	form := url.Values{
		"client_id":   {p.cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	body, err := p.postForm(ctx, p.cfg.TokenURL, form)
	if err != nil {
		return nil, mapOAuthError(err)
	}
	return parseToken(body)
}

func (p *anthropicDeviceFlow) RefreshToken(ctx context.Context, refreshToken string) (*ClaudeCodeOAuthToken, error) {
	form := url.Values{
		"client_id":     {p.cfg.ClientID},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	body, err := p.postForm(ctx, p.cfg.TokenURL, form)
	if err != nil {
		return nil, mapOAuthError(err)
	}
	return parseToken(body)
}

func (p *anthropicDeviceFlow) Revoke(ctx context.Context, token string) error {
	if p.cfg.RevokeURL == "" {
		return nil
	}
	form := url.Values{
		"client_id": {p.cfg.ClientID},
		"token":     {token},
	}
	_, err := p.postForm(ctx, p.cfg.RevokeURL, form)
	return err
}

func (p *anthropicDeviceFlow) postForm(ctx context.Context, target string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}
	// На 4xx OAuth-провайдеры возвращают JSON {"error":"..."}; пробрасываем как структурную ошибку.
	var tr tokenResponse
	if json.Unmarshal(body, &tr) == nil && tr.Error != "" {
		return nil, oauthError{code: tr.Error, body: string(body)}
	}
	return nil, fmt.Errorf("claude-code oauth: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

type oauthError struct {
	code string
	body string
}

func (e oauthError) Error() string { return fmt.Sprintf("oauth error: %s", e.code) }

func mapOAuthError(err error) error {
	var oe oauthError
	if !errors.As(err, &oe) {
		return err
	}
	switch oe.code {
	case "authorization_pending":
		return ErrAuthorizationPending
	case "slow_down":
		return ErrSlowDown
	case "expired_token":
		return ErrExpiredToken
	case "access_denied":
		return ErrAccessDenied
	case "invalid_grant":
		return ErrOAuthInvalidGrant
	default:
		return err
	}
}

func parseToken(body []byte) (*ClaudeCodeOAuthToken, error) {
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("claude-code oauth: decode token: %w", err)
	}
	if tr.AccessToken == "" {
		if tr.Error != "" {
			return nil, mapOAuthError(oauthError{code: tr.Error, body: string(body)})
		}
		return nil, fmt.Errorf("claude-code oauth: empty access_token")
	}
	tok := &ClaudeCodeOAuthToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scopes:       tr.Scope,
	}
	if tr.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
		tok.ExpiresAt = &t
	}
	return tok, nil
}
