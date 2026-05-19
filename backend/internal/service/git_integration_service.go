package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// Reason-коды для IntegrationConnectionChanged (Git OAuth) — зеркало §4a.5.
const (
	GitReasonUserCancelled       = "user_cancelled"
	GitReasonInvalidGrant        = "invalid_grant"
	GitReasonProviderUnreachable = "provider_unreachable"
	GitReasonOAuthNotConfigured  = "oauth_not_configured"
	GitReasonInvalidHost         = "invalid_host"
	GitReasonStateExpired        = "state_expired"
	GitReasonInternalError       = "internal_error"
)

// GitIntegrationStatus — публичный статус подключения git-провайдера.
type GitIntegrationStatus struct {
	Provider     models.GitIntegrationProvider `json:"provider"`
	Connected    bool                          `json:"connected"`
	Host         string                        `json:"host,omitempty"`
	AccountLogin string                        `json:"account_login,omitempty"`
	Scopes       string                        `json:"scopes,omitempty"`
	ExpiresAt    *time.Time                    `json:"expires_at,omitempty"`
	ConnectedAt  *time.Time                    `json:"connected_at,omitempty"`
}

// GitIntegrationInitResult — то, что фронт получает от Init.
type GitIntegrationInitResult struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

// GitIntegrationCallbackResult — то, что фронт получает от Callback (или 302).
type GitIntegrationCallbackResult struct {
	Provider models.GitIntegrationProvider `json:"provider"`
	Status   GitIntegrationStatus          `json:"status"`
}

// BYOGitLabInit — параметры self-hosted GitLab init (3.5b).
type BYOGitLabInit struct {
	Host         string // raw, валидируется
	ClientID     string
	ClientSecret string
}

