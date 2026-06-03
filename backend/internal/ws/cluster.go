package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// cluster.go — кросс-нодовая доставка WebSocket-сообщений через Redis Pub/Sub.
//
// Проблема: WS-соединения держатся в памяти конкретного инстанса (Hub.projects /
// Hub.clientsByUser). При N репликах за балансировщиком клиент проекта P может быть
// подключён к api-2, а доменное событие по P опубликовано на api-1. Без моста api-1
// доставит сообщение только своим локальным клиентам — UI на api-2 «замёрзнет».
//
// Решение: каждый инстанс публикует исходящие project/user-scoped сообщения в общий
// Redis-канал и одновременно слушает его, ре-доставляя ЧУЖИЕ сообщения своим локальным
// клиентам. Доменная шина событий (events.EventBus) и её подписчики (индексаторы,
// vectordb-listener) при этом НЕ затрагиваются — наружу уходит уже сериализованный
// и заскрабленный WS-payload из HubBridge, ровно один раз на ноде-продюсере.
//
// Unicast (SendToClient) НЕ ретранслируется: клиент по ClientID существует только на той
// ноде, к которой подключён, поэтому межинстансная адресная доставка не имеет смысла.

// RedisChannelWSFanout — общий Pub/Sub-канал для кросс-нодового WS fan-out.
const RedisChannelWSFanout = "devteam:ws"

const (
	wsScopeProject = "project"
	wsScopeUser    = "user"
)

// clusterEnvelope — сериализуемая обёртка WS-сообщения для передачи между инстансами.
type clusterEnvelope struct {
	Origin  string          `json:"o"` // instanceID отправителя — для подавления эха
	Scope   string          `json:"s"` // project | user
	Key     string          `json:"k"` // projectID | userID
	Type    string          `json:"t"` // MessageType
	Payload json.RawMessage `json:"p"` // готовый envelope из HubBridge (уже заскраблен)
}

// ClusterBridge связывает локальный Hub с остальными инстансами через Redis Pub/Sub.
//
// Подавление эха: Redis отдаёт сообщение и самому публикующему. По полю Origin инстанс
// отбрасывает собственные сообщения — они уже доставлены локально внутри SendTo*.
type ClusterBridge struct {
	client     *redis.Client
	hub        *Hub
	instanceID string
	log        *slog.Logger
}

// NewClusterBridge создаёт мост. instanceID должен быть уникален на инстанс
// (например, uuid на старте процесса) — на нём держится подавление эха.
func NewClusterBridge(client *redis.Client, hub *Hub, instanceID string, log *slog.Logger) *ClusterBridge {
	if log == nil {
		log = slog.Default()
	}
	return &ClusterBridge{client: client, hub: hub, instanceID: instanceID, log: log}
}

// publishProject ретранслирует project-scoped сообщение остальным инстансам (best-effort).
func (c *ClusterBridge) publishProject(projectID, msgType string, payload []byte) {
	c.publish(wsScopeProject, projectID, msgType, payload)
}

// publishUser ретранслирует user-scoped сообщение остальным инстансам (best-effort).
func (c *ClusterBridge) publishUser(userID, msgType string, payload []byte) {
	c.publish(wsScopeUser, userID, msgType, payload)
}

func (c *ClusterBridge) publish(scope, key, msgType string, payload []byte) {
	env := clusterEnvelope{
		Origin:  c.instanceID,
		Scope:   scope,
		Key:     key,
		Type:    msgType,
		Payload: json.RawMessage(payload),
	}
	data, err := json.Marshal(env)
	if err != nil {
		c.log.Error("ws cluster: marshal envelope failed", "error", err, "scope", scope)
		return
	}
	// Best-effort: ошибка публикации не должна влиять на локальную доставку (она уже
	// произошла в SendTo*). Background-ctx — переживаем отмену request-scoped контекста.
	if err := c.client.Publish(context.Background(), RedisChannelWSFanout, data).Err(); err != nil {
		c.log.Warn("ws cluster: publish failed", "error", err, "scope", scope)
	}
}

// Run слушает общий канал и ре-доставляет чужие сообщения локальным клиентам.
// Блокируется до закрытия ctx.
func (c *ClusterBridge) Run(ctx context.Context) {
	pubsub := c.client.Subscribe(ctx, RedisChannelWSFanout)
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handle(msg.Payload)
		}
	}
}

func (c *ClusterBridge) handle(raw string) {
	var env clusterEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		c.log.Warn("ws cluster: unmarshal envelope failed", "error", err)
		return
	}
	// Подавление эха: собственные сообщения уже доставлены локально в SendTo*.
	if env.Origin == c.instanceID {
		return
	}
	switch env.Scope {
	case wsScopeProject:
		c.hub.enqueueProject(env.Key, env.Type, env.Payload)
	case wsScopeUser:
		c.hub.enqueueUser(env.Key, env.Type, env.Payload)
	default:
		c.log.Warn("ws cluster: unknown scope", "scope", env.Scope)
	}
}
