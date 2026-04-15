package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockContainerStopper struct {
	mock.Mock
}

func (m *mockContainerStopper) ForceStop(ctx context.Context, containerID string, graceSeconds int, reason string, taskID string) error {
	args := m.Called(ctx, containerID, graceSeconds, reason, taskID)
	return args.Error(0)
}

var _ ContainerStopper = (*mockContainerStopper)(nil)

func TestOnBusinessDeadline_skipsWhenUserStopIntent(t *testing.T) {
	st := newInstanceState("task-1")
	st.userStopIntent = true
	m := new(mockContainerStopper)
	onBusinessDeadline(st, m, strings.Repeat("a", 64), "task-1")
	m.AssertNotCalled(t, "ForceStop")
}

func TestOnBusinessDeadline_skipsWhenWaitCompleted(t *testing.T) {
	st := newInstanceState("task-1")
	st.waitCompleted = true
	m := new(mockContainerStopper)
	onBusinessDeadline(st, m, strings.Repeat("b", 64), "task-1")
	m.AssertNotCalled(t, "ForceStop")
}

func TestOnBusinessDeadline_callsForceStop(t *testing.T) {
	st := newInstanceState("task-1")
	m := new(mockContainerStopper)
	m.On("ForceStop", mock.Anything, strings.Repeat("c", 64), 0, "timeout", "task-1").Return(nil).Once()
	onBusinessDeadline(st, m, strings.Repeat("c", 64), "task-1")
	m.AssertExpectations(t)
	require.True(t, func() bool {
		st.mu.Lock()
		defer st.mu.Unlock()
		return st.businessTimeoutIntent
	}())
}

func TestApplyUserStopIntent_idempotent(t *testing.T) {
	st := newInstanceState("task-1")
	st.containerID = strings.Repeat("f", 64)
	st.stopGracePeriod = 8 * time.Second
	_, g1, already := st.applyUserStopIntent()
	require.False(t, already)
	require.Equal(t, 8, g1)
	_, g2, already2 := st.applyUserStopIntent()
	require.True(t, already2)
	require.Equal(t, 8, g2)
}

func TestScheduleBusinessDeadline_runsAfterDelay(t *testing.T) {
	st := newInstanceState("task-2")
	m := new(mockContainerStopper)
	m.On("ForceStop", mock.Anything, strings.Repeat("9", 64), 0, "timeout", "task-2").Return(nil).Once()
	scheduleSandboxBusinessDeadline(st, m, strings.Repeat("9", 64), 25*time.Millisecond, "task-2")
	t.Cleanup(func() { st.stopBusinessTimer() })
	require.Eventually(t, func() bool {
		st.mu.Lock()
		defer st.mu.Unlock()
		return st.businessTimeoutIntent
	}, time.Second, 10*time.Millisecond)
	m.AssertExpectations(t)
}