// GitIntegrationService — управление OAuth-привязками GitHub/GitLab/BYO GitLab.
//
// Безопасность:
//   - state — one-shot, привязан к user_id; consume() удаляет запись.
//   - secrets (refresh, access, byo client_secret) шифруются AES-GCM, AAD = id записи.
//   - все outbound HTTP к BYO GitLab — через SafeGitHTTPClient + ValidateGitProviderHost
//     при каждом вызове (anti DNS rebinding).
//   - все логи — через logging.NewHandler (redact-обёртка); body провайдера —
//     через SafeRawAttr (длина + хэш), никогда не как plain.
type GitIntegrationService interface {
	// InitGitHub возвращает authorize URL для редиректа.
	InitGitHub(ctx context.Context, userID uuid.UUID, redirectURI string) (*GitIntegrationInitResult, error)
	// InitGitLabShared — gitlab.com (общие client_id/secret из env).
	InitGitLabShared(ctx context.Context, userID uuid.UUID, redirectURI string) (*GitIntegrationInitResult, error)
	// InitGitLabBYO — self-hosted GitLab: host + client_id/secret приходят от пользователя.
	InitGitLabBYO(ctx context.Context, userID uuid.UUID, redirectURI string, byo BYOGitLabInit) (*GitIntegrationInitResult, error)
	// HandleCallback — общий callback handler. provider определяется из state; userID не передаётся
	// (защита: state привязан к user, callback от другого user не пройдёт).
	HandleCallback(ctx context.Context, code, state, providerError string) (*GitIntegrationCallbackResult, error)
	// Revoke вызывает remote revoke endpoint провайдера, затем удаляет локальную запись (fail-soft).
	// remoteFailed=true означает, что remote-revoke упал (сеть) — локально всё равно удалено.
	Revoke(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (remoteFailed bool, err error)
	// Status — статус одного провайдера; nil если не подключён.
	Status(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (*GitIntegrationStatus, error)
	// ListStatuses — все подключённые провайдеры пользователя.
	ListStatuses(ctx context.Context, userID uuid.UUID) ([]GitIntegrationStatus, error)
}

// GitIntegrationServiceDeps — зависимости конструктора.
type GitIntegrationServiceDeps struct {
	Repo       repository.GitIntegrationCredentialRepository
	Encryptor  Encryptor
	GitHub     GitOAuthClient
	GitLab     GitOAuthClient
	Validator  *GitProviderHostValidator
	StateStore GitOAuthStateStore
	Bus        events.EventBus
	Logger     *slog.Logger
	// SafeHTTPClientFactory — фабрика SafeGitHTTPClient (allowedIPs → *http.Client). Для тестов.
	// Если nil — используется service.SafeGitHTTPClient.
	SafeHTTPClientFactory SafeHTTPClientFactory
	// Now — мокабельный clock.
	Now func() time.Time
	// StateTTL — TTL state-токена (default 10m).
	StateTTL time.Duration
	// RevokeTimeout — таймаут remote revoke (default 10s).
	RevokeTimeout time.Duration
}

// SafeHTTPClientFactory — для BYO GitLab (3.5b).
type SafeHTTPClientFactory func(allowedIPs []net.IP, timeout time.Duration) *http.Client

type gitIntegrationService struct {
	deps GitIntegrationServiceDeps
}

// NewGitIntegrationService — конструктор.
func NewGitIntegrationService(deps GitIntegrationServiceDeps) GitIntegrationService {
	if deps.Logger == nil {
		deps.Logger = logging.NopLogger()
	}
	if deps.StateStore == nil {
		deps.StateStore = NewInMemoryGitOAuthStateStore()
	}
	if deps.Validator == nil {
		deps.Validator = NewGitProviderHostValidator(nil, false)
	}
	if deps.GitHub == nil {
		deps.GitHub = unconfiguredGitOAuth{}
	}
	if deps.GitLab == nil {
		deps.GitLab = unconfiguredGitOAuth{}
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	if deps.StateTTL <= 0 {
		deps.StateTTL = 10 * time.Minute
	}
	if deps.RevokeTimeout <= 0 {
		deps.RevokeTimeout = 10 * time.Second
	}
	return &gitIntegrationService{deps: deps}
}

func (s *gitIntegrationService) InitGitHub(ctx context.Context, userID uuid.UUID, redirectURI string) (*GitIntegrationInitResult, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id required")
	}
	state, err := s.deps.StateStore.New(GitOAuthState{
		UserID:      userID,
		Provider:    models.GitIntegrationProviderGitHub,
		RedirectURI: redirectURI,
		CreatedAt:   s.deps.Now(),
	}, s.deps.StateTTL)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	authURL := s.deps.GitHub.AuthCodeURL(state, redirectURI)
	if authURL == "" {
		return nil, ErrGitOAuthNotConfigured
	}
	return &GitIntegrationInitResult{AuthorizeURL: authURL, State: state}, nil
}

func (s *gitIntegrationService) InitGitLabShared(ctx context.Context, userID uuid.UUID, redirectURI string) (*GitIntegrationInitResult, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id required")
	}
	state, err := s.deps.StateStore.New(GitOAuthState{
		UserID:      userID,
		Provider:    models.GitIntegrationProviderGitLab,
		RedirectURI: redirectURI,
		CreatedAt:   s.deps.Now(),
	}, s.deps.StateTTL)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	authURL := s.deps.GitLab.AuthCodeURL(state, redirectURI)
	if authURL == "" {
		return nil, ErrGitOAuthNotConfigured
	}
	return &GitIntegrationInitResult{AuthorizeURL: authURL, State: state}, nil
}

func (s *gitIntegrationService) InitGitLabBYO(ctx context.Context, userID uuid.UUID, redirectURI string, byo BYOGitLabInit) (*GitIntegrationInitResult, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id required")
	}
	byo.Host = strings.TrimSpace(byo.Host)
	byo.ClientID = strings.TrimSpace(byo.ClientID)
	byo.ClientSecret = strings.TrimSpace(byo.ClientSecret)
	if byo.Host == "" || byo.ClientID == "" || byo.ClientSecret == "" {
		return nil, fmt.Errorf("%w: host/client_id/client_secret required", ErrInvalidGitProviderHost)
	}
	canonical, _, err := s.deps.Validator.ValidateGitProviderHost(ctx, byo.Host)
	if err != nil {
		return nil, err
	}
	// Build a BYO oauth client just to compute authorize URL — нам не нужен HTTP-клиент пока.
	tmp := NewGitLabOAuthClient(GitLabOAuthConfig{
		ClientID:     byo.ClientID,
		ClientSecret: byo.ClientSecret,
		Scopes:       gitlabSharedScopes(s.deps.GitLab),
		BaseURL:      canonical,
	})
	state, err := s.deps.StateStore.New(GitOAuthState{
		UserID:          userID,
		Provider:        models.GitIntegrationProviderGitLab,
		Host:            canonical,
		ByoClientID:     byo.ClientID,
		ByoClientSecret: byo.ClientSecret,
		RedirectURI:     redirectURI,
		CreatedAt:       s.deps.Now(),
	}, s.deps.StateTTL)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	authURL := tmp.AuthCodeURL(state, redirectURI)
	return &GitIntegrationInitResult{AuthorizeURL: authURL, State: state}, nil
}

