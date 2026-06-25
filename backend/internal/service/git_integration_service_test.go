package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeRepo struct {
	mu    sync.Mutex
	byKey map[string]*models.GitIntegrationCredential // key=userID|provider
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byKey: map[string]*models.GitIntegrationCredential{}} }

func fakeRepoKey(uid uuid.UUID, p models.GitIntegrationProvider) string {
	return uid.String() + "|" + string(p)
}

func (r *fakeRepo) Upsert(_ context.Context, c *models.GitIntegrationCredential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.byKey[fakeRepoKey(c.UserID, c.Provider)] = &cp
	return nil
}

func (r *fakeRepo) GetByUserAndProvider(_ context.Context, uid uuid.UUID, p models.GitIntegrationProvider) (*models.GitIntegrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byKey[fakeRepoKey(uid, p)]
	if !ok {
		return nil, repository.ErrGitIntegrationNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *fakeRepo) ListByUserID(_ context.Context, uid uuid.UUID) ([]models.GitIntegrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.GitIntegrationCredential
	for _, c := range r.byKey {
		if c.UserID == uid {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *fakeRepo) DeleteByUserAndProvider(_ context.Context, uid uuid.UUID, p models.GitIntegrationProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[fakeRepoKey(uid, p)]; !ok {
		return repository.ErrGitIntegrationNotFound
	}
	delete(r.byKey, fakeRepoKey(uid, p))
	return nil
}

func (r *fakeRepo) UpdateTokens(_ context.Context, id uuid.UUID, accessTokenEnc, refreshTokenEnc []byte, expiresAt, lastRefreshedAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.byKey {
		if c.ID == id {
			c.AccessTokenEnc = accessTokenEnc
			if len(refreshTokenEnc) > 0 {
				c.RefreshTokenEnc = refreshTokenEnc
			}
			c.ExpiresAt = expiresAt
			c.LastRefreshedAt = lastRefreshedAt
			return nil
		}
	}
	return repository.ErrGitIntegrationNotFound
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*models.GitIntegrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.byKey {
		if c.ID == id {
			cp := *c
			return &cp, nil
		}
	}
	return nil, repository.ErrGitIntegrationNotFound
}

func (r *fakeRepo) ListByUserAndProvider(_ context.Context, uid uuid.UUID, p models.GitIntegrationProvider) ([]models.GitIntegrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.GitIntegrationCredential
	for _, c := range r.byKey {
		if c.UserID == uid && c.Provider == p {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *fakeRepo) DeleteByID(_ context.Context, uid, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, c := range r.byKey {
		if c.ID == id && c.UserID == uid {
			delete(r.byKey, k)
			return nil
		}
	}
	return repository.ErrGitIntegrationNotFound
}

func (r *fakeRepo) DeleteLegacyUnlabeled(_ context.Context, uid uuid.UUID, p models.GitIntegrationProvider, host string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, c := range r.byKey {
		if c.UserID == uid && c.Provider == p && c.Host == host && c.AccountLogin == "" {
			delete(r.byKey, k)
		}
	}
	return nil
}

// ─── recording event bus ────────────────────────────────────────────────────

type recordingBus struct {
	mu     sync.Mutex
	events []events.DomainEvent
}

func (b *recordingBus) Publish(_ context.Context, ev events.DomainEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ev)
}
func (b *recordingBus) Subscribe(_ string, _ int) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch)
	return ch, func() {}
}
func (b *recordingBus) Close() {}

func (b *recordingBus) lastIntegration(t *testing.T) events.IntegrationConnectionChanged {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := len(b.events) - 1; i >= 0; i-- {
		if e, ok := b.events[i].(events.IntegrationConnectionChanged); ok {
			return e
		}
	}
	t.Fatal("no IntegrationConnectionChanged events recorded")
	return events.IntegrationConnectionChanged{}
}

// ─── fake oauth client ───────────────────────────────────────────────────────

type fakeOAuthClient struct {
	tok       *GitOAuthToken
	login     string
	exchErr   error
	revokeErr error
	exchCount int
	revoked   bool
	calls     []string
}

func (c *fakeOAuthClient) AuthCodeURL(state, redirectURI string) string {
	c.calls = append(c.calls, "auth")
	return "https://provider.example/auth?state=" + state + "&redirect=" + redirectURI
}
func (c *fakeOAuthClient) ExchangeCode(_ context.Context, code, _ string) (*GitOAuthToken, error) {
	c.exchCount++
	c.calls = append(c.calls, "exchange:"+code)
	if c.exchErr != nil {
		return nil, c.exchErr
	}
	return c.tok, nil
}
func (c *fakeOAuthClient) GetAuthenticatedLogin(_ context.Context, _ string) (string, error) {
	c.calls = append(c.calls, "login")
	return c.login, nil
}
func (c *fakeOAuthClient) RefreshToken(_ context.Context, _ string) (*GitOAuthToken, error) {
	c.calls = append(c.calls, "refresh")
	if c.tok != nil {
		return c.tok, nil
	}
	return &GitOAuthToken{AccessToken: "refreshed", TokenType: "Bearer"}, nil
}
func (c *fakeOAuthClient) Revoke(_ context.Context, _ string) error {
	c.calls = append(c.calls, "revoke")
	c.revoked = true
	return c.revokeErr
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func testKey32(t *testing.T) []byte {
	t.Helper()
	k, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil || len(k) != 32 {
		t.Fatalf("bad test key: %v", err)
	}
	return k
}

func newTestService(t *testing.T, ghClient, glClient GitOAuthClient, repo repository.GitIntegrationCredentialRepository, bus events.EventBus, logBuf *bytes.Buffer) GitIntegrationService {
	t.Helper()
	enc, err := crypto.NewAESEncryptor(testKey32(t))
	if err != nil {
		t.Fatalf("enc: %v", err)
	}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return NewGitIntegrationService(GitIntegrationServiceDeps{
		Repo:       repo,
		Encryptor:  enc,
		GitHub:     ghClient,
		GitLab:     glClient,
		Validator:  NewGitProviderHostValidator(&fakeResolver{responses: [][]net.IP{{net.ParseIP("8.8.8.8")}, {net.ParseIP("8.8.8.8")}}}, true),
		StateStore: NewInMemoryGitOAuthStateStore(),
		Bus:        bus,
		Logger:     logger,
		Now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	})
}

// ─── tests: 4 OAuth scenarios для GitHub и GitLab ────────────────────────────

func TestGitIntegration_GitHub_Success(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	gh := &fakeOAuthClient{tok: &GitOAuthToken{
		AccessToken:  "ghp_secret_xyz",
		RefreshToken: "ghr_secret_xyz",
		TokenType:    "Bearer",
		Scopes:       "repo,read:user",
		ExpiresAt:    &exp,
	}}
	repo := newFakeRepo()
	bus := &recordingBus{}
	logBuf := &bytes.Buffer{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, logBuf)

	uid := uuid.New()
	init, err := svc.InitGitHub(context.Background(), uid, "https://app.example/cb")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(init.AuthorizeURL, init.State) {
		t.Fatal("state must appear in authorize url")
	}

	res, err := svc.HandleCallback(context.Background(), "auth_code_xyz", init.State, "")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if !res.Status.Connected {
		t.Fatal("expected connected")
	}

	ev := bus.lastIntegration(t)
	if ev.Status != events.IntegrationStatusConnected || ev.Provider != "github" {
		t.Fatalf("bad event: %+v", ev)
	}

	// Лог не должен содержать access_token plaintext.
	if strings.Contains(logBuf.String(), "ghp_secret_xyz") {
		t.Fatal("log leaks access token plaintext")
	}
}

func TestGitIntegration_GitHub_AccessDenied(t *testing.T) {
	gh := &fakeOAuthClient{}
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})
	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")

	_, err := svc.HandleCallback(context.Background(), "", init.State, "access_denied")
	if !errors.Is(err, ErrGitOAuthUserCancelled) {
		t.Fatalf("expected user-cancelled, got %v", err)
	}
	ev := bus.lastIntegration(t)
	if ev.Status != events.IntegrationStatusError || ev.Reason != GitReasonUserCancelled {
		t.Fatalf("bad event: %+v", ev)
	}
}

func TestGitIntegration_GitHub_InvalidGrant(t *testing.T) {
	gh := &fakeOAuthClient{exchErr: ErrGitOAuthInvalidGrant}
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")
	_, err := svc.HandleCallback(context.Background(), "bad_code", init.State, "")
	if !errors.Is(err, ErrGitOAuthInvalidGrant) {
		t.Fatalf("expected invalid_grant, got %v", err)
	}
	ev := bus.lastIntegration(t)
	if ev.Status != events.IntegrationStatusError || ev.Reason != GitReasonInvalidGrant {
		t.Fatalf("bad event: %+v", ev)
	}
}

func TestGitIntegration_GitHub_ProviderUnreachable(t *testing.T) {
	gh := &fakeOAuthClient{exchErr: ErrGitOAuthProviderUnreachable}
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")
	_, err := svc.HandleCallback(context.Background(), "code", init.State, "")
	if !errors.Is(err, ErrGitOAuthProviderUnreachable) {
		t.Fatalf("expected unreachable, got %v", err)
	}
	ev := bus.lastIntegration(t)
	if ev.Reason != GitReasonProviderUnreachable {
		t.Fatalf("bad reason: %s", ev.Reason)
	}
}

// ─── revoke order + fail-soft ────────────────────────────────────────────────

func TestGitIntegration_RevokeCallsHTTPBeforeDelete(t *testing.T) {
	tok := &GitOAuthToken{AccessToken: "ghp_xx", TokenType: "Bearer"}
	gh := &fakeOAuthClient{tok: tok}
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")
	_, err := svc.HandleCallback(context.Background(), "c", init.State, "")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}

	// Сбрасываем вызовы fake-клиента и удаляем запись.
	gh.calls = nil
	remoteFailed, err := svc.Revoke(context.Background(), uid, models.GitIntegrationProviderGitHub)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if remoteFailed {
		t.Fatal("expected remoteFailed=false")
	}
	if len(gh.calls) == 0 || gh.calls[0] != "revoke" {
		t.Fatalf("expected revoke called first, got %v", gh.calls)
	}
	// Проверка: запись удалена из repo.
	if _, err := repo.GetByUserAndProvider(context.Background(), uid, models.GitIntegrationProviderGitHub); !errors.Is(err, repository.ErrGitIntegrationNotFound) {
		t.Fatalf("record still present: %v", err)
	}
}

