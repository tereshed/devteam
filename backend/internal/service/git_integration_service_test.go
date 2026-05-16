package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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
