package ws

import (
	"context"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/pkg/secrets"
	"github.com/google/uuid"
)

// SandboxLogAdapter реализует sandbox.LogPublisher и публикует в ws.EventBus.
// Маскирование секретов выполняется здесь, до попадания в шину.
type SandboxLogAdapter struct {
	bus   events.EventBus
	scrub secrets.Scrubber
}

// NewSandboxLogAdapter создает новый адаптер.
func NewSandboxLogAdapter(bus events.EventBus, scrub secrets.Scrubber) *SandboxLogAdapter {
	return &SandboxLogAdapter{
		bus:   bus,
		scrub: scrub,
	}
}

// Publish маскирует секреты и отправляет событие в EventBus.
func (a *SandboxLogAdapter) Publish(ctx context.Context, projectID, taskID uuid.UUID, sandboxID string, seq int64, entry sandbox.LogEntry) error {
	// Маскируем секреты ДО публикации в шину
	masked := a.scrub.Scrub(entry.Line)

	ev := events.SandboxLogEmitted{
		ProjectID:  projectID,
		TaskID:     taskID,
		SandboxID:  sandboxID,
		Stderr:     entry.Stderr,
		Line:       masked,
		Seq:        seq,
		Truncated:  entry.Truncated,
		OccurredAt: entry.Timestamp,
	}

	// Публикуем в шину. Используем WithoutCancel, чтобы логи долетели даже если
	// контекст запроса отменен (но pumpCtx в раннере все равно следит за жизненным циклом).
	a.bus.Publish(context.WithoutCancel(ctx), ev)
	return nil
}
