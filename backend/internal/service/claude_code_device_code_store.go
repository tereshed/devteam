package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// DeviceCodeStore — эфемерный store device_code → user_id с TTL.
//
// Sprint 15.B (security): без него атакующий A может инициировать device-flow, социнженерить
// жертву B ввести user_code на anthropic.com, и поллить /callback своим JWT — оркестратор
// сохранит OAuth-подписку B на user_id A. Привязка device_code к инициатору закрывает эту атаку.
//
// Все store-операции потокобезопасны. TTL > expires_in device-кода (15 мин по умолчанию).
type DeviceCodeStore interface {
	Put(deviceCode string, userID uuid.UUID, ttl time.Duration)
	// Get возвращает (userID, true), если deviceCode известен и TTL не истёк.
	Get(deviceCode string) (uuid.UUID, bool)
	Delete(deviceCode string)
}

type deviceCodeEntry struct {
	userID    uuid.UUID
	expiresAt time.Time
}

type inMemoryDeviceCodeStore struct {
	mu      sync.Mutex
	entries map[string]deviceCodeEntry
	clock   func() time.Time
}

// NewInMemoryDeviceCodeStore — реализация по умолчанию (in-process, без Redis).
// Для multi-replica deploy замените на Redis-backed аналог (контракт сохранять).
func NewInMemoryDeviceCodeStore() DeviceCodeStore {
	return &inMemoryDeviceCodeStore{
		entries: map[string]deviceCodeEntry{},
		clock:   time.Now,
	}
}

func (s *inMemoryDeviceCodeStore) Put(deviceCode string, userID uuid.UUID, ttl time.Duration) {
	if deviceCode == "" || userID == uuid.Nil {
		return
	}
	exp := s.clock().Add(ttl)
	s.mu.Lock()
	s.entries[deviceCode] = deviceCodeEntry{userID: userID, expiresAt: exp}
	// Оппортунистический gc — без бесконечного роста при долгих сессиях с протухшими кодами.
	if len(s.entries) > 256 {
		s.gcLocked()
	}
	s.mu.Unlock()
	// Sprint 15.minor: AfterFunc — даже при низком rate выпавший device_code будет gc'нут точно
	// через ttl, не дожидаясь >256 порога. Таймер сам разовый, без leak.
	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if e, ok := s.entries[deviceCode]; ok && !e.expiresAt.After(s.clock()) {
				delete(s.entries, deviceCode)
			}
		})
	}
}

func (s *inMemoryDeviceCodeStore) Get(deviceCode string) (uuid.UUID, bool) {
	if deviceCode == "" {
		return uuid.Nil, false
	}
	now := s.clock()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[deviceCode]
	if !ok {
		return uuid.Nil, false
	}
	if !e.expiresAt.After(now) {
		delete(s.entries, deviceCode)
		return uuid.Nil, false
	}
	return e.userID, true
}

func (s *inMemoryDeviceCodeStore) Delete(deviceCode string) {
	if deviceCode == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, deviceCode)
}

func (s *inMemoryDeviceCodeStore) gcLocked() {
	now := s.clock()
	for k, v := range s.entries {
		if !v.expiresAt.After(now) {
			delete(s.entries, k)
		}
	}
}