func TestGitIntegration_RevokeFailSoftLocalStillRemoved(t *testing.T) {
	tok := &GitOAuthToken{AccessToken: "ghp_xx", TokenType: "Bearer"}
	gh := &fakeOAuthClient{tok: tok, revokeErr: errors.New("network down")}
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")
	if _, err := svc.HandleCallback(context.Background(), "c", init.State, ""); err != nil {
		t.Fatalf("callback: %v", err)
	}

	remoteFailed, err := svc.Revoke(context.Background(), uid, models.GitIntegrationProviderGitHub)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !remoteFailed {
		t.Fatal("expected remoteFailed=true")
	}
	if _, err := repo.GetByUserAndProvider(context.Background(), uid, models.GitIntegrationProviderGitHub); !errors.Is(err, repository.ErrGitIntegrationNotFound) {
		t.Fatalf("record still present after fail-soft: %v", err)
	}

	// WS-event на disconnect — Reason содержит remote_revoke_failed.
	ev := bus.lastIntegration(t)
	if ev.Status != events.IntegrationStatusDisconnected || !strings.Contains(ev.Reason, "remote_revoke_failed") {
		t.Fatalf("bad event: %+v", ev)
	}
}

// ─── lognode: redaction sanity ───────────────────────────────────────────────

func TestGitIntegration_LogsDoNotLeakSecretsOnProviderError(t *testing.T) {
	// Эмулируем «болтливую» ошибку от провайдера: внутри строки — пара access_token=xxx.
	leakyErr := errors.New(`{"error":"server_error","raw":"access_token=ghp_secret_leak_xxx"}`)
	gh := &fakeOAuthClient{exchErr: leakyErr}
	repo := newFakeRepo()
	bus := &recordingBus{}
	logBuf := &bytes.Buffer{}
	svc := newTestService(t, gh, &fakeOAuthClient{}, repo, bus, logBuf)

	uid := uuid.New()
	init, _ := svc.InitGitHub(context.Background(), uid, "https://app/cb")
	_, _ = svc.HandleCallback(context.Background(), "c", init.State, "")

	if strings.Contains(logBuf.String(), "ghp_secret_leak_xxx") {
		t.Fatalf("log leaks plaintext: %s", logBuf.String())
	}
	// SafeRawAttr должен оставить метку <redacted len=N>.
	if !strings.Contains(logBuf.String(), "head_sha256_8") && !strings.Contains(logBuf.String(), "redacted") {
		t.Fatalf("expected redact marker in log: %s", logBuf.String())
	}
}

