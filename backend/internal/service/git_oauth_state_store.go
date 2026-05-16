package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// ErrGitOAuthStateNotFound — state не найден / истёк / был использован.
var ErrGitOAuthStateNotFound = errors.New("git oauth state not found or expired")

// GitOAuthState — pending данные между Init и Callback.
// Содержит ВСЁ, что нужно сервису для exchange:
//   - UserID (на чью учётку привязываем);
//   - Provider + Host (для BYO GitLab);
//   - ByoClientID / ByoClientSecret (в памяти plaintext, никогда не логируется);
//   - RedirectURI (callback URL, должен совпадать на exchange).
type GitOAuthState struct {
	UserID          uuid.UUID
	Provider        models.GitIntegrationProvider
	Host            string // пусто для shared (github.com / gitlab.com)
	ByoClientID     string
	ByoClientSecret string
	RedirectURI     string
	CreatedAt       time.Time
}

// GitOAuthStateStore — эфемерный store state-nonce → pending state.
// One-shot: Consume атомарно достаёт и удаляет запись, чтобы один state нельзя было reuse'нуть.
type GitOAuthStateStore interface {
	// New генерирует криптостойкий state-token, сохраняет данные с TTL и возвращает токен.
	New(state GitOAuthState, ttl time.Duration) (string, error)
	// Consume атомарно достаёт и удаляет запись. Возвращает ErrGitOAuthStateNotFound если нет/истекла.
	Consume(token string) (GitOAuthState, error)
}

type stateEntry struct {
	state     GitOAuthState
	expiresAt time.Time
}

type inMemoryGitOAuthStateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	clock   func() time.Time
}

// NewInMemoryGitOAuthStateStore — реализация по умолчанию.
func NewInMemoryGitOAuthStateStore() GitOAuthStateStore {
	return &inMemoryGitOAuthStateStore{
		entries: map[string]stateEntry{},
		clock:   time.Now,
	}
}

func (s *inMemoryGitOAuthStateStore) New(state GitOAuthState, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	tok, err := randomStateToken()
	if err != nil {
		return "", err
	}
	exp := s.clock().Add(ttl)
	s.mu.Lock()
	s.entries[tok] = stateEntry{state: state, expiresAt: exp}
	if len(s.entries) > 512 {
		s.gcLocked()
	}
	s.mu.Unlock()
	time.AfterFunc(ttl, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if e, ok := s.entries[tok]; ok && !e.expiresAt.After(s.clock()) {
			delete(s.entries, tok)
		}
	})
	return tok, nil
}

func (s *inMemoryGitOAuthStateStore) Consume(tok string) (GitOAuthState, error) {
	if tok == "" {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[tok]
	if !ok {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	delete(s.entries, tok) // one-shot
	if !e.expiresAt.After(s.clock()) {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	return e.state, nil
}

func (s *inMemoryGitOAuthStateStore) gcLocked() {
	now := s.clock()
	for k, v := range s.entries {
		if !v.expiresAt.After(now) {
			delete(s.entries, k)
		}
	}
}

func randomStateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
