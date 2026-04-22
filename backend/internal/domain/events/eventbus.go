package events

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// DomainEvent — маркер интерфейс. Реализуется конкретными событиями ниже.
type DomainEvent interface {
	domainEvent() // приватный метод-маркер (нельзя реализовать снаружи пакета)
	GetProjectID() uuid.UUID
	GetTraceID() string
}

// TaskStatusChanged — переход статуса задачи (см. 3.6).
type TaskStatusChanged struct {
	ProjectID       uuid.UUID
	TaskID          uuid.UUID
	ParentTaskID    *uuid.UUID
	Previous        string // task.StatusType
	Current         string
	AssignedAgentID *uuid.UUID
	AgentRole       string
	ErrorMessage    string // только при Current == failed
	OccurredAt      time.Time
	TraceID         string
}

func (e TaskStatusChanged) domainEvent()          {}
func (e TaskStatusChanged) GetProjectID() uuid.UUID { return e.ProjectID }
func (e TaskStatusChanged) GetTraceID() string      { return e.TraceID }

// TaskMessageCreated — новое сообщение в логе задачи (см. 3.2 / main.mdc §2.5).
type TaskMessageCreated struct {
	ProjectID   uuid.UUID
	TaskID      uuid.UUID
	MessageID   uuid.UUID
	SenderType  string
	SenderID    uuid.UUID
	SenderRole  string
	MessageType string
	Content     string
	Metadata    map[string]any // raw, фильтрация — в подписчике
	OccurredAt  time.Time
	TraceID     string
}

func (e TaskMessageCreated) domainEvent()          {}
func (e TaskMessageCreated) GetProjectID() uuid.UUID { return e.ProjectID }
func (e TaskMessageCreated) GetTraceID() string      { return e.TraceID }

// SandboxLogEmitted — одна строка лога из контейнера (см. 5.x).
type SandboxLogEmitted struct {
	ProjectID  uuid.UUID
	TaskID     uuid.UUID
	SandboxID  string
	Stderr     bool
	Line       string // raw, маскирование — в подписчике
	Seq        int64
	Truncated  bool
	OccurredAt time.Time
	TraceID    string
}

func (e SandboxLogEmitted) domainEvent()          {}
func (e SandboxLogEmitted) GetProjectID() uuid.UUID { return e.ProjectID }
func (e SandboxLogEmitted) GetTraceID() string      { return e.TraceID }

// PipelineErrored — ошибка пайплайна / executor'а / sandbox'а (см. 6.x).
type PipelineErrored struct {
	ProjectID  uuid.UUID
	TaskID     *uuid.UUID
	Code       string // из enum 7.3 error.code
	Message    string // safe-string из константного перечня
	OccurredAt time.Time
	TraceID    string
}

func (e PipelineErrored) domainEvent()          {}
func (e PipelineErrored) GetProjectID() uuid.UUID { return e.ProjectID }
func (e PipelineErrored) GetTraceID() string      { return e.TraceID }

// ProjectDeleted — удаление проекта (см. 9.1).
type ProjectDeleted struct {
	ProjectID  uuid.UUID
	OccurredAt time.Time
	TraceID    string
}

func (e ProjectDeleted) domainEvent()          {}
func (e ProjectDeleted) GetProjectID() uuid.UUID { return e.ProjectID }
func (e ProjectDeleted) GetTraceID() string      { return e.TraceID }

// UserDeleted — удаление пользователя (см. 9.4).
type UserDeleted struct {
	UserID     uuid.UUID
	OccurredAt time.Time
	TraceID    string
}

func (e UserDeleted) domainEvent()          {}
func (e UserDeleted) GetProjectID() uuid.UUID { return uuid.Nil }
func (e UserDeleted) GetTraceID() string      { return e.TraceID }

func (e UserDeleted) isGlobal() bool { return true }

// ConversationDeleted — удаление чата (см. 9.4).
type ConversationDeleted struct {
	ProjectID      uuid.UUID
	ConversationID uuid.UUID
	OccurredAt     time.Time
	TraceID        string
}

func (e ConversationDeleted) domainEvent()          {}
func (e ConversationDeleted) GetProjectID() uuid.UUID { return e.ProjectID }
func (e ConversationDeleted) GetTraceID() string      { return e.TraceID }

// ConversationMessageCreated — новое сообщение в чате (см. 9.4).
type ConversationMessageCreated struct {
	ProjectID      uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	Role           string
	Content        string
	OccurredAt     time.Time
	TraceID        string
}

func (e ConversationMessageCreated) domainEvent()          {}
func (e ConversationMessageCreated) GetProjectID() uuid.UUID { return e.ProjectID }
func (e ConversationMessageCreated) GetTraceID() string      { return e.TraceID }

// ConversationMessageDeleted — удаление сообщения (см. 9.4).
type ConversationMessageDeleted struct {
	ProjectID      uuid.UUID
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	OccurredAt     time.Time
	TraceID        string
}

func (e ConversationMessageDeleted) domainEvent()          {}
func (e ConversationMessageDeleted) GetProjectID() uuid.UUID { return e.ProjectID }
func (e ConversationMessageDeleted) GetTraceID() string      { return e.TraceID }

