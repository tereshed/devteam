package sandbox

import (
	"context"
	"sync"
	"time"
)

// instanceState — in-memory состояние инстанса (ожидание Wait, таймер, стрим, пути уборки).
//
// Порядок блокировок (5.8): сначала st.mu (lifecycle, таймер, намерения), затем streamMu для StreamLogs.
// Никогда не захватывать streamMu удерживая st.mu в обратном порядке — дедлок со стримом.
//
// Источник правды для контейнера остаётся Docker Engine; поля нужны для таймаутов и Cleanup.
type instanceState struct {
	mu sync.Mutex

	taskID           string
	containerID      string
	containerName    string
	hostTempDir      string
	networkID        string
	effectiveTimeout time.Duration
	stopGracePeriod  time.Duration
	createdAt        time.Time

	doneOnce sync.Once
	doneCh   chan struct{}

	finalStatus  *SandboxStatus
	finalWaitErr error

	waitLoopOnce sync.Once

	// Намерения и фазы остановки — только под st.mu (без разрозненных атомиков, 5.8).
	initCancelRequested     bool
	userStopIntent          bool
	businessTimeoutIntent   bool
	waitCompleted           bool
	cleaned                 bool
	businessTimer           *time.Timer
	cancelWait              context.CancelFunc

	// onCleanup вызывается при Cleanup (7.6).
	onCleanup func()

	streamMu     sync.Mutex
	streamCancel context.CancelFunc
	streamActive bool
	streamCh     chan LogEntry // мастер-канал для tee (7.6)
	externalCh   <-chan LogEntry // второе плечо tee для StreamLogs (7.6)
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

func (s *instanceState) cancelContainerWaitLocked() {
	if s.cancelWait != nil {
		s.cancelWait()
		s.cancelWait = nil
	}
}
