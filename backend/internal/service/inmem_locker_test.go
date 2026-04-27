package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryLocker_AcquireRelease(t *testing.T) {
	l := NewInMemoryLocker()
	id, err := l.Lock(context.Background(), "k", 100*time.Millisecond)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	require.NoError(t, l.Unlock(context.Background(), "k", id))
}

func TestInMemoryLocker_AcquireWhileHeldFails(t *testing.T) {
	l := NewInMemoryLocker()
	id, err := l.Lock(context.Background(), "k", time.Hour)
	require.NoError(t, err)

	_, err = l.Lock(context.Background(), "k", time.Hour)
	assert.ErrorIs(t, err, ErrLockHeld)

	require.NoError(t, l.Unlock(context.Background(), "k", id))
}

func TestInMemoryLocker_ExpiredLockCanBeReacquired(t *testing.T) {
	l := NewInMemoryLocker()
	_, err := l.Lock(context.Background(), "k", 5*time.Millisecond)
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)

	id2, err := l.Lock(context.Background(), "k", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, id2)
}

func TestInMemoryLocker_UnlockWrongOwner(t *testing.T) {
	l := NewInMemoryLocker()
	_, err := l.Lock(context.Background(), "k", time.Hour)
	require.NoError(t, err)

	err = l.Unlock(context.Background(), "k", "stranger")
	assert.ErrorIs(t, err, ErrLockNotOwned)
}

func TestInMemoryLocker_RefreshExtendsTTL(t *testing.T) {
	l := NewInMemoryLocker()
	id, err := l.Lock(context.Background(), "k", 30*time.Millisecond)
	require.NoError(t, err)

	time.Sleep(15 * time.Millisecond)
	require.NoError(t, l.Refresh(context.Background(), "k", id, time.Hour))

	// Через 50ms лок не должен истечь, т.к. был продлён.
	time.Sleep(50 * time.Millisecond)
	_, err = l.Lock(context.Background(), "k", time.Hour)
	assert.ErrorIs(t, err, ErrLockHeld)
}

func TestInMemoryLocker_ConcurrentAcquire(t *testing.T) {
	l := NewInMemoryLocker()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	winners := make(chan string, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			id, err := l.Lock(context.Background(), "k", time.Hour)
			if err == nil {
				winners <- id
			}
		}()
	}
	wg.Wait()
	close(winners)

	count := 0
	for range winners {
		count++
	}
	assert.Equal(t, 1, count, "ровно один захватчик должен получить лок")
}
