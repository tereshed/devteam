package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
)

// step_worker.go — Sprint 17 / Orchestration v2 — пул воркеров типа step_req.
//
// Воркер забирает task_event(kind=step_req) → вызывает Orchestrator.Step → помечает
// событие Complete (на успехе) или Fail (на ошибке инфра).
//
// Wakeup:
//   - Polling каждые PollInterval (~500ms) через ClaimNext + SKIP LOCKED.
//   - Redis Pub/Sub на канале devteam:task_events (low-latency, если notifier есть).
//
// Yugabyte НЕ поддерживает LISTEN/NOTIFY, поэтому polling — основной механизм,
// Redis — оптимизация для latency.

// StepWorkerConfig — настройки одного воркера. Несколько воркеров в пуле
// разделяют один и тот же конфиг, но имеют уникальные WorkerID (для observability).
type StepWorkerConfig struct {
	WorkerID     string
	PollInterval time.Duration // default 500ms
}

// DefaultStepWorkerConfig — разумные дефолты.
func DefaultStepWorkerConfig() StepWorkerConfig {
	return StepWorkerConfig{
		WorkerID:     "step-worker-default",
		PollInterval: 500 * time.Millisecond,
	}
}

// StepWorker — один воркер пула step_req.
type StepWorker struct {
	eventRepo    repository.TaskEventRepository
	orchestrator *Orchestrator
	notifier     *RedisNotifier // опционально — может быть nil
	logger       *slog.Logger
	cfg          StepWorkerConfig
}

// NewStepWorker — конструктор.
func NewStepWorker(
	eventRepo repository.TaskEventRepository,
	orchestrator *Orchestrator,
	notifier *RedisNotifier,
	logger *slog.Logger,
	cfg StepWorkerConfig,
) *StepWorker {
	if logger == nil {
		logger = logging.NopLogger()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "step-worker-default"
	}
	return &StepWorker{
		eventRepo: eventRepo, orchestrator: orchestrator,
		notifier: notifier, logger: logger, cfg: cfg,
	}
}

// Run блокирует до отмены ctx. Безопасно вызывать N раз параллельно (несколько
// goroutine'ов одного процесса) с одним и тем же WorkerID, либо запускать
// несколько процессов с разными WorkerID — SKIP LOCKED разводит конкурентов.
func (w *StepWorker) Run(ctx context.Context) error {
	// Подписка на Redis (если есть). Используем как wakeup-источник;
	// polling — fallback.
	var wakeupCh <-chan struct{}
	if w.notifier != nil {
		pubsub := w.notifier.SubscribeTaskEvents(ctx)
		defer pubsub.Close()
		// Конвертируем *redis.Message → struct{} — нам не нужно содержимое сообщения,
		// достаточно факта "что-то изменилось, попробуй забрать работу".
		ch := make(chan struct{}, 64)
		go func() {
			defer close(ch)
			for msg := range pubsub.Channel() {
				// Фильтруем: нас интересует только step_req (другие kind игнорируем —
				// общий канал для всех воркеров пула).
				if msg.Payload != string(models.TaskEventKindStepReq) {
					continue
				}
				select {
				case ch <- struct{}{}:
				default:
					// Канал переполнен — это OK, следующий polling-tick подберёт.
				}
			}
		}()
		wakeupCh = ch
	}

	w.logger.InfoContext(ctx, "step worker started",
		"worker_id", w.cfg.WorkerID, "poll_interval", w.cfg.PollInterval)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		// Пытаемся обработать всё что есть в очереди прямо сейчас.
		w.drainQueue(ctx)

		// Ждём следующего сигнала.
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "step worker stopping", "worker_id", w.cfg.WorkerID)
			return nil
		case <-ticker.C:
			// polling tick
		case <-wakeupCh:
			// redis-сигнал; tickers ничего не теряют.
		}
	}
}

// drainQueue — забирает события пока есть; выходит при ErrNoTaskEventAvailable или ошибке ctx.
func (w *StepWorker) drainQueue(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		ev, err := w.eventRepo.ClaimNext(ctx, models.TaskEventKindStepReq, w.cfg.WorkerID)
		if err != nil {
			if errors.Is(err, repository.ErrNoTaskEventAvailable) {
				return
			}
			w.logger.ErrorContext(ctx, "claim next step_req failed",
				"worker_id", w.cfg.WorkerID, "error", err.Error())
			return
		}
		w.processOne(ctx, ev)
	}
}

// processOne обрабатывает один claimed event.
func (w *StepWorker) processOne(ctx context.Context, ev *models.TaskEvent) {
	if err := w.orchestrator.Step(ctx, ev.TaskID); err != nil {
		w.logger.ErrorContext(ctx, "orchestrator step failed",
			"worker_id", w.cfg.WorkerID,
			"task_event_id", ev.ID,
			"task_id", ev.TaskID,
			"attempt", ev.Attempts+1,
			"error", err.Error(),
		)
		// Fail с exponential backoff: 1s, 2s, 4s, ...
		backoff := time.Duration(1<<ev.Attempts) * time.Second
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
		if ferr := w.eventRepo.Fail(ctx, ev.ID, truncate(err.Error(), 512), backoff); ferr != nil {
			w.logger.ErrorContext(ctx, "mark step_req as failed",
				"task_event_id", ev.ID, "error", ferr.Error())
		}
		return
	}

	if err := w.eventRepo.Complete(ctx, ev.ID); err != nil {
		w.logger.ErrorContext(ctx, "complete step_req failed",
			"task_event_id", ev.ID, "error", err.Error())
	}
}

