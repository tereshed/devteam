package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- in-memory mock ---

type mockClaudeCodeSubRepo struct {
	mu   sync.Mutex
	subs map[uuid.UUID]*models.ClaudeCodeSubscription
}

func newMockClaudeCodeSubRepo() *mockClaudeCodeSubRepo {
	return &mockClaudeCodeSubRepo{subs: map[uuid.UUID]*models.ClaudeCodeSubscription{}}
}
func (m *mockClaudeCodeSubRepo) Upsert(_ context.Context, sub *models.ClaudeCodeSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	clone := *sub
	m.subs[sub.UserID] = &clone
	return nil
}
func (m *mockClaudeCodeSubRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*models.ClaudeCodeSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[userID]
	if !ok {
		return nil, repository.ErrClaudeCodeSubscriptionNotFound
	}
	clone := *s
	return &clone, nil
}
func (m *mockClaudeCodeSubRepo) DeleteByUserID(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.subs[userID]; !ok {
		return repository.ErrClaudeCodeSubscriptionNotFound
	}
	delete(m.subs, userID)
	return nil
}
func (m *mockClaudeCodeSubRepo) ListExpiring(_ context.Context, now time.Time, within time.Duration) ([]models.ClaudeCodeSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	threshold := now.Add(within)
	var out []models.ClaudeCodeSubscription
	for _, s := range m.subs {
		if s.ExpiresAt != nil && !s.ExpiresAt.After(threshold) {
			out = append(out, *s)
		}
	}
	return out, nil
}

// --- mock OAuth provider ---

type stubOAuthProvider struct {
	initFn    func(ctx context.Context) (*ClaudeCodeDeviceInit, error)
	pollFn    func(ctx context.Context, deviceCode string) (*ClaudeCodeOAuthToken, error)
	refreshFn func(ctx context.Context, refreshToken string) (*ClaudeCodeOAuthToken, error)
	revokeCh  chan string
}

func (s *stubOAuthProvider) InitDeviceCode(ctx context.Context) (*ClaudeCodeDeviceInit, error) {
	return s.initFn(ctx)
}
func (s *stubOAuthProvider) PollDeviceToken(ctx context.Context, deviceCode string) (*ClaudeCodeOAuthToken, error) {
	return s.pollFn(ctx, deviceCode)
}
func (s *stubOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*ClaudeCodeOAuthToken, error) {
	return s.refreshFn(ctx, refreshToken)
}
func (s *stubOAuthProvider) Revoke(_ context.Context, token string) error {
	if s.revokeCh != nil {
		s.revokeCh <- token
	}
	return nil
}

// --- tests ---

// seedDeviceCode подменяет внутренний store на «уже инициировал user, код наш» — тестам не надо звать Init.
// Sprint 15.B (B2): без этого CompleteDeviceCode возвращает ErrDeviceCodeOwnerMismatch.
func seedDeviceCode(svc ClaudeCodeAuthService, uid uuid.UUID, code string) ClaudeCodeAuthService {
	store := NewInMemoryDeviceCodeStore()
	store.Put(code, uid, time.Hour)
	return WithClaudeCodeDeviceStore(svc, store)
}

func TestClaudeCodeAuth_CompleteDeviceCode_PersistsAndStatus(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	expires := time.Now().Add(time.Hour)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, deviceCode string) (*ClaudeCodeOAuthToken, error) {
			assert.Equal(t, "dc-123", deviceCode)
			return &ClaudeCodeOAuthToken{
				AccessToken: "access-abc", RefreshToken: "refresh-xyz",
				TokenType: "Bearer", Scopes: "user:inference", ExpiresAt: &expires,
			}, nil
		},
	}
	uid := uuid.New()
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc-123")

	status, err := svc.CompleteDeviceCode(context.Background(), uid, "dc-123")
	require.NoError(t, err)
	assert.True(t, status.Connected)
	assert.Equal(t, "Bearer", status.TokenType)
	assert.Equal(t, "user:inference", status.Scopes)

	// токен доступен для sandbox (без рефреша, т.к. ещё не истёк)
	tok, err := svc.AccessTokenForSandbox(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, "access-abc", tok)

	got, err := svc.Status(context.Background(), uid)
	require.NoError(t, err)
	assert.True(t, got.Connected)
}

