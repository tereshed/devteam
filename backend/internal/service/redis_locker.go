package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// redis_locker.go — распределённая реализация Locker поверх Redis.
//
// Заменяет InMemoryLocker в multi-instance деплое: лок виден всем репликам, поэтому
// фоновые операции с эксклюзивным доступом (например, переиндексация проекта в Weaviate)
// не запускаются параллельно на разных нодах. InMemoryLocker держал состояние в памяти
// процесса — на N репликах каждая считала лок свободным и работала независимо (двойная запись).

const redisLockKeyPrefix = "devteam:lock:"

// RedisLocker реализует Locker через SET NX PX + owner-токен.
type RedisLocker struct {
	client *redis.Client
}

// NewRedisLocker создаёт распределённый локер поверх Redis.
func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{client: client}
}

func redisLockKey(key string) string { return redisLockKeyPrefix + key }

// Lock: атомарный SET key=ownerID NX PX=ttl. Успех → владеем; занято → ErrLockHeld.
func (l *RedisLocker) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	id := uuid.NewString()
	ok, err := l.client.SetNX(ctx, redisLockKey(key), id, ttl).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrLockHeld
	}
	return id, nil
}

// Unlock и Refresh идут через Lua: проверка владельца и действие должны быть атомарны,
// иначе возможна гонка «проверил владельца → TTL истёк, ключ перехватили → удалил чужой лок».

// redisUnlockScript: 0 — ключа нет (уже освобождён), 1 — удалили как владелец, -1 — чужой владелец.
var redisUnlockScript = redis.NewScript(`
local v = redis.call("GET", KEYS[1])
if not v then
	return 0
elseif v == ARGV[1] then
	redis.call("DEL", KEYS[1])
	return 1
else
	return -1
end`)

// Unlock освобождает лок, если caller владеет им. Отсутствие лока — не ошибка (как у InMemoryLocker).
func (l *RedisLocker) Unlock(ctx context.Context, key, lockID string) error {
	res, err := redisUnlockScript.Run(ctx, l.client, []string{redisLockKey(key)}, lockID).Int()
	if err != nil {
		return err
	}
	if res == -1 {
		return ErrLockNotOwned
	}
	return nil
}

// redisRefreshScript: 1 — продлили как владелец, иначе -1 (ключа нет или чужой владелец).
var redisRefreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
	return -1
end`)

// Refresh продлевает TTL лока, если caller владеет им (иначе ErrLockNotOwned).
func (l *RedisLocker) Refresh(ctx context.Context, key, lockID string, ttl time.Duration) error {
	res, err := redisRefreshScript.Run(ctx, l.client, []string{redisLockKey(key)}, lockID, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotOwned
	}
	return nil
}
