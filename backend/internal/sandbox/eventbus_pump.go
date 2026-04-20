package sandbox

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// streamLogsToBus читает логи из tee-канала и публикует их через LogPublisher.
// Завершается когда закрывается stopCh (Cleanup) или ctx (SIGTERM/timeout),
// либо когда закрывается канал логов (контейнер завершился).
func (r *DockerSandboxRunner) streamLogsToBus(ctx context.Context, stopCh <-chan struct{}, projectID, taskID uuid.UUID, sandboxID string, logCh <-chan LogEntry) {
	slog.Debug("sandbox: starting log pump", "task_id", taskID, "sandbox_id", sandboxID)

	// Метрики: активная горутина
	incPumpActive()
	defer decPumpActive()

	var seq int64

	for {
		select {
		case <-ctx.Done():
			slog.Debug("sandbox: log pump context cancelled", "task_id", taskID, "sandbox_id", sandboxID)
			return
		case <-stopCh:
			slog.Debug("sandbox: log pump stopped by cleanup", "task_id", taskID, "sandbox_id", sandboxID)
			return
		case entry, ok := <-logCh:
			if !ok {
				slog.Debug("sandbox: log pump channel closed", "task_id", taskID, "sandbox_id", sandboxID)
				return
			}

			// Пропускаем терминальные ошибки (они логируются в runLogStream)
			if entry.Error != nil {
				incPumpErrors()
				continue
			}

			// Публикуем через адаптер
			seq++
			if err := r.publisher.Publish(ctx, projectID, taskID, sandboxID, seq, entry); err != nil {
				slog.Warn("sandbox: log pump publish error", "task_id", taskID, "sandbox_id", sandboxID, "err", err)
				incPumpErrors()
			} else {
				streamLabel := "stdout"
				if entry.Stderr {
					streamLabel = "stderr"
				}
				incEventsPublished(streamLabel)
			}
		}
	}
}

var (
	logEventsPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sandbox_log_events_published_total",
		Help: "Total number of log events published to the event bus",
	}, []string{"stream"})

	logPumpActiveGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sandbox_log_pump_active",
		Help: "Number of active log pump goroutines",
	})

	logPumpErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sandbox_log_pump_errors_total",
		Help: "Total number of errors encountered by the log pump",
	})
)

func incEventsPublished(stream string) { logEventsPublishedTotal.WithLabelValues(stream).Inc() }
func incPumpActive()      { logPumpActiveGauge.Inc() }
func decPumpActive()      { logPumpActiveGauge.Dec() }
func incPumpErrors()      { logPumpErrorsTotal.Inc() }
