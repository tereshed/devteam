package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrLockHeld возвращается, когда лок уже захвачен другим владельцем и ещё не истёк.
var ErrLockHeld = errors.New("lock is held by another owner")

// ErrLockNotOwned возвращается, когда вызывающий пытается продлить или освободить
// лок, который ему не принадлежит (или уже был освобождён/перехвачен).
var ErrLockNotOwned = errors.New("lock is not owned by caller")

// InMemoryLocker — in-process реализация Locker.
//
// Подходит для single-instance деплоя. В кластерном развёртывании следует заменить
// на Redis/Postgres-based реализацию (см. Sprint 9.5: distributed lock с TTL).
type InMemoryLocker struct {
	mu    sync.Mutex
	locks map[string]inMemLockEntry
}

type inMemLockEntry struct {
	id        string
	expiresAt time.Time
}

// NewInMemoryLocker создаёт новый локер.
func NewInMemoryLocker() *InMemoryLocker {
	return &InMemoryLocker{locks: make(map[string]inMemLockEntry)}
}

// Lock пытается захватить лок. Если существующий лок истёк, он перезаписывается.
func (l *InMemoryLocker) Lock(_ context.Context, key string, ttl time.Duration) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if existing, ok := l.locks[key]; ok && existing.expiresAt.After(now) {
		return "", ErrLockHeld
	}

	id := uuid.NewString()
	l.locks[key] = inMemLockEntry{id: id, expiresAt: now.Add(ttl)}
	return id, nil
}

// Unlock освобождает лок, если caller владеет им.
func (l *InMemoryLocker) Unlock(_ context.Context, key, lockID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	existing, ok := l.locks[key]
	if !ok {
		return nil // already released
	}
	if existing.id != lockID {
		return ErrLockNotOwned
	}
	delete(l.locks, key)
	return nil
}

// Refresh продлевает TTL существующего лока.
func (l *InMemoryLocker) Refresh(_ context.Context, key, lockID string, ttl time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	existing, ok := l.locks[key]
	if !ok || existing.id != lockID {
		return ErrLockNotOwned
	}
	existing.expiresAt = time.Now().Add(ttl)
	l.locks[key] = existing
	return nil
}
