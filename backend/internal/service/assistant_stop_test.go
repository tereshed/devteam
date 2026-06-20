package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestAssistantCancelRegistry проверяет in-memory реестр cancel-функций, на
// котором держится StopRun: track → cancelRun вызывает cancel и возвращает true,
// untrack → cancelRun возвращает false.
func TestAssistantCancelRegistry(t *testing.T) {
	s := &assistantService{runs: make(map[uuid.UUID]context.CancelFunc)}
	id := uuid.New()

	if s.cancelRun(id) {
		t.Fatal("cancelRun на незарегистрированную сессию должен вернуть false")
	}

	called := false
	s.trackRun(id, func() { called = true })

	if !s.cancelRun(id) {
		t.Fatal("cancelRun на зарегистрированную сессию должен вернуть true")
	}
	if !called {
		t.Fatal("cancel-функция должна быть вызвана")
	}

	s.untrackRun(id)
	if s.cancelRun(id) {
		t.Fatal("cancelRun после untrack должен вернуть false")
	}
}

// TestAssistantTrackRunReplacesStale: повторный track по тому же id отменяет
// старую запись (защита от утечки), новая остаётся активной.
func TestAssistantTrackRunReplacesStale(t *testing.T) {
	s := &assistantService{runs: make(map[uuid.UUID]context.CancelFunc)}
	id := uuid.New()

	staleCancelled := false
	s.trackRun(id, func() { staleCancelled = true })
	s.trackRun(id, func() {})

	if !staleCancelled {
		t.Fatal("повторный trackRun должен отменить устаревшую cancel-функцию")
	}
}
