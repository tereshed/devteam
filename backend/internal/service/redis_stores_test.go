package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// newMiniredisClient поднимает in-memory Redis (miniredis) и go-redis клиент к нему.
func newMiniredisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRedisLocker_LockUnlockRefresh(t *testing.T) {
	l := NewRedisLocker(newMiniredisClient(t))
	ctx := context.Background()

	id, err := l.Lock(ctx, "proj-1", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	// Повторный захват занятого ключа.
	_, err = l.Lock(ctx, "proj-1", time.Minute)
	require.ErrorIs(t, err, ErrLockHeld)

	// Unlock/Refresh чужим владельцем.
	require.ErrorIs(t, l.Unlock(ctx, "proj-1", "not-owner"), ErrLockNotOwned)
	require.ErrorIs(t, l.Refresh(ctx, "proj-1", "not-owner", time.Minute), ErrLockNotOwned)

	// Refresh и Unlock владельцем.
	require.NoError(t, l.Refresh(ctx, "proj-1", id, time.Minute))
	require.NoError(t, l.Unlock(ctx, "proj-1", id))

	// После освобождения ключ снова свободен.
	id2, err := l.Lock(ctx, "proj-1", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, id2)

	// Unlock отсутствующего ключа — не ошибка (как у InMemoryLocker).
	require.NoError(t, l.Unlock(ctx, "absent", "x"))
	// Refresh отсутствующего — ErrLockNotOwned.
	require.ErrorIs(t, l.Refresh(ctx, "absent", "x", time.Minute), ErrLockNotOwned)
}

func TestRedisDeviceCodeStore_PutGetDelete(t *testing.T) {
	s := NewRedisDeviceCodeStore(newMiniredisClient(t))
	uid := uuid.New()

	s.Put("dev-1", uid, time.Minute)
	got, ok := s.Get("dev-1")
	require.True(t, ok)
	require.Equal(t, uid, got)

	_, ok = s.Get("unknown")
	require.False(t, ok)

	s.Delete("dev-1")
	_, ok = s.Get("dev-1")
	require.False(t, ok)

	// ttl<=0 не сохраняется (иначе ключ завис бы без TTL).
	s.Put("dev-2", uid, 0)
	_, ok = s.Get("dev-2")
	require.False(t, ok)
}

func TestRedisGitOAuthStateStore_NewConsumeOneShot(t *testing.T) {
	s := NewRedisGitOAuthStateStore(newMiniredisClient(t))
	uid := uuid.New()

	tok, err := s.New(GitOAuthState{
		UserID:          uid,
		Provider:        models.GitIntegrationProvider("gitlab"),
		Host:            "gitlab.example.com",
		ByoClientSecret: "super-secret",
		RedirectURI:     "https://app/cb",
	}, time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	st, err := s.Consume(tok)
	require.NoError(t, err)
	require.Equal(t, uid, st.UserID)
	require.Equal(t, "gitlab.example.com", st.Host)
	require.Equal(t, "super-secret", st.ByoClientSecret)
	require.Equal(t, "https://app/cb", st.RedirectURI)

	// One-shot: повторный Consume не находит запись.
	_, err = s.Consume(tok)
	require.ErrorIs(t, err, ErrGitOAuthStateNotFound)

	// Неизвестный и пустой токен.
	_, err = s.Consume("nope")
	require.ErrorIs(t, err, ErrGitOAuthStateNotFound)
	_, err = s.Consume("")
	require.ErrorIs(t, err, ErrGitOAuthStateNotFound)
}