// ─── BYO GitLab: host validation + DNS rebinding ─────────────────────────────

func TestGitIntegration_BYOGitLab_RejectsPrivateHost(t *testing.T) {
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, &fakeOAuthClient{}, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	_, err := svc.InitGitLabBYO(context.Background(), uid, "https://app/cb", BYOGitLabInit{
		Host: "https://192.168.1.5", ClientID: "cid", ClientSecret: "csec",
	})
	if !errors.Is(err, ErrPrivateGitProviderHost) {
		t.Fatalf("expected private host rejection, got %v", err)
	}
}

func TestGitIntegration_BYOGitLab_RejectsUserinfo(t *testing.T) {
	repo := newFakeRepo()
	bus := &recordingBus{}
	svc := newTestService(t, &fakeOAuthClient{}, &fakeOAuthClient{}, repo, bus, &bytes.Buffer{})

	uid := uuid.New()
	_, err := svc.InitGitLabBYO(context.Background(), uid, "https://app/cb", BYOGitLabInit{
		Host: "https://u:p@gitlab.example", ClientID: "cid", ClientSecret: "csec",
	})
	if !errors.Is(err, ErrInvalidGitProviderHost) {
		t.Fatalf("expected userinfo rejection, got %v", err)
	}
}

func TestGitIntegration_BYOGitLab_DNSRebindingDefence(t *testing.T) {
	// Первый резолв — public, второй — 127.0.0.1.
	// validate всегда отрабатывает синхронно, dial — на отдельном этапе.
	resolver := &fakeResolver{responses: [][]net.IP{
		{net.ParseIP("8.8.8.8")},
	}}
	v := NewGitProviderHostValidator(resolver, true)

	canon, allowed, err := v.ValidateGitProviderHost(context.Background(), "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_ = canon

	// Поднимаем "fake" HTTP server и публикуем его IP в allow-list.
	// Затем dial-им на ip=127.0.0.1 (rebind) — должен упасть DisallowedDialTarget.
	dialFn := safeDialContextFactory(&net.Dialer{}, allowed)
	_, err = dialFn(context.Background(), "tcp", "127.0.0.1:443")
	if !errors.Is(err, ErrDisallowedDialTarget) {
		t.Fatalf("expected disallowed dial on rebind, got %v", err)
	}
}

// ─── integration with a real httptest GitLab BYO server ──────────────────────

func TestGitIntegration_BYOGitLab_Success_WithSafeClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = w.Write([]byte(`{"access_token":"glat_xxx","refresh_token":"glrt_xxx","token_type":"Bearer","expires_in":3600,"scope":"api"}`))
		case "/oauth/revoke":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvHost, srvPort, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	srvIP := net.ParseIP(srvHost)

	// Резолвер вернёт IP test-сервера.
	resolver := &fakeResolver{responses: [][]net.IP{{srvIP}, {srvIP}, {srvIP}, {srvIP}}}
	validator := NewGitProviderHostValidator(resolver, false) // dev, чтобы пустить 127.0.0.1

	repo := newFakeRepo()
	bus := &recordingBus{}
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	svc := NewGitIntegrationService(GitIntegrationServiceDeps{
		Repo:       repo,
		Encryptor:  enc,
		GitHub:     &fakeOAuthClient{},
		GitLab:     &fakeOAuthClient{},
		Validator:  validator,
		StateStore: NewInMemoryGitOAuthStateStore(),
		Bus:        bus,
		Logger:     logging.NopLogger(),
		Now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	})

	uid := uuid.New()
	init, err := svc.InitGitLabBYO(context.Background(), uid, "https://app/cb", BYOGitLabInit{
		Host: "http://" + net.JoinHostPort("gitlab.example.com", srvPort), ClientID: "cid", ClientSecret: "csec",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(init.AuthorizeURL, "/oauth/authorize") {
		t.Fatalf("bad authorize url: %s", init.AuthorizeURL)
	}

	res, err := svc.HandleCallback(context.Background(), "auth_code", init.State, "")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if !res.Status.Connected {
		t.Fatal("expected connected")
	}
	// Проверяем хранение: byo_client_id — plain, byo_client_secret — encrypted blob (≥29 байт).
	cred, _ := repo.GetByUserAndProvider(context.Background(), uid, models.GitIntegrationProviderGitLab)
	if cred.ByoClientID != "cid" {
		t.Fatalf("byo_client_id mismatch: %q", cred.ByoClientID)
	}
	if len(cred.ByoClientSecretEnc) < crypto.MinCiphertextBlobLen {
		t.Fatal("byo_client_secret_enc looks unencrypted")
	}
	if bytes.Contains(cred.ByoClientSecretEnc, []byte("csec")) {
		t.Fatal("byo_client_secret_enc contains plaintext")
	}
}

// ─── state hygiene ───────────────────────────────────────────────────────────

func TestGitOAuthStateStore_OneShot(t *testing.T) {
	store := NewInMemoryGitOAuthStateStore()
	uid := uuid.New()
	tok, err := store.New(GitOAuthState{UserID: uid, Provider: models.GitIntegrationProviderGitHub}, time.Minute)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := store.Consume(tok); err != nil {
		t.Fatalf("consume1: %v", err)
	}
	if _, err := store.Consume(tok); !errors.Is(err, ErrGitOAuthStateNotFound) {
		t.Fatalf("expected one-shot failure, got %v", err)
	}
}

func TestGitOAuthStateStore_Expired(t *testing.T) {
	store := &inMemoryGitOAuthStateStore{entries: map[string]stateEntry{}, clock: time.Now}
	tok, _ := store.New(GitOAuthState{UserID: uuid.New(), Provider: models.GitIntegrationProviderGitHub}, time.Nanosecond)
	time.Sleep(time.Millisecond)
	if _, err := store.Consume(tok); !errors.Is(err, ErrGitOAuthStateNotFound) {
		t.Fatalf("expected expired state error, got %v", err)
	}
}

type rewriteTransport struct {
	targetURL string
	transport http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(t.targetURL)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return t.transport.RoundTrip(req)
}

func TestGitIntegration_CreateRepository_GitHub_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/user/repos" {
			// Check request body for auto_init
			var body struct {
				Name     string `json:"name"`
				Private  bool   `json:"private"`
				AutoInit bool   `json:"auto_init"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Name != "my-new-repo" || !body.Private || !body.AutoInit {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"name": "my-new-repo",
				"full_name": "test-user/my-new-repo",
				"html_url": "https://github.com/test-user/my-new-repo",
				"clone_url": "https://github.com/test-user/my-new-repo.git",
				"description": "hello"
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	repo := newFakeRepo()
	uid := uuid.New()
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	// Add mock credential
	credID := uuid.New()
	tokenEnc, _ := enc.Encrypt([]byte("ghp_fake_token"), repository.GitIntegrationCredentialAAD(credID))
	_ = repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             credID,
		UserID:         uid,
		Provider:       models.GitIntegrationProviderGitHub,
		AccessTokenEnc: tokenEnc,
	})

	svc := NewGitIntegrationService(GitIntegrationServiceDeps{
		Repo:      repo,
		Encryptor: enc,
		Logger:    logging.NopLogger(),
	})

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Transport: &rewriteTransport{
			targetURL: srv.URL,
			transport: http.DefaultTransport,
		},
	})

	res, err := svc.CreateRepository(ctx, uid, models.GitIntegrationProviderGitHub, uuid.Nil, "my-new-repo", true, "hello")
	if err != nil {
		t.Fatalf("CreateRepository failed: %v", err)
	}
	if res.Name != "my-new-repo" || res.FullName != "test-user/my-new-repo" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestGitIntegration_CreateRepository_GitLab_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v4/projects" {
			var body struct {
				Name                 string `json:"name"`
				Visibility           string `json:"visibility"`
				InitializeWithReadme bool   `json:"initialize_with_readme"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Name != "my-new-repo" || body.Visibility != "private" || !body.InitializeWithReadme {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"name": "my-new-repo",
				"path_with_namespace": "test-user/my-new-repo",
				"web_url": "https://gitlab.com/test-user/my-new-repo",
				"http_url_to_repo": "https://gitlab.com/test-user/my-new-repo.git",
				"description": "hello"
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	srvHost, srvPort, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	srvIP := net.ParseIP(srvHost)

	resolver := &fakeResolver{responses: [][]net.IP{{srvIP}, {srvIP}}}
	validator := NewGitProviderHostValidator(resolver, false)

	repo := newFakeRepo()
	uid := uuid.New()
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	credID := uuid.New()
	tokenEnc, _ := enc.Encrypt([]byte("glpat-fake"), repository.GitIntegrationCredentialAAD(credID))
	_ = repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             credID,
		UserID:         uid,
		Provider:       models.GitIntegrationProviderGitLab,
		Host:           "http://" + net.JoinHostPort("gitlab.example.com", srvPort),
		AccessTokenEnc: tokenEnc,
	})

	svc := NewGitIntegrationService(GitIntegrationServiceDeps{
		Repo:      repo,
		Encryptor: enc,
		Validator: validator,
		Logger:    logging.NopLogger(),
	})

	res, err := svc.CreateRepository(context.Background(), uid, models.GitIntegrationProviderGitLab, uuid.Nil, "my-new-repo", true, "hello")
	if err != nil {
		t.Fatalf("CreateRepository failed: %v", err)
	}
	if res.Name != "my-new-repo" || res.FullName != "test-user/my-new-repo" {
		t.Errorf("unexpected response: %+v", res)
	}
}

// TestGitIntegration_ListRepositories_ExplicitAccountID_UsesThatAccount — ядро фикса
// мульти-аккаунта: при заданном accountID листинг идёт через ВЫБРАННЫЙ аккаунт (его токен),
// а не через «первый аккаунт провайдера».
func TestGitIntegration_ListRepositories_ExplicitAccountID_UsesThatAccount(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v4/projects" {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"name":"acc2-repo","path_with_namespace":"acc2/repo","web_url":"https://gitlab.example.com/acc2/repo","http_url_to_repo":"https://gitlab.example.com/acc2/repo.git","default_branch":"main"}]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	srvHost, srvPort, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	srvIP := net.ParseIP(srvHost)
	resolver := &fakeResolver{responses: [][]net.IP{{srvIP}, {srvIP}}}
	validator := NewGitProviderHostValidator(resolver, false)

	repo := newFakeRepo()
	uid := uuid.New()
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	credID := uuid.New()
	tokenEnc, _ := enc.Encrypt([]byte("glpat-acc2"), repository.GitIntegrationCredentialAAD(credID))
	_ = repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             credID,
		UserID:         uid,
		Provider:       models.GitIntegrationProviderGitLab,
		Host:           "http://" + net.JoinHostPort("gitlab.example.com", srvPort),
		AccountLogin:   "acc2",
		AccessTokenEnc: tokenEnc,
	})

	svc := NewGitIntegrationService(GitIntegrationServiceDeps{
		Repo:      repo,
		Encryptor: enc,
		Validator: validator,
		Logger:    logging.NopLogger(),
	})

	repos, err := svc.ListRepositories(context.Background(), uid, models.GitIntegrationProviderGitLab, credID)
	if err != nil {
		t.Fatalf("ListRepositories failed: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName != "acc2/repo" {
		t.Fatalf("unexpected repos: %+v", repos)
	}
	if gotAuth != "Bearer glpat-acc2" {
		t.Errorf("expected token of the SELECTED account, got Authorization=%q", gotAuth)
	}
}

