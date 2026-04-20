package sandbox

import (
	"context"

	"github.com/google/uuid"
)

// LogPublisher публикует логи песочницы.
// Реализация находится вне sandbox — в ws-пакете или main.go.
// Sandbox не знает про ws.EventBus и ws.SandboxLogEmitted.
type LogPublisher interface {
	Publish(ctx context.Context, projectID, taskID uuid.UUID, sandboxID string, seq int64, entry LogEntry) error
}

// WithLogPublisher подключает произвольный LogPublisher.
// Если не задан — стрим логов наружу НЕ запускается.
func WithLogPublisher(p LogPublisher) RunnerOption {
	return func(r *DockerSandboxRunner) {
		r.publisher = p
	}
}
