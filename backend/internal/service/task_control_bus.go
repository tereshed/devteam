package service

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/models"
)

// UserTaskControlType команда управления задачей (шина для оркестратора и UI).
type UserTaskControlType string

const (
	UserTaskControlPause   UserTaskControlType = "pause"
	UserTaskControlCancel  UserTaskControlType = "cancel"
	UserTaskControlResume  UserTaskControlType = "resume"
	UserTaskControlCorrect UserTaskControlType = "correct"
)

// UserTaskControlCommand событие от пользователя (после успешной смены в TaskService или из внешнего EventBus).
type UserTaskControlCommand struct {
	Kind     UserTaskControlType
	TaskID   uuid.UUID
	UserID   uuid.UUID
	UserRole models.UserRole
}

// TaskControlOutcome результат обработки команды оркестратором (для UI / WebSocket).
type TaskControlOutcome struct {
	TaskID    uuid.UUID
	ProjectID uuid.UUID
	Kind      UserTaskControlType
	Detail    string
}

// UserTaskControlBus in-process шина: подписка оркестратора на команды и оповещение UI об исходе.
type UserTaskControlBus struct {
	mu sync.RWMutex

	cmdSubs    []func(context.Context, UserTaskControlCommand)
	outcomeSubs []func(context.Context, TaskControlOutcome)
}

// NewUserTaskControlBus создаёт шину (один экземпляр на процесс API).
func NewUserTaskControlBus() *UserTaskControlBus {
	return &UserTaskControlBus{}
}

// SubscribeCommands регистрирует обработчик команд (обычно OrchestratorService.Start).
func (b *UserTaskControlBus) SubscribeCommands(fn func(context.Context, UserTaskControlCommand)) {
	if b == nil || fn == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cmdSubs = append(b.cmdSubs, fn)
}

// SubscribeOutcomes регистрирует подписчиков на исходы (UI, аналитика).
func (b *UserTaskControlBus) SubscribeOutcomes(fn func(context.Context, TaskControlOutcome)) {
	if b == nil || fn == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.outcomeSubs = append(b.outcomeSubs, fn)
}

// PublishCommand публикует команду всем подписчикам (асинхронно, без блокировки HTTP).
// Контекст запроса не передаётся в фон: после ответа HTTP он отменяется — используем WithoutCancel.
func (b *UserTaskControlBus) PublishCommand(ctx context.Context, cmd UserTaskControlCommand) {
	if b == nil {
		return
	}
	dispatchCtx := context.WithoutCancel(ctx)
	b.mu.RLock()
	subs := append([]func(context.Context, UserTaskControlCommand){}, b.cmdSubs...)
	b.mu.RUnlock()
	for _, fn := range subs {
		f := fn
		go f(dispatchCtx, cmd)
	}
}

// PublishOutcome публикует исход обработки (после смены состояния / реакции оркестратора).
func (b *UserTaskControlBus) PublishOutcome(ctx context.Context, ev TaskControlOutcome) {
	if b == nil {
		return
	}
	dispatchCtx := context.WithoutCancel(ctx)
	b.mu.RLock()
	subs := append([]func(context.Context, TaskControlOutcome){}, b.outcomeSubs...)
	b.mu.RUnlock()
	for _, fn := range subs {
		f := fn
		go f(dispatchCtx, ev)
	}
}