func (s *gitIntegrationService) HandleCallback(ctx context.Context, code, state, providerError string) (*GitIntegrationCallbackResult, error) {
	if providerError == "access_denied" {
		// Достаём state, чтобы понять, кто и какой провайдер.
		ps, _ := s.deps.StateStore.Consume(state)
		if ps.UserID != uuid.Nil {
			s.publishStatus(ctx, ps.UserID, ps.Provider, events.IntegrationStatusError, GitReasonUserCancelled, nil, nil, false)
		}
		return nil, ErrGitOAuthUserCancelled
	}
	if state == "" || code == "" {
		return nil, fmt.Errorf("%w: missing code/state", ErrGitOAuthInvalidGrant)
	}
	pending, err := s.deps.StateStore.Consume(state)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitOAuthStateNotFound, err)
	}

	// Persist через context.WithoutCancel: между exchange и INSERT не должно быть гонок отмены.
	persistCtx := context.WithoutCancel(ctx)

	client, redirectURI, err := s.providerClientFor(persistCtx, pending)
	if err != nil {
		s.publishStatus(persistCtx, pending.UserID, pending.Provider, events.IntegrationStatusError, mapErrReason(err), nil, nil, false)
		return nil, err
	}

	tok, err := client.ExchangeCode(ctx, code, redirectURI)
	if err != nil {
		s.deps.Logger.Warn("git oauth code exchange failed",
			"provider", string(pending.Provider),
			"reason", mapErrReason(err),
			"error_summary", logging.SafeRawAttr([]byte(err.Error())),
		)
		s.publishStatus(persistCtx, pending.UserID, pending.Provider, events.IntegrationStatusError, mapErrReason(err), nil, nil, false)
		return nil, err
	}

	cred, err := s.persistCred(persistCtx, pending, tok)
	if err != nil {
		s.publishStatus(persistCtx, pending.UserID, pending.Provider, events.IntegrationStatusError, GitReasonInternalError, nil, nil, false)
		return nil, fmt.Errorf("persist: %w", err)
	}

	now := s.deps.Now()
	s.publishStatus(persistCtx, pending.UserID, pending.Provider, events.IntegrationStatusConnected, "", &now, cred.ExpiresAt, false)
	return &GitIntegrationCallbackResult{
		Provider: pending.Provider,
		Status:   credToStatus(cred),
	}, nil
}

