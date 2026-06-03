package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// redis_device_code_store.go — распределённый DeviceCodeStore поверх Redis.
//
// Заменяет inMemoryDeviceCodeStore в multi-instance деплое: device-flow init и polling
// (/callback) могут попасть на разные реплики. In-memory store держал device_code→user_id
// в памяти инстанса инициатора → polling на другой реплике кода не находил.
// Привязка device_code→user_id с TTL (защита из Sprint 15.B) сохраняется.

const redisDeviceCodePrefix = "devteam:devicecode:"

// RedisDeviceCodeStore хранит device_code → user_id с TTL в Redis.
type RedisDeviceCodeStore struct {
	client *redis.Client
}

// NewRedisDeviceCodeStore создаёт Redis-backed store.
func NewRedisDeviceCodeStore(client *redis.Client) *RedisDeviceCodeStore {
	return &RedisDeviceCodeStore{client: client}
}

func (s *RedisDeviceCodeStore) Put(deviceCode string, userID uuid.UUID, ttl time.Duration) {
	// ttl<=0 пропускаем: SET без TTL завис бы навсегда (утечка привязки).
	if deviceCode == "" || userID == uuid.Nil || ttl <= 0 {
		return
	}
	_ = s.client.Set(context.Background(), redisDeviceCodePrefix+deviceCode, userID.String(), ttl).Err()
}

func (s *RedisDeviceCodeStore) Get(deviceCode string) (uuid.UUID, bool) {
	if deviceCode == "" {
		return uuid.Nil, false
	}
	v, err := s.client.Get(context.Background(), redisDeviceCodePrefix+deviceCode).Result()
	if err != nil { // redis.Nil (нет/истёк) или ошибка соединения → считаем неизвестным
		return uuid.Nil, false
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func (s *RedisDeviceCodeStore) Delete(deviceCode string) {
	if deviceCode == "" {
		return
	}
	_ = s.client.Del(context.Background(), redisDeviceCodePrefix+deviceCode).Err()
}
