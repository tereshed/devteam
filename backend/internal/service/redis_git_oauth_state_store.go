package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redis_git_oauth_state_store.go — распределённый GitOAuthStateStore поверх Redis.
//
// Заменяет inMemoryGitOAuthStateStore в multi-instance деплое: при балансировщике OAuth-init
// и callback могут попасть на разные реплики. In-memory store держал state в памяти инстанса
// инициатора → callback на другой реплике не находил state (ErrGitOAuthStateNotFound).
//
// Безопасность: blob содержит ByoClientSecret в открытом виде (как и in-memory вариант),
// живёт под коротким TTL (≤10 мин). Если Redis общий/managed — рассмотреть шифрование blob
// (encryptor доступен в DI) как отдельное усиление.

const redisGitOAuthStatePrefix = "devteam:gitoauth:"

// RedisGitOAuthStateStore хранит pending OAuth-state как JSON с TTL; Consume — атомарный one-shot.
type RedisGitOAuthStateStore struct {
	client *redis.Client
	clock  func() time.Time
}

// NewRedisGitOAuthStateStore создаёт Redis-backed store.
func NewRedisGitOAuthStateStore(client *redis.Client) *RedisGitOAuthStateStore {
	return &RedisGitOAuthStateStore{client: client, clock: time.Now}
}

func (s *RedisGitOAuthStateStore) New(state GitOAuthState, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	tok, err := randomStateToken()
	if err != nil {
		return "", err
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = s.clock()
	}
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(context.Background(), redisGitOAuthStatePrefix+tok, data, ttl).Err(); err != nil {
		return "", err
	}
	return tok, nil
}

func (s *RedisGitOAuthStateStore) Consume(tok string) (GitOAuthState, error) {
	if tok == "" {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	// GetDel — атомарный one-shot: достаём и сразу удаляем, исключая reuse одного state.
	// Истёкший по TTL ключ уже отсутствует → redis.Nil → ErrGitOAuthStateNotFound.
	data, err := s.client.GetDel(context.Background(), redisGitOAuthStatePrefix+tok).Bytes()
	if errors.Is(err, redis.Nil) {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	if err != nil {
		return GitOAuthState{}, err
	}
	var state GitOAuthState
	if err := json.Unmarshal(data, &state); err != nil {
		return GitOAuthState{}, ErrGitOAuthStateNotFound
	}
	return state, nil
}