func (s *gitIntegrationService) Revoke(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (bool, error) {
	cred, err := s.deps.Repo.GetByUserAndProvider(ctx, userID, provider)
	if errors.Is(err, repository.ErrGitIntegrationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// 1. Сначала remote revoke (best-effort) — порядок важен по спеке 3.4/3.5.
	remoteFailed := false
	if accessToken, decErr := s.decryptToString(cred.AccessTokenEnc, repository.GitIntegrationCredentialAAD(cred.ID)); decErr == nil && accessToken != "" {
		client, _, providerErr := s.providerClientForCred(ctx, cred)
		if providerErr != nil {
			s.deps.Logger.Warn("git oauth revoke: provider client unavailable",
				"provider", string(cred.Provider),
				"reason", mapErrReason(providerErr))
			remoteFailed = true
		} else {
			rCtx, cancel := context.WithTimeout(ctx, s.deps.RevokeTimeout)
			defer cancel()
			if revErr := client.Revoke(rCtx, accessToken); revErr != nil {
				s.deps.Logger.Warn("git oauth remote revoke failed (fail-soft, local still removed)",
					"provider", string(cred.Provider),
					"error_summary", logging.SafeRawAttr([]byte(revErr.Error())))
				remoteFailed = true
			}
		}
	} else if decErr != nil {
		s.deps.Logger.Warn("git oauth revoke: decrypt access token failed",
			"provider", string(cred.Provider),
			"error_summary", logging.SafeRawAttr([]byte(decErr.Error())))
		remoteFailed = true
	}

	// 2. Локальное удаление — всегда, даже если remote-revoke упал.
	if err := s.deps.Repo.DeleteByUserAndProvider(ctx, userID, provider); err != nil {
		return remoteFailed, err
	}

	now := s.deps.Now()
	s.publishStatus(ctx, userID, provider, events.IntegrationStatusDisconnected, "", &now, nil, remoteFailed)
	return remoteFailed, nil
}

func (s *gitIntegrationService) Status(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (*GitIntegrationStatus, error) {
	cred, err := s.deps.Repo.GetByUserAndProvider(ctx, userID, provider)
	if errors.Is(err, repository.ErrGitIntegrationNotFound) {
		return &GitIntegrationStatus{Provider: provider, Connected: false}, nil
	}
	if err != nil {
		return nil, err
	}
	st := credToStatus(cred)
	return &st, nil
}

func (s *gitIntegrationService) ListStatuses(ctx context.Context, userID uuid.UUID) ([]GitIntegrationStatus, error) {
	creds, err := s.deps.Repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]GitIntegrationStatus, 0, len(creds))
	for i := range creds {
		out = append(out, credToStatus(&creds[i]))
	}
	return out, nil
}

// ─── internals ────────────────────────────────────────────────────────────────

func (s *gitIntegrationService) providerClientFor(ctx context.Context, pending GitOAuthState) (GitOAuthClient, string, error) {
	switch pending.Provider {
	case models.GitIntegrationProviderGitHub:
		return s.deps.GitHub, pending.RedirectURI, nil
	case models.GitIntegrationProviderGitLab:
		if pending.Host == "" {
			return s.deps.GitLab, pending.RedirectURI, nil
		}
		// BYO GitLab: пере-валидируем host для свежего allowedIPs (anti DNS rebinding),
		// собираем клиент с SafeGitHTTPClient под этот allow-list.
		canonical, allowedIPs, err := s.deps.Validator.ValidateGitProviderHost(ctx, pending.Host)
		if err != nil {
			return nil, "", err
		}
		client := NewGitLabOAuthClient(GitLabOAuthConfig{
			ClientID:     pending.ByoClientID,
			ClientSecret: pending.ByoClientSecret,
			BaseURL:      canonical,
			HTTPClient:   s.buildSafeHTTPClient(allowedIPs),
		})
		return client, pending.RedirectURI, nil
	default:
		return nil, "", fmt.Errorf("unknown provider %q", pending.Provider)
	}
}

func (s *gitIntegrationService) providerClientForCred(ctx context.Context, cred *models.GitIntegrationCredential) (GitOAuthClient, string, error) {
	switch cred.Provider {
	case models.GitIntegrationProviderGitHub:
		return s.deps.GitHub, "", nil
	case models.GitIntegrationProviderGitLab:
		if cred.Host == "" {
			return s.deps.GitLab, "", nil
		}
		canonical, allowedIPs, err := s.deps.Validator.ValidateGitProviderHost(ctx, cred.Host)
		if err != nil {
			return nil, "", err
		}
		secret, err := s.decryptToString(cred.ByoClientSecretEnc, repository.GitIntegrationCredentialAAD(cred.ID))
		if err != nil {
			return nil, "", fmt.Errorf("decrypt byo client secret: %w", err)
		}
		client := NewGitLabOAuthClient(GitLabOAuthConfig{
			ClientID:     cred.ByoClientID,
			ClientSecret: secret,
			BaseURL:      canonical,
			HTTPClient:   s.buildSafeHTTPClient(allowedIPs),
		})
		return client, "", nil
	default:
		return nil, "", fmt.Errorf("unknown provider %q", cred.Provider)
	}
}

func (s *gitIntegrationService) buildSafeHTTPClient(allowedIPs []net.IP) *http.Client {
	timeout := 30 * time.Second
	if s.deps.SafeHTTPClientFactory != nil {
		return s.deps.SafeHTTPClientFactory(allowedIPs, timeout)
	}
	return SafeGitHTTPClient(allowedIPs, timeout)
}

func (s *gitIntegrationService) persistCred(ctx context.Context, pending GitOAuthState, tok *GitOAuthToken) (*models.GitIntegrationCredential, error) {
	id := uuid.New()
	aad := repository.GitIntegrationCredentialAAD(id)
	accessEnc, err := s.deps.Encryptor.Encrypt([]byte(tok.AccessToken), aad)
	if err != nil {
		return nil, fmt.Errorf("encrypt access: %w", err)
	}
	var refreshEnc []byte
	if tok.RefreshToken != "" {
		refreshEnc, err = s.deps.Encryptor.Encrypt([]byte(tok.RefreshToken), aad)
		if err != nil {
			return nil, fmt.Errorf("encrypt refresh: %w", err)
		}
	}
	var byoSecretEnc []byte
	if pending.ByoClientSecret != "" {
		byoSecretEnc, err = s.deps.Encryptor.Encrypt([]byte(pending.ByoClientSecret), aad)
		if err != nil {
			return nil, fmt.Errorf("encrypt byo secret: %w", err)
		}
	}
	now := s.deps.Now()
	cred := &models.GitIntegrationCredential{
		ID:                 id,
		UserID:             pending.UserID,
		Provider:           pending.Provider,
		Host:               pending.Host,
		ByoClientID:        pending.ByoClientID,
		ByoClientSecretEnc: byoSecretEnc,
		AccessTokenEnc:     accessEnc,
		RefreshTokenEnc:    refreshEnc,
		TokenType:          firstNonEmpty(tok.TokenType, "Bearer"),
		Scopes:             tok.Scopes,
		ExpiresAt:          tok.ExpiresAt,
		LastRefreshedAt:    &now,
	}
	if err := s.deps.Repo.Upsert(ctx, cred); err != nil {
		return nil, err
	}
	return cred, nil
}

func (s *gitIntegrationService) decryptToString(blob, aad []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	plain, err := s.deps.Encryptor.Decrypt(blob, aad)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *gitIntegrationService) publishStatus(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider, status events.IntegrationConnectionStatus, reason string, connectedAt, expiresAt *time.Time, remoteRevokeFailed bool) {
	if s.deps.Bus == nil || userID == uuid.Nil {
		return
	}
	reasonOut := reason
	if remoteRevokeFailed {
		// Кодируем флаг в Reason, чтобы не расширять схему DomainEvent. Frontend парсит и отображает уведомление.
		if reasonOut == "" {
			reasonOut = "remote_revoke_failed"
		} else {
			reasonOut = reasonOut + ":remote_revoke_failed"
		}
	}
	s.deps.Bus.Publish(ctx, events.IntegrationConnectionChanged{
		UserID:      userID,
		Provider:    string(provider),
		Status:      status,
		Reason:      reasonOut,
		ConnectedAt: connectedAt,
		ExpiresAt:   expiresAt,
		OccurredAt:  s.deps.Now(),
	})
}

func credToStatus(cred *models.GitIntegrationCredential) GitIntegrationStatus {
	now := cred.CreatedAt
	return GitIntegrationStatus{
		Provider:     cred.Provider,
		Connected:    true,
		Host:         cred.Host,
		AccountLogin: cred.AccountLogin,
		Scopes:       cred.Scopes,
		ExpiresAt:    cred.ExpiresAt,
		ConnectedAt:  &now,
	}
}

func mapErrReason(err error) string {
	switch {
	case errors.Is(err, ErrGitOAuthUserCancelled):
		return GitReasonUserCancelled
	case errors.Is(err, ErrGitOAuthInvalidGrant):
		return GitReasonInvalidGrant
	case errors.Is(err, ErrGitOAuthProviderUnreachable):
		return GitReasonProviderUnreachable
	case errors.Is(err, ErrGitOAuthNotConfigured):
		return GitReasonOAuthNotConfigured
	case errors.Is(err, ErrGitOAuthStateNotFound):
		return GitReasonStateExpired
	case errors.Is(err, ErrInvalidGitProviderHost), errors.Is(err, ErrPrivateGitProviderHost), errors.Is(err, ErrGitProviderResolveFailed):
		return GitReasonInvalidHost
	default:
		return GitReasonInternalError
	}
}

// gitlabSharedScopes — извлекает scopes из preconfigured shared-gitlab client (для BYO).
// Если shared не сконфигурирован — возвращает дефолтный набор «api+read_user».
func gitlabSharedScopes(c GitOAuthClient) string {
	if g, ok := c.(*gitlabOAuthClient); ok {
		return g.cfg.Scopes
	}
	return "api read_user"
}