func TestClaudeCodeAuth_Status_NoSubscription(t *testing.T) {
	svc := NewClaudeCodeAuthService(newMockClaudeCodeSubRepo(), NoopEncryptor{}, &stubOAuthProvider{})
	s, err := svc.Status(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, s.Connected)
}

func TestClaudeCodeAuth_AccessTokenForSandbox_RefreshesExpired(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()

	expired := time.Now().Add(-time.Minute)
	refreshed := time.Now().Add(2 * time.Hour)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{
				AccessToken: "old", RefreshToken: "r1", TokenType: "Bearer", ExpiresAt: &expired,
			}, nil
		},
		refreshFn: func(_ context.Context, refreshToken string) (*ClaudeCodeOAuthToken, error) {
			assert.Equal(t, "r1", refreshToken)
			return &ClaudeCodeOAuthToken{
				AccessToken: "fresh", RefreshToken: "r2", TokenType: "Bearer", ExpiresAt: &refreshed,
			}, nil
		},
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	tok, err := svc.AccessTokenForSandbox(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, "fresh", tok, "expired access token must trigger refresh")
}

func TestClaudeCodeAuth_Revoke_BestEffortAndDeletes(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()
	revokeSeen := make(chan string, 1)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			exp := time.Now().Add(time.Hour)
			return &ClaudeCodeOAuthToken{AccessToken: "a", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &exp}, nil
		},
		revokeCh: revokeSeen,
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	require.NoError(t, svc.Revoke(context.Background(), uid))
	select {
	case tok := <-revokeSeen:
		assert.Equal(t, "a", tok)
	case <-time.After(time.Second):
		t.Fatal("oauth.Revoke was not called")
	}

	// repo пустой → status = not connected.
	s, err := svc.Status(context.Background(), uid)
	require.NoError(t, err)
	assert.False(t, s.Connected)
}

// Sprint 15.B (B2) security regression — device_code инициатора A нельзя завершить от имени B.
func TestClaudeCodeAuth_CompleteDeviceCode_RejectsForeignUser(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	oauth := &stubOAuthProvider{
		initFn: func(_ context.Context) (*ClaudeCodeDeviceInit, error) {
			return &ClaudeCodeDeviceInit{
				DeviceCode: "dc-attacker", UserCode: "ABCD", VerificationURI: "https://x",
			}, nil
		},
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			t.Fatal("pollFn must not be reached when owner mismatch")
			return nil, nil
		},
	}
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)

	attacker := uuid.New()
	victim := uuid.New()

	// Атакующий инициирует flow.
	init, err := svc.InitDeviceCode(context.Background(), attacker)
	require.NoError(t, err)

	// Жертва с тем же device_code (через social engineering) пытается завершить — должна получить отказ.
	_, err = svc.CompleteDeviceCode(context.Background(), victim, init.DeviceCode)
	require.ErrorIs(t, err, ErrDeviceCodeOwnerMismatch)
}

// Sprint 15.B (B2) — неизвестный device_code также отвергается (нельзя поллить чужой код вслепую).
func TestClaudeCodeAuth_CompleteDeviceCode_RejectsUnknownDeviceCode(t *testing.T) {
	svc := NewClaudeCodeAuthService(newMockClaudeCodeSubRepo(), NoopEncryptor{}, &stubOAuthProvider{})
	_, err := svc.CompleteDeviceCode(context.Background(), uuid.New(), "never-initiated")
	require.ErrorIs(t, err, ErrDeviceCodeOwnerMismatch)
}

func TestClaudeCodeAuth_InitDeviceCode_PassesThrough(t *testing.T) {
	calls := 0
	oauth := &stubOAuthProvider{
		initFn: func(_ context.Context) (*ClaudeCodeDeviceInit, error) {
			calls++
			return &ClaudeCodeDeviceInit{DeviceCode: "dc", UserCode: "ABCD-EFGH", VerificationURI: "https://x"}, nil
		},
	}
	svc := NewClaudeCodeAuthService(newMockClaudeCodeSubRepo(), NoopEncryptor{}, oauth)
	init, err := svc.InitDeviceCode(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "dc", init.DeviceCode)
}

