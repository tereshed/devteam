package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// redis_notifier.go — Sprint 17 / Orchestration v2 — low-latency wakeup поверх Redis Pub/Sub.
//
// Назначение: Yugabyte не поддерживает LISTEN/NOTIFY, и polling в одиночку даёт latency
// 500-1000ms. Redis Pub/Sub добавляет ms-уровень wakeup для:
//
//	1. Воркеров пула очередей — "появилась работа kind=step_req/agent_job".
//	2. Запущенных Agent Worker'ов — "пользователь отменил задачу <id>, ctx надо завершить".
//
// При этом polling остаётся как FALLBACK на случай разрыва Redis-соединения —
// никаких потерь работы.

// RedisChannelTaskEvents — общий канал для wakeup воркеров очереди.
// Payload — kind ("step_req" | "agent_job"), чтобы пулы реагировали избирательно.
const RedisChannelTaskEvents = "devteam:task_events"

// RedisChannelTaskCancel формирует имя канала для отмены конкретной задачи.
// Подписка в Agent Worker'е при старте каждого agent_job; публикация —
// в HTTP-хендлере /tasks/:id/cancel.
func RedisChannelTaskCancel(taskID uuid.UUID) string {
	return "devteam:task_cancel:" + taskID.String()
}

// ErrRedisNotifierClosed — попытка использовать закрытый notifier.
var ErrRedisNotifierClosed = errors.New("redis notifier is closed")

// RedisNotifier — тонкая обёртка вокруг go-redis для двух операций.
//
// Все методы безопасны для конкурентного использования (go-redis сам по себе
// thread-safe).
type RedisNotifier struct {
	client *redis.Client
}

// NewRedisNotifier — конструктор. Принимает уже сконфигурированный клиент
// (создаётся в cmd/api/main.go из cfg.RedisURL). Это позволяет переиспользовать
// один пул соединений на весь процесс.
func NewRedisNotifier(client *redis.Client) *RedisNotifier {
	return &RedisNotifier{client: client}
}

// NotifyTaskEvent публикует wakeup для воркеров очереди.
// payload — обычно строка kind ("step_req" / "agent_job"); воркеры неподходящего
// типа просто игнорируют. Это позволяет иметь один shared канал на всё приложение.
//
// Ошибки публикации НЕ должны валить вызывающий код (worker всё равно подберёт
// событие на следующем polling-цикле, max 500-1000ms задержки). Caller логирует ошибку,
// но не возвращает её выше.
func (n *RedisNotifier) NotifyTaskEvent(ctx context.Context, kind string) error {
	if n == nil || n.client == nil {
		return ErrRedisNotifierClosed
	}
	if err := n.client.Publish(ctx, RedisChannelTaskEvents, kind).Err(); err != nil {
		return fmt.Errorf("failed to publish task event notification: %w", err)
	}
	return nil
}

// NotifyTaskCancel публикует сигнал отмены для конкретной задачи.
// В Agent Worker'ах, подписанных на RedisChannelTaskCancel(taskID),
// сигнал триггерит ctx.Cancel() для запущенного sandbox/llm-агента.
//
// Контракт: вызывается ПОСЛЕ COMMIT транзакции UPDATE tasks SET cancel_requested=true,
// иначе Worker может подхватить отмену до того как изменение видно через polling-fallback
// (race — если Worker внезапно потеряет Redis-соединение в этот момент).
func (n *RedisNotifier) NotifyTaskCancel(ctx context.Context, taskID uuid.UUID) error {
	if n == nil || n.client == nil {
		return ErrRedisNotifierClosed
	}
	channel := RedisChannelTaskCancel(taskID)
	if err := n.client.Publish(ctx, channel, "cancel").Err(); err != nil {
		return fmt.Errorf("failed to publish cancel notification for task %s: %w", taskID, err)
	}
	return nil
}

// SubscribeTaskEvents подписывается на shared-канал. Возвращает *redis.PubSub —
// caller отвечает за вызов .Close() и чтение из .Channel().
//
// Используется воркерами пула очередей при старте процесса:
//
//	pubsub := notifier.SubscribeTaskEvents(ctx)
//	defer pubsub.Close()
//	for {
//	    select {
//	    case msg := <-pubsub.Channel():
//	        // wakeup — пробуем claim
//	    case <-time.After(500*time.Millisecond):
//	        // polling fallback
//	    case <-ctx.Done():
//	        return
//	    }
//	    repo.ClaimNext(...)
//	}
func (n *RedisNotifier) SubscribeTaskEvents(ctx context.Context) *redis.PubSub {
	return n.client.Subscribe(ctx, RedisChannelTaskEvents)
}

// SubscribeTaskCancel подписывается на канал отмены КОНКРЕТНОЙ задачи.
//
// ВАЖНО: для race-free отмены Agent Worker ОБЯЗАН после Subscribe сделать SELECT
// cancel_requested FROM tasks — это ловит NOTIFY, отправленный ДО подписки
// (стандартный pattern для distributed pub/sub).
//
// Использование:
//
//	pubsub := notifier.SubscribeTaskCancel(ctx, taskID)
//	defer pubsub.Close()
//	// 1. Проверяем текущий стейт ПОСЛЕ подписки.
//	var cancelled bool
//	db.QueryRow("SELECT cancel_requested FROM tasks WHERE id=?", taskID).Scan(&cancelled)
//	if cancelled { return /* job aborted */ }
//	// 2. Запускаем работу с ctx, отменяемым из pubsub.Channel().
func (n *RedisNotifier) SubscribeTaskCancel(ctx context.Context, taskID uuid.UUID) *redis.PubSub {
	return n.client.Subscribe(ctx, RedisChannelTaskCancel(taskID))
}

// Ping — health-check для readiness-эндпоинта.
func (n *RedisNotifier) Ping(ctx context.Context) error {
	if n == nil || n.client == nil {
		return ErrRedisNotifierClosed
	}
	return n.client.Ping(ctx).Err()
}
