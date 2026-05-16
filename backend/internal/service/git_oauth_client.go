package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Standard OAuth errors for git providers.
var (
	// ErrGitOAuthUserCancelled — пользователь нажал Cancel на consent screen (error=access_denied).
	ErrGitOAuthUserCancelled = errors.New("git_oauth_user_cancelled")
	// ErrGitOAuthInvalidGrant — code/refresh не принят провайдером.
	ErrGitOAuthInvalidGrant = errors.New("git_oauth_invalid_grant")
	// ErrGitOAuthProviderUnreachable — сеть/5xx со стороны провайдера.
	ErrGitOAuthProviderUnreachable = errors.New("git_oauth_provider_unreachable")
	// ErrGitOAuthNotConfigured — backend запущен без CLIENT_ID/SECRET для нужного провайдера.
	ErrGitOAuthNotConfigured = errors.New("git_oauth_not_configured")
)

// GitOAuthToken — пара токенов после code exchange.
type GitOAuthToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scopes       string
	ExpiresAt    *time.Time
}

// GitOAuthClient — провайдер-агностичный клиент authorization-code flow.
// Реализуется отдельно для GitHub / GitLab.com / BYO GitLab.
type GitOAuthClient interface {
	// AuthCodeURL возвращает URL, куда нужно отправить пользователя.
	AuthCodeURL(state, redirectURI string) string
	// ExchangeCode обменивает authorization-code на токены.
	ExchangeCode(ctx context.Context, code, redirectURI string) (*GitOAuthToken, error)
	// Revoke best-effort вызов revoke endpoint провайдера. Возвращает nil при HTTP 2xx/4xx
	// (4xx = «уже отозвано»), оборачивает ErrGitOAuthProviderUnreachable при сетевой ошибке.
	Revoke(ctx context.Context, accessToken string) error
}

// ─── GitHub ──────────────────────────────────────────────────────────────────

// GitHubOAuthConfig — env GITHUB_OAUTH_CLIENT_ID / GITHUB_OAUTH_CLIENT_SECRET.
type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
	// AuthorizeURL / TokenURL / API base override (для тестов).
	AuthorizeURL string
	TokenURL     string
	APIBaseURL   string
	HTTPClient   *http.Client
}

const (
	defaultGitHubAuthorize = "https://github.com/login/oauth/authorize"
	defaultGitHubToken     = "https://github.com/login/oauth/access_token"
	defaultGitHubAPIBase   = "https://api.github.com"
)

type githubOAuthClient struct {
	cfg GitHubOAuthConfig
}

// NewGitHubOAuthClient — фабрика. Если ClientID/Secret пусты — возвращает заглушку.
func NewGitHubOAuthClient(cfg GitHubOAuthConfig) GitOAuthClient {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return unconfiguredGitOAuth{}
	}
	if cfg.AuthorizeURL == "" {
		cfg.AuthorizeURL = defaultGitHubAuthorize
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultGitHubToken
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultGitHubAPIBase
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &githubOAuthClient{cfg: cfg}
}

func (c *githubOAuthClient) AuthCodeURL(state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if c.cfg.Scopes != "" {
		q.Set("scope", c.cfg.Scopes)
	}
	q.Set("allow_signup", "false")
	return c.cfg.AuthorizeURL + "?" + q.Encode()
}