// Sprint 15.minor regression — отмена caller-context ПОСЛЕ начала refresh не приводит
// к потере свежего refresh_token. WithoutCancel внутри singleflight гарантирует, что
// persistToken дойдёт до БД.
func TestClaudeCodeAuth_RefreshOne_CallerCtxCancel_DoesNotLoseToken(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()

	soon := time.Now().Add(2 * time.Minute)
	refreshed := time.Now().Add(2 * time.Hour)
	oauthSlow := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{
				AccessToken: "a", RefreshToken: "r-old", TokenType: "Bearer", ExpiresAt: &soon,
			}, nil
		},
		refreshFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			// Симулируем задержку: caller отменит свой ctx за это время.
			time.Sleep(80 * time.Millisecond)
			return &ClaudeCodeOAuthToken{
				AccessToken: "fresh", RefreshToken: "r-new", TokenType: "Bearer", ExpiresAt: &refreshed,
			}, nil
		},
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauthSlow), uid, "dc")
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)
	sub, err := repo.GetByUserID(context.Background(), uid)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() { doneCh <- svc.RefreshOne(ctx, sub) }()
	time.Sleep(20 * time.Millisecond)
	cancel() // caller отменил context — но refresh должен довести persist до конца.

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshOne did not return")
	}

	// Проверяем: в БД лежит "fresh" access_token (а НЕ "a" из-за прерывания).
	tok, err := svc.AccessTokenForSandbox(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, "fresh", tok,
		"persist must complete even after caller ctx is cancelled (M3 WithoutCancel)")
}

// Sprint 15.B (B3) regression — параллельные RefreshOne для одного user_id коалесцируются.
// Anthropic ротейтит refresh_token; второй call без singleflight получал бы invalid_grant.
func TestClaudeCodeAuth_RefreshOne_Singleflight_CoalescesConcurrent(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()

	var refreshCalls atomic.Int32
	soon := time.Now().Add(time.Minute)
	refreshed := time.Now().Add(2 * time.Hour)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{
				AccessToken: "a", RefreshToken: "r-original", TokenType: "Bearer", ExpiresAt: &soon,
			}, nil
		},
		refreshFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			refreshCalls.Add(1)
			// небольшая задержка, чтобы concurrent caller-ы попали в одну группу
			time.Sleep(30 * time.Millisecond)
			return &ClaudeCodeOAuthToken{
				AccessToken: "fresh", RefreshToken: "r-rotated", TokenType: "Bearer", ExpiresAt: &refreshed,
			}, nil
		},
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	sub, err := repo.GetByUserID(context.Background(), uid)
	require.NoError(t, err)

	const concurrency = 5
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_ = svc.RefreshOne(context.Background(), sub)
		}()
	}
	wg.Wait()

	// Должен пройти только один вызов RefreshToken, остальные ждут результат первого.
	assert.Equal(t, int32(1), refreshCalls.Load(),
		"singleflight must coalesce concurrent refreshes per user_id")
}

// --- UI Refactoring §4a.4: тесты публикации IntegrationConnectionChanged. ---

// waitForIntegrationEvent читает один event указанного типа из подписки шины.
func waitForIntegrationEvent(t *testing.T, ch <-chan events.DomainEvent) events.IntegrationConnectionChanged {
	t.Helper()
	for {
		select {
		case ev := <-ch:
			if ice, ok := ev.(events.IntegrationConnectionChanged); ok {
				return ice
			}
			// игнорируем посторонние события
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for IntegrationConnectionChanged event")
		}
	}
}

func newClaudeCodeAuthSvcWithBus(t *testing.T) (
	ClaudeCodeAuthService,
	*stubOAuthProvider,
	*mockClaudeCodeSubRepo,
	<-chan events.DomainEvent,
	func(),
) {
	t.Helper()
	bus := events.NewInMemoryBus(nil, nil)
	ch, unsub := bus.Subscribe("test_integration_status", 16)
	repo := newMockClaudeCodeSubRepo()
	oauth := &stubOAuthProvider{}
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)
	svc = WithClaudeCodeEventBus(svc, bus)
	cleanup := func() {
		unsub()
		bus.Close()
	}
	return svc, oauth, repo, ch, cleanup
}

