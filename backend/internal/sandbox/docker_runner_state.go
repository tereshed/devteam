package sandbox

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// instanceState — in-memory состояние инстанса (ожидание Wait, таймер, стрим, пути уборки).
// Источник правды для контейнера остаётся Docker Engine; поля нужны для таймаутов и Cleanup.
type instanceState struct {
	mu sync.Mutex

	taskID           string
	containerID      string
	containerName    string
	hostTempDir      string
	networkID        string
	effectiveTimeout time.Duration
	createdAt        time.Time

	doneOnce sync.Once
	doneCh   chan struct{}

	finalStatus  *SandboxStatus
	finalWaitErr error

	waitLoopOnce sync.Once

	timedOut        atomic.Uint32
	stoppedByRunner atomic.Uint32
	cleaned         atomic.Bool

	streamMu     sync.Mutex
	streamCancel context.CancelFunc
	streamActive bool

	businessTimer *time.Timer
}

func newInstanceState(taskID string) *instanceState {
	return &instanceState{
		taskID:    taskID,
		doneCh:    make(chan struct{}),
		createdAt: time.Now(),
	}
}

func (s *instanceState) closeDone() {
	s.doneOnce.Do(func() {
		close(s.doneCh)
	})
}

func (s *instanceState) stopBusinessTimer() {
	s.mu.Lock()
	t := s.businessTimer
	s.businessTimer = nil
	s.mu.Unlock()
	if t != nil {
		t.Stop()
	}
}
