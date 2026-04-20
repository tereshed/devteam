package sandbox

import (
	"context"
	"log/slog"
	"time"
)

// scheduleSandboxBusinessDeadline планирует жёсткий дедлайн после успешного ContainerStart+sanity (5.8).
// Таймер и ссылка на него живут под st.mu, чтобы Stop мог атомарно отменить его вместе с флагом userStopIntent.
func scheduleSandboxBusinessDeadline(st *instanceState, stopper ContainerStopper, containerID string, d time.Duration, taskID string) {
	st.mu.Lock()
	if st.businessTimer != nil {
		st.businessTimer.Stop()
	}
	st.businessTimer = time.AfterFunc(d, func() {
		onBusinessDeadline(st, stopper, containerID, taskID)
	})
	st.mu.Unlock()
}

// onBusinessDeadline — колбэк time.AfterFunc: внешний timer.Stop() не гарантирует, что колбэк не стартовал,
// поэтому первым делом берём st.mu и проверяем связный стейт (согласование с Stop, wait и Cleanup) — иначе TOCTOU.
//
// cancelWait не вызываем до ForceStop: иначе ContainerWait вернёт context.Canceled, и wait-loop выйдет
// без Inspect/артефактов. Отмена wait — только если ForceStop не смог пробить зависший демон (ниже).
func onBusinessDeadline(st *instanceState, stopper ContainerStopper, containerID, taskID string) {
	st.mu.Lock()
	if st.cleaned || st.initCancelRequested || st.userStopIntent || st.waitCompleted {
		// cleaned — после Cleanup, без Docker в колбэке
		st.mu.Unlock()
		return
	}
	st.businessTimeoutIntent = true
	st.mu.Unlock()

	ctx, cancel := detachTimeout(context.Background(), dockerOpDetachTimeout)
	defer cancel()
	if err := stopper.ForceStop(ctx, containerID, 0, "timeout", taskID); err != nil {
		slog.Error("sandbox: business timeout force-stop failed",
			"task_id", taskID, "sandbox_id", containerID, "reason", "timeout", "err", err)
		st.mu.Lock()
		st.cancelContainerWaitLocked()
		st.mu.Unlock()
	}
}

// applyUserStopIntent отмечает ручную остановку и гасит бизнес-таймер под st.mu.
// cancelWait намеренно не трогаем до ForceStop в Stop — см. onBusinessDeadline.
// alreadyStopped: повторный Stop — без повторного Docker RPC.
func (st *instanceState) applyUserStopIntent() (containerID string, graceSec int, alreadyStopped bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.userStopIntent {
		return st.containerID, stopGraceSecondsFromDuration(st.stopGracePeriod), true
	}
	st.userStopIntent = true
	if st.businessTimer != nil {
		st.businessTimer.Stop()
		st.businessTimer = nil
	}
	return st.containerID, stopGraceSecondsFromDuration(st.stopGracePeriod), false
}

// markWaitCompleted фиксирует, что ContainerWait уже вернулся — колбэк таймера не должен классифицировать инстанс как timed_out.
func (st *instanceState) markWaitCompleted() {
	st.mu.Lock()
	st.waitCompleted = true
	st.mu.Unlock()
}

// errIfInitCancelled — проверка между долгими шагами RunTask (анти-зомби при StopTask).
func (st *instanceState) errIfInitCancelled() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.initCancelRequested {
		return ErrSandboxInitCancelled
	}
	return nil
}

// lifecycleInfraStrictLocked — только под st.mu: таймаут/ручной стоп влияют на merge артефактов (5.7).
func (st *instanceState) lifecycleInfraStrictLocked() bool {
	return st.businessTimeoutIntent || st.userStopIntent
}

// setCleaned — Cleanup: помечает уборку и отпускает ContainerWait, чтобы не оставлять горутину wait.
func (st *instanceState) setCleaned() {
	st.mu.Lock()
	st.cleaned = true
	st.cancelContainerWaitLocked()
	if st.onCleanup != nil {
		st.onCleanup()
		st.onCleanup = nil
	}
	st.mu.Unlock()
}
