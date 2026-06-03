package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLeaseStore — управляемый из теста LeaseStore: leader переключается вручную.
type fakeLeaseStore struct {
	mu           sync.Mutex
	leader       bool
	err          error
	releaseCount int
}

func (f *fakeLeaseStore) setLeader(b bool) {
	f.mu.Lock()
	f.leader = b
	f.mu.Unlock()
}

func (f *fakeLeaseStore) setErr(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

func (f *fakeLeaseStore) Acquire(_ context.Context, _, _ string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.leader, f.err
}

func (f *fakeLeaseStore) Release(_ context.Context, _, _ string) error {
	f.mu.Lock()
	f.releaseCount++
	f.mu.Unlock()
	return nil
}

func (f *fakeLeaseStore) releases() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.releaseCount
}

func newTestElector(store LeaseStore) *LeaderElector {
	return NewLeaderElectorWithConfig(store, "test", "inst-1", time.Second, 5*time.Millisecond, discardLogger())
}

// Полный жизненный цикл: получение лидерства → задача стартует; потеря → задача
// останавливается (ctx отменён); повторное получение → задача перезапускается;
// shutdown → лиз освобождается.
func TestLeaderElector_TaskLifecycleFollowsLeadership(t *testing.T) {
	store := &fakeLeaseStore{leader: true}
	e := newTestElector(store)

	var starts, exits atomic.Int32
	started := make(chan struct{}, 8)
	e.OnLeader("task", func(ctx context.Context) {
		starts.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		exits.Add(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()

	// 1. Стал лидером → задача запущена.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start after acquiring leadership")
	}
	assert.True(t, e.IsLeader())
	assert.Equal(t, int32(1), starts.Load())

	// 2. Потерял лидерство → ctx задачи отменён, задача вышла.
	store.setLeader(false)
	assert.Eventually(t, func() bool { return !e.IsLeader() }, 2*time.Second, 5*time.Millisecond)
	assert.Eventually(t, func() bool { return exits.Load() == 1 }, 2*time.Second, 5*time.Millisecond)

	// 3. Снова лидер → задача перезапущена.
	store.setLeader(true)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not restart after re-acquiring leadership")
	}
	assert.Equal(t, int32(2), starts.Load())

	// 4. Shutdown → Run завершился, лиз освобождён.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
	assert.GreaterOrEqual(t, store.releases(), 1)
}

// Follower не запускает задачи.
func TestLeaderElector_FollowerRunsNoTasks(t *testing.T) {
	store := &fakeLeaseStore{leader: false}
	e := newTestElector(store)

	var starts atomic.Int32
	e.OnLeader("task", func(ctx context.Context) {
		starts.Add(1)
		<-ctx.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	time.Sleep(50 * time.Millisecond)
	assert.False(t, e.IsLeader())
	assert.Equal(t, int32(0), starts.Load())
}

// Разовая ошибка Acquire не сбрасывает уже полученное лидерство (лиз мог ещё быть нашим).
func TestLeaderElector_TransientErrorDoesNotDropLeadership(t *testing.T) {
	store := &fakeLeaseStore{leader: true}
	e := newTestElector(store)
	e.OnLeader("task", func(ctx context.Context) { <-ctx.Done() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	require.Eventually(t, e.IsLeader, 2*time.Second, 5*time.Millisecond)

	store.setErr(assert.AnError) // следующие Acquire падают
	time.Sleep(50 * time.Millisecond)
	assert.True(t, e.IsLeader(), "transient acquire error must not drop leadership")
}
