package service

import (
	"context"
	"sync"
	"testing"
	"time"

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
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)
	uid := uuid.New()

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
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)

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
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)
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
	svc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	worker := NewClaudeCodeTokenRefresher(repo, svc, nil)
	worker.Tick(context.Background())

	tok, err := svc.AccessTokenForSandbox(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, "new", tok)
}