// EventBus — публикация и подписка на доменные события в одном процессе.
type EventBus interface {
	Publish(ctx context.Context, ev DomainEvent)
	Subscribe(name string, buffer int) (<-chan DomainEvent, func())
	Close()
}

type subscription struct {
	ch   chan DomainEvent
	name string
}

type inMemoryBus struct {
	subs    atomic.Pointer[[]*subscription]
	mu      sync.Mutex // только для Subscribe/Unsubscribe/Close
	closed  atomic.Bool
	metrics Metrics
	log     *slog.Logger

	// Rate limiting для логов дропа событий
	lastDropLog atomic.Pointer[time.Time]
}

// Metrics — интерфейс для сбора метрик шины событий.
type Metrics interface {
	IncPublished(eventType string)
	IncDropped(subscriber string, eventType string)
	SetSubscribers(count int)
}

type noopMetrics struct{}

func (m noopMetrics) IncPublished(eventType string)             {}
func (m noopMetrics) IncDropped(subscriber string, eventType string) {}
func (m noopMetrics) SetSubscribers(count int)                  {}

// NewInMemoryBus создает реализацию EventBus для одного инстанса.
func NewInMemoryBus(metrics Metrics, log *slog.Logger) EventBus {
	if metrics == nil {
		metrics = noopMetrics{}
	}
	if log == nil {
		log = slog.Default()
	}
	bus := &inMemoryBus{
		metrics: metrics,
		log:     log,
	}
	empty := make([]*subscription, 0)
	bus.subs.Store(&empty)
	
	now := time.Now().Add(-time.Hour)
	bus.lastDropLog.Store(&now)
	
	return bus
}

func (b *inMemoryBus) Publish(ctx context.Context, ev DomainEvent) {
	if ev == nil {
		b.log.Warn("eventbus: attempt to publish nil event")
		return
	}
	
	// Check if event is global (e.g. UserDeleted)
	isGlobal := false
	if g, ok := ev.(interface{ isGlobal() bool }); ok {
		isGlobal = g.isGlobal()
	}

	if !isGlobal && ev.GetProjectID() == uuid.Nil {
		b.log.Warn("eventbus: event with nil ProjectID dropped", "type", getEventTypeName(ev))
		return
	}

	if b.closed.Load() {
		return
	}

	subsPtr := b.subs.Load()
	if subsPtr == nil {
		return
	}

	eventType := getEventTypeName(ev)
	b.metrics.IncPublished(eventType)

	for _, s := range *subsPtr {
		select {
		case s.ch <- ev:
		case <-ctx.Done():
			return
		default:
			// slow subscriber → drop
			b.metrics.IncDropped(s.name, eventType)
			b.logRateLimitedDrop(s.name, eventType)
		}
	}
}

func (b *inMemoryBus) logRateLimitedDrop(subscriber, eventType string) {
	const logInterval = 5 * time.Second
	now := time.Now()
	last := b.lastDropLog.Load()
	
	if now.Sub(*last) > logInterval {
		if b.lastDropLog.CompareAndSwap(last, &now) {
			b.log.Warn("eventbus: event dropped due to slow subscriber",
				"subscriber", subscriber,
				"event_type", eventType,
				"interval", logInterval)
		}
	}
}

func (b *inMemoryBus) Subscribe(name string, buffer int) (<-chan DomainEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed.Load() {
		ch := make(chan DomainEvent)
		close(ch)
		return ch, func() {}
	}

	if name == "" {
		name = "unnamed_subscriber"
	}

	sub := &subscription{
		ch:   make(chan DomainEvent, buffer),
		name: name,
	}

	oldSubs := b.subs.Load()
	newSubs := make([]*subscription, len(*oldSubs)+1)
	copy(newSubs, *oldSubs)
	newSubs[len(*oldSubs)] = sub

	b.subs.Store(&newSubs)
	b.metrics.SetSubscribers(len(newSubs))

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.unsubscribe(sub)
		})
	}

	return sub.ch, unsubscribe
}

func (b *inMemoryBus) unsubscribe(sub *subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldSubs := b.subs.Load()
	newSubs := make([]*subscription, 0, len(*oldSubs))
	for _, s := range *oldSubs {
		if s != sub {
			newSubs = append(newSubs, s)
		}
	}

	b.subs.Store(&newSubs)
	b.metrics.SetSubscribers(len(newSubs))
}

func (b *inMemoryBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed.Swap(true) {
		return
	}

	subsPtr := b.subs.Load()
	for _, s := range *subsPtr {
		close(s.ch)
	}

	empty := make([]*subscription, 0)
	b.subs.Store(&empty)
	b.metrics.SetSubscribers(0)
}

func getEventTypeName(ev DomainEvent) string {
	switch ev.(type) {
	case TaskStatusChanged:
		return "task_status_changed"
	case TaskMessageCreated:
		return "task_message_created"
	case SandboxLogEmitted:
		return "sandbox_log_emitted"
	case PipelineErrored:
		return "pipeline_errored"
	case ProjectDeleted:
		return "project_deleted"
	case UserDeleted:
		return "user_deleted"
	case ConversationDeleted:
		return "conversation_deleted"
	case ConversationMessageCreated:
		return "conversation_message_created"
	case ConversationMessageDeleted:
		return "conversation_message_deleted"
	default:
		return "unknown"
	}
}
