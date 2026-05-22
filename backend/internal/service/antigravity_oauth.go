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

// AntigravityOAuthProvider — абстракция OAuth2 device-flow для Antigravity.
type AntigravityOAuthProvider interface {
	// InitDeviceCode инициирует device-code flow, возвращая user-facing URL и интервал поллинга.
	InitDeviceCode(ctx context.Context) (*AntigravityDeviceInit, error)
	// PollDeviceToken единичный poll: возвращает токены, если пользователь подтвердил доступ;
	// возвращает ErrAuthorizationPending, пока пользователь ещё не подтвердил.
	PollDeviceToken(ctx context.Context, deviceCode string) (*AntigravityOAuthToken, error)
	// RefreshToken обменивает refresh_token на новую пару токенов.
	RefreshToken(ctx context.Context, refreshToken string) (*AntigravityOAuthToken, error)
	// Revoke отзывает токен у провайдера.
	Revoke(ctx context.Context, token string) error
}

// AntigravityDeviceInit — данные device-flow для отображения пользователю.
type AntigravityDeviceInit struct {
	DeviceCode              string        `json:"device_code"`
	UserCode                string        `json:"user_code"`
	VerificationURI         string        `json:"verification_uri"`
	VerificationURIComplete string        `json:"verification_uri_complete,omitempty"`
	ExpiresIn               time.Duration `json:"expires_in"`
	Interval                time.Duration `json:"interval"`
}

// AntigravityOAuthToken — пара токенов после успешного обмена.
type AntigravityOAuthToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scopes       string
	ExpiresAt    *time.Time
}

// AntigravityOAuthConfig — настройки OAuth-провайдера.
type AntigravityOAuthConfig struct {
	ClientID        string
	DeviceCodeURL   string
	TokenURL        string
	RevokeURL       string
	Scopes          string
	DefaultInterval time.Duration
	DefaultExpires  time.Duration
	HTTPClient      *http.Client
}

// NewAntigravityOAuthProvider — реализация по умолчанию (Antigravity device-flow).
func NewAntigravityOAuthProvider(cfg AntigravityOAuthConfig) AntigravityOAuthProvider {
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
		return &unconfiguredAntigravityOAuth{}
	}
	return &antigravityDeviceFlow{cfg: cfg}
}

// ErrAntigravityOAuthNotConfigured — backend запущен без ANTIGRAVITY_OAUTH_* env.
var ErrAntigravityOAuthNotConfigured = errors.New("antigravity oauth provider is not configured")

type unconfiguredAntigravityOAuth struct{}

func (unconfiguredAntigravityOAuth) InitDeviceCode(context.Context) (*AntigravityDeviceInit, error) {
	return nil, ErrAntigravityOAuthNotConfigured
}
func (unconfiguredAntigravityOAuth) PollDeviceToken(context.Context, string) (*AntigravityOAuthToken, error) {
	return nil, ErrAntigravityOAuthNotConfigured
}
func (unconfiguredAntigravityOAuth) RefreshToken(context.Context, string) (*AntigravityOAuthToken, error) {
	return nil, ErrAntigravityOAuthNotConfigured
}
func (unconfiguredAntigravityOAuth) Revoke(context.Context, string) error {
	return ErrAntigravityOAuthNotConfigured
}

type antigravityDeviceFlow struct{ cfg AntigravityOAuthConfig }

func (p *antigravityDeviceFlow) InitDeviceCode(ctx context.Context) (*AntigravityDeviceInit, error) {
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
		return nil, fmt.Errorf("antigravity oauth: decode device response: %w", err)
	}
	if dr.DeviceCode == "" || dr.UserCode == "" || dr.VerificationURI == "" {
		return nil, fmt.Errorf("antigravity oauth: incomplete device response")
	}
	interval := time.Duration(dr.Interval) * time.Second
	if interval <= 0 {
		interval = p.cfg.DefaultInterval
	}
	expiresIn := time.Duration(dr.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = p.cfg.DefaultExpires
	}
	return &AntigravityDeviceInit{
		DeviceCode:              dr.DeviceCode,
		UserCode:                dr.UserCode,
		VerificationURI:         dr.VerificationURI,
		VerificationURIComplete: dr.VerificationURIComplete,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

func (p *antigravityDeviceFlow) PollDeviceToken(ctx context.Context, deviceCode string) (*AntigravityOAuthToken, error) {
	form := url.Values{
		"client_id":   {p.cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	body, err := p.postForm(ctx, p.cfg.TokenURL, form)
	if err != nil {
		return nil, mapOAuthError(err)
	}
	return parseAntigravityToken(body)
}

func (p *antigravityDeviceFlow) RefreshToken(ctx context.Context, refreshToken string) (*AntigravityOAuthToken, error) {
	form := url.Values{
		"client_id":     {p.cfg.ClientID},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	body, err := p.postForm(ctx, p.cfg.TokenURL, form)
	if err != nil {
		return nil, mapOAuthError(err)
	}
	return parseAntigravityToken(body)
}

func (p *antigravityDeviceFlow) Revoke(ctx context.Context, token string) error {
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

func (p *antigravityDeviceFlow) postForm(ctx context.Context, target string, form url.Values) ([]byte, error) {
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
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("antigravity oauth: read response: %w", readErr)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}
	var tr tokenResponse
	if json.Unmarshal(body, &tr) == nil && tr.Error != "" {
		return nil, oauthError{code: tr.Error, body: string(body)}
	}
	return nil, fmt.Errorf("antigravity oauth: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func parseAntigravityToken(body []byte) (*AntigravityOAuthToken, error) {
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("antigravity oauth: decode token: %w", err)
	}
	if tr.AccessToken == "" {
		if tr.Error != "" {
			return nil, mapOAuthError(oauthError{code: tr.Error, body: string(body)})
		}
		return nil, fmt.Errorf("antigravity oauth: empty access_token")
	}
	tok := &AntigravityOAuthToken{
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