// 4a.5 case "connected": успешный обмен device_code → access_token.
func TestClaudeCodeAuth_PublishesIntegrationEvent_OnSuccess(t *testing.T) {
	svc, oauth, _, ch, cleanup := newClaudeCodeAuthSvcWithBus(t)
	defer cleanup()

	expires := time.Now().Add(time.Hour).UTC()
	oauth.pollFn = func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
		return &ClaudeCodeOAuthToken{
			AccessToken: "a", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &expires,
		}, nil
	}
	uid := uuid.New()
	svc = seedDeviceCode(svc, uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	ev := waitForIntegrationEvent(t, ch)
	assert.Equal(t, uid, ev.UserID)
	assert.Equal(t, ProviderClaudeCodeOAuth, ev.Provider)
	assert.Equal(t, events.IntegrationStatusConnected, ev.Status)
	assert.Equal(t, "", ev.Reason)
	require.NotNil(t, ev.ConnectedAt)
	require.NotNil(t, ev.ExpiresAt)
}

// 4a.5 case "user cancelled (?error=access_denied)": Status=error, Reason=user_cancelled.
func TestClaudeCodeAuth_PublishesIntegrationEvent_OnAccessDenied(t *testing.T) {
	svc, oauth, _, ch, cleanup := newClaudeCodeAuthSvcWithBus(t)
	defer cleanup()

	oauth.pollFn = func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
		return nil, ErrAccessDenied
	}
	uid := uuid.New()
	svc = seedDeviceCode(svc, uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.ErrorIs(t, err, ErrAccessDenied)

	ev := waitForIntegrationEvent(t, ch)
	assert.Equal(t, events.IntegrationStatusError, ev.Status)
	assert.Equal(t, ReasonUserCancelled, ev.Reason)
}

// 4a.5 case "network / провайдер недоступен": Status=error, Reason=provider_unreachable.
func TestClaudeCodeAuth_PublishesIntegrationEvent_OnProviderError(t *testing.T) {
	svc, oauth, _, ch, cleanup := newClaudeCodeAuthSvcWithBus(t)
	defer cleanup()

	oauth.pollFn = func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
		return nil, fmt.Errorf("dial tcp: i/o timeout")
	}
	uid := uuid.New()
	svc = seedDeviceCode(svc, uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrAccessDenied))

	ev := waitForIntegrationEvent(t, ch)
	assert.Equal(t, events.IntegrationStatusError, ev.Status)
	assert.Equal(t, ReasonProviderUnreachable, ev.Reason)
}

// 4a.5 case "revoke": Status=disconnected.
func TestClaudeCodeAuth_PublishesIntegrationEvent_OnRevoke(t *testing.T) {
	svc, oauth, _, ch, cleanup := newClaudeCodeAuthSvcWithBus(t)
	defer cleanup()

	expires := time.Now().Add(time.Hour).UTC()
	oauth.pollFn = func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
		return &ClaudeCodeOAuthToken{
			AccessToken: "a", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &expires,
		}, nil
	}
	uid := uuid.New()
	svc = seedDeviceCode(svc, uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)
	// drain connected event
	_ = waitForIntegrationEvent(t, ch)

	require.NoError(t, svc.Revoke(context.Background(), uid))
	ev := waitForIntegrationEvent(t, ch)
	assert.Equal(t, events.IntegrationStatusDisconnected, ev.Status)
}

// 4a.5 case "pending — поллинг не публикует промежуточные события".
func TestClaudeCodeAuth_DoesNotPublishOnAuthorizationPending(t *testing.T) {
	svc, oauth, _, ch, cleanup := newClaudeCodeAuthSvcWithBus(t)
	defer cleanup()

	oauth.pollFn = func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
		return nil, ErrAuthorizationPending
	}
	uid := uuid.New()
	svc = seedDeviceCode(svc, uid, "dc")

	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.ErrorIs(t, err, ErrAuthorizationPending)

	select {
	case ev := <-ch:
		t.Fatalf("unexpected event for authorization_pending: %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// ok — публикации не было
	}
}

func TestClaudeCodeTokenRefresher_Tick_RefreshesExpiring(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()

	soon := time.Now().Add(2 * time.Minute) // в пределах refreshAhead=10m
	refreshed := time.Now().Add(2 * time.Hour)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{AccessToken: "old", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &soon}, nil
		},
		refreshFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{AccessToken: "new", RefreshToken: "r2", TokenType: "Bearer", ExpiresAt: &refreshed}, nil
		},
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	worker := NewClaudeCodeTokenRefresher(repo, svc, nil)
	worker.Tick(context.Background())

	tok, err := svc.AccessTokenForSandbox(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, "new", tok)
}