func (c *githubOAuthClient) ExchangeCode(ctx context.Context, code, redirectURI string) (*GitOAuthToken, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("github exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: github token endpoint: %v", ErrGitOAuthProviderUnreachable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: github HTTP %d", ErrGitOAuthProviderUnreachable, resp.StatusCode)
	}
	var parsed struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		RefreshExpiresIn int    `json:"refresh_token_expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: github malformed token response", ErrGitOAuthProviderUnreachable)
	}
	if parsed.Error != "" {
		switch parsed.Error {
		case "access_denied":
			return nil, ErrGitOAuthUserCancelled
		case "bad_verification_code", "incorrect_client_credentials", "invalid_grant":
			return nil, ErrGitOAuthInvalidGrant
		default:
			return nil, fmt.Errorf("%w: github error %q", ErrGitOAuthProviderUnreachable, parsed.Error)
		}
	}
	if parsed.AccessToken == "" {
		return nil, fmt.Errorf("%w: github empty access_token", ErrGitOAuthInvalidGrant)
	}
	tok := &GitOAuthToken{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		TokenType:    firstNonEmpty(parsed.TokenType, "Bearer"),
		Scopes:       parsed.Scope,
	}
	if parsed.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second).UTC()
		tok.ExpiresAt = &exp
	}
	return tok, nil
}

func (c *githubOAuthClient) Revoke(ctx context.Context, accessToken string) error {
	// DELETE /applications/{client_id}/grant — Basic auth = client_id:client_secret,
	// body = {"access_token": "..."}.
	endpoint := c.cfg.APIBaseURL + "/applications/" + url.PathEscape(c.cfg.ClientID) + "/grant"
	bodyBytes, _ := json.Marshal(map[string]string{"access_token": accessToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("github revoke request: %w", err)
	}
	basic := base64.StdEncoding.EncodeToString([]byte(c.cfg.ClientID + ":" + c.cfg.ClientSecret))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: github revoke: %v", ErrGitOAuthProviderUnreachable, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
	// 204 — успех; 404 — уже отозвано; 422 — bad token. Любой 5xx — считаем сетевой.
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: github revoke HTTP %d", ErrGitOAuthProviderUnreachable, resp.StatusCode)
	}
	return nil
}

// ─── GitLab (shared + BYO) ────────────────────────────────────────────────────

// GitLabOAuthConfig — env GITLAB_OAUTH_CLIENT_ID / GITLAB_OAUTH_CLIENT_SECRET (для gitlab.com).
//
// Для BYO host: ClientID/Secret приходят отдельно при вызове New (см. ниже),
// HTTPClient — SafeGitHTTPClient под фиксированный набор allowedIPs.
type GitLabOAuthConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
	// BaseURL — без trailing slash; для gitlab.com = https://gitlab.com.
	// Для BYO — canonical host из ValidateGitProviderHost.
	BaseURL    string
	HTTPClient *http.Client
}

const defaultGitLabBase = "https://gitlab.com"

type gitlabOAuthClient struct {
	cfg GitLabOAuthConfig
}

// NewGitLabOAuthClient — фабрика. Если ClientID/Secret пусты — возвращает заглушку.
func NewGitLabOAuthClient(cfg GitLabOAuthConfig) GitOAuthClient {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return unconfiguredGitOAuth{}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultGitLabBase
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &gitlabOAuthClient{cfg: cfg}
}

func (c *gitlabOAuthClient) AuthCodeURL(state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	if c.cfg.Scopes != "" {
		q.Set("scope", c.cfg.Scopes)
	}
	return c.cfg.BaseURL + "/oauth/authorize?" + q.Encode()
}

func (c *gitlabOAuthClient) ExchangeCode(ctx context.Context, code, redirectURI string) (*GitOAuthToken, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gitlab exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: gitlab token endpoint: %v", ErrGitOAuthProviderUnreachable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: gitlab HTTP %d", ErrGitOAuthProviderUnreachable, resp.StatusCode)
	}
	var parsed struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: gitlab malformed token response", ErrGitOAuthProviderUnreachable)
	}
	if parsed.Error != "" {
		switch parsed.Error {
		case "access_denied":
			return nil, ErrGitOAuthUserCancelled
		case "invalid_grant", "invalid_client":
			return nil, ErrGitOAuthInvalidGrant
		default:
			return nil, fmt.Errorf("%w: gitlab error %q", ErrGitOAuthProviderUnreachable, parsed.Error)
		}
	}
	if parsed.AccessToken == "" {
		return nil, fmt.Errorf("%w: gitlab empty access_token", ErrGitOAuthInvalidGrant)
	}
	tok := &GitOAuthToken{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		TokenType:    firstNonEmpty(parsed.TokenType, "Bearer"),
		Scopes:       parsed.Scope,
	}
	if parsed.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second).UTC()
		tok.ExpiresAt = &exp
	}
	return tok, nil
}

func (c *gitlabOAuthClient) Revoke(ctx context.Context, accessToken string) error {
	// POST /oauth/revoke (form): token=<access_token>&client_id=...&client_secret=...
	form := url.Values{}
	form.Set("token", accessToken)
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/oauth/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("gitlab revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: gitlab revoke: %v", ErrGitOAuthProviderUnreachable, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: gitlab revoke HTTP %d", ErrGitOAuthProviderUnreachable, resp.StatusCode)
	}
	return nil
}

// ─── unconfigured fallback ────────────────────────────────────────────────────

type unconfiguredGitOAuth struct{}

func (unconfiguredGitOAuth) AuthCodeURL(_, _ string) string { return "" }
func (unconfiguredGitOAuth) ExchangeCode(context.Context, string, string) (*GitOAuthToken, error) {
	return nil, ErrGitOAuthNotConfigured
}
func (unconfiguredGitOAuth) Revoke(context.Context, string) error { return ErrGitOAuthNotConfigured }