// TestGitIntegration_ListRepositories_AccountWrongOwner — чужой accountID не должен светиться
// как существующий: fail-loud «не найдено», без молчаливого фолбэка на свой первый аккаунт.
func TestGitIntegration_ListRepositories_AccountWrongOwner(t *testing.T) {
	repo := newFakeRepo()
	owner := uuid.New()
	other := uuid.New()
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	credID := uuid.New()
	tokenEnc, _ := enc.Encrypt([]byte("glpat-x"), repository.GitIntegrationCredentialAAD(credID))
	_ = repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             credID,
		UserID:         owner,
		Provider:       models.GitIntegrationProviderGitLab,
		AccessTokenEnc: tokenEnc,
	})

	svc := NewGitIntegrationService(GitIntegrationServiceDeps{Repo: repo, Encryptor: enc, Logger: logging.NopLogger()})

	_, err := svc.ListRepositories(context.Background(), other, models.GitIntegrationProviderGitLab, credID)
	if !errors.Is(err, repository.ErrGitIntegrationNotFound) {
		t.Fatalf("expected ErrGitIntegrationNotFound for foreign account, got %v", err)
	}
}

// TestGitIntegration_ListRepositories_AccountProviderMismatch — accountID другого провайдера
// должен явно отвергаться (ErrInvalidInput), а не молча использоваться.
func TestGitIntegration_ListRepositories_AccountProviderMismatch(t *testing.T) {
	repo := newFakeRepo()
	uid := uuid.New()
	enc, _ := crypto.NewAESEncryptor(testKey32(t))
	credID := uuid.New()
	tokenEnc, _ := enc.Encrypt([]byte("ghp_x"), repository.GitIntegrationCredentialAAD(credID))
	_ = repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             credID,
		UserID:         uid,
		Provider:       models.GitIntegrationProviderGitHub,
		AccessTokenEnc: tokenEnc,
	})

	svc := NewGitIntegrationService(GitIntegrationServiceDeps{Repo: repo, Encryptor: enc, Logger: logging.NopLogger()})

	// просим gitlab-репозитории, передав github-аккаунт
	_, err := svc.ListRepositories(context.Background(), uid, models.GitIntegrationProviderGitLab, credID)
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for provider mismatch, got %v", err)
	}
}
