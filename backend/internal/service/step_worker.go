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
	WorkerID string
	// PollInterval — минимальная пауза между опросами очереди (нижняя граница
	// адаптивного backoff'а). Так часто воркер опрашивает БД сразу после активности.
	PollInterval time.Duration // default 500ms
	// MaxPollInterval — верхняя граница backoff'а: на простаивающей очереди пауза
	// удваивается от PollInterval до этого значения. Без backoff'а N воркеров в
	// полку долбят Yugabyte распределёнными SELECT ... FOR UPDATE SKIP LOCKED даже
	// когда работы нет — это главный источник idle-CPU.
	MaxPollInterval time.Duration // default 5s
}

// DefaultStepWorkerConfig — разумные дефолты.
func DefaultStepWorkerConfig() StepWorkerConfig {
	return StepWorkerConfig{
		WorkerID:        "step-worker-default",
		PollInterval:    500 * time.Millisecond,
		MaxPollInterval: 5 * time.Second,
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
	if cfg.MaxPollInterval <= 0 {
		cfg.MaxPollInterval = 5 * time.Second
	}
	if cfg.MaxPollInterval < cfg.PollInterval {
		cfg.MaxPollInterval = cfg.PollInterval
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
		"worker_id", w.cfg.WorkerID,
		"poll_interval", w.cfg.PollInterval, "max_poll_interval", w.cfg.MaxPollInterval)

	// Адаптивный backoff: delay стартует с PollInterval, удваивается на каждом
	// пустом опросе до MaxPollInterval и сбрасывается к минимуму, как только
	// появилась работа (или пришёл Redis-wakeup). Так под нагрузкой латентность
	// низкая, а на простое пул почти не трогает БД.
	delay := w.cfg.PollInterval

	for {
		// Пытаемся обработать всё что есть в очереди прямо сейчас.
		worked := w.drainQueue(ctx)
		if worked {
			delay = w.cfg.PollInterval
		} else {
			delay *= 2
			if delay > w.cfg.MaxPollInterval {
				delay = w.cfg.MaxPollInterval
			}
		}

		// Ждём следующего сигнала. jitterDuration расфазирует пул, чтобы N
		// воркеров не били Yugabyte синхронным залпом.
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "step worker stopping", "worker_id", w.cfg.WorkerID)
			return nil
		case <-time.After(jitterDuration(delay)):
			// polling tick
		case <-wakeupCh:
			// redis-сигнал: сбрасываем backoff чтобы мгновенно отреагировать.
			delay = w.cfg.PollInterval
		}
	}
}

// drainQueue — забирает события пока есть; выходит при ErrNoTaskEventAvailable
// или ошибке ctx. Возвращает true, если обработал хотя бы одно событие (сигнал
// для сброса backoff'а в Run).
func (w *StepWorker) drainQueue(ctx context.Context) bool {
	worked := false
	for {
		if ctx.Err() != nil {
			return worked
		}
		ev, err := w.eventRepo.ClaimNext(ctx, models.TaskEventKindStepReq, w.cfg.WorkerID)
		if err != nil {
			if errors.Is(err, repository.ErrNoTaskEventAvailable) {
				return worked
			}
			w.logger.ErrorContext(ctx, "claim next step_req failed",
				"worker_id", w.cfg.WorkerID, "error", err.Error())
			return worked
		}
		w.processOne(ctx, ev)
		worked = true
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

