package ws

import (
	"context"
)

// maxProjectsPerClient ограничивает число project_id на одного клиента (защита от OOM / зависания Run).
const maxProjectsPerClient = 100

// Hub — центральный менеджер WebSocket-подключений, организованный по project_id.
// Использует Actor Model: ВСЕ мутации состояния происходят в единственной горутине Run().
// Это исключает race conditions при работе с clients.
//
// Контракт:
//   - Register: добавляет Client в группу project_id (отправляет в канал register)
//   - Unregister: удаляет Client из всех групп (отправляет в канал unregister)
//   - SendToProject: отправляет сообщение всем клиентам в проекте (неблокирующая запись)
//   - SendToClient: отправляет сообщение конкретному клиенту (неблокирующая запись)
//   - RegisterIfUnderLimit: атомарно проверяет лимит и регистрирует клиента
//   - CountUserConnections: возвращает число активных соединений для пары (userID, projectID)
type Hub struct {
	register       chan *RegisterMessage
	unregister     chan *Client
	broadcast      chan *Message
	userBroadcast  chan *UserMessage
	unicast        chan *UnicastMessage
	done           chan struct{}
	projects       map[string]map[*Client]bool
	clientProjects map[*Client]map[string]bool
	clientsByID    map[string]*Client
	// clientsByUser хранит множество клиентов одного пользователя для user-scoped fan-out
	// (Hub.SendToUser → integration_status и т.п., см. dashboard-redesign §4a.4).
	clientsByUser map[string]map[*Client]bool
	// userConnCounts хранит количество соединений для пары (userID, projectID).
	// Ключ = "userID:projectID". Обновляется атомарно в Run().
	userConnCounts map[string]int
	// countReq is the channel for connection count queries
	countReq chan *CountUserConnectionsMessage
	// registerIfLimitReq is the channel for atomic register-with-limit check
	registerIfLimitReq chan *RegisterIfLimitMessage
}

// RegisterIfLimitMessage is the message for atomic register-if-under-limit check.
type RegisterIfLimitMessage struct {
	Client     *Client
	ProjectIDs []string
	Max        int
	Resp       chan bool // true if registered, false if limit exceeded
}

// Message — сообщение для broadcast в проект.
type Message struct {
	ProjectID string
	Type      string
	Payload   []byte
}

// UnicastMessage — сообщение для конкретного клиента.
type UnicastMessage struct {
	ClientID string
	Type     string
	Payload  []byte
}

// UserMessage — сообщение для всех клиентов одного пользователя (user-scoped fan-out).
type UserMessage struct {
	UserID  string
	Type    string
	Payload []byte
}

// RegisterMessage — сообщение для регистрации клиента (Actor Model: данные летят вместе с клиентом).
type RegisterMessage struct {
	Client     *Client
	ProjectIDs []string
	// ack, если не nil, закрывается после завершения addClient в Run (строгая синхронизация для тестов).
	ack chan struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		register:           make(chan *RegisterMessage),
		unregister:         make(chan *Client),
		broadcast:          make(chan *Message, 256),
		userBroadcast:      make(chan *UserMessage, 256),
		unicast:            make(chan *UnicastMessage, 256),
		done:               make(chan struct{}),
		projects:           make(map[string]map[*Client]bool),
		clientProjects:     make(map[*Client]map[string]bool),
		clientsByID:        make(map[string]*Client),
		clientsByUser:      make(map[string]map[*Client]bool),
		userConnCounts:     make(map[string]int),
		countReq:           make(chan *CountUserConnectionsMessage),
		registerIfLimitReq: make(chan *RegisterIfLimitMessage),
	}
}

// CountUserConnectionsMessage is the message for querying connection count.
type CountUserConnectionsMessage struct {
	UserID    string
	ProjectID string
	Resp      chan int
}

// Run запускает основной цикл Hub. ВСЕ мутации состояния — здесь.
// Блокируется до закрытия контекста или закрытия каналов.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case reg := <-h.register:
			h.addClient(reg.Client, reg.ProjectIDs)
			if reg.ack != nil {
				close(reg.ack)
			}
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.broadcastToProject(msg)
		case msg := <-h.userBroadcast:
			h.broadcastToUser(msg)
		case msg := <-h.unicast:
			h.sendToClient(msg)
		case req := <-h.countReq:
			req.Resp <- h.getUserConnCount(req.UserID, req.ProjectID)
		case req := <-h.registerIfLimitReq:
			req.Resp <- h.registerIfUnderLimit(req.Client, req.ProjectIDs, req.Max)
		}
	}
}

// Register добавляет клиента в Hub. Вызывается из Handler (7.2).
// Hub НЕ валидирует права доступа — это обязанность Handler.
// Handler ОБЯЗАН проверить, что пользователь имеет доступ к projectIDs
// (через сервис/БД), прежде чем вызывать Register.
//
// Actor Model: projectIDs летят вместе с событием регистрации, НЕ мутируем клиента.
func (h *Hub) Register(client *Client, projectIDs []string) {
	select {
	case h.register <- &RegisterMessage{Client: client, ProjectIDs: projectIDs}:
	case <-h.done:
	}
}

// Unregister удаляет клиента из Hub. Вызывается при закрытии соединения.
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

// CountUserConnections returns the number of active connections for a (userID, projectID) pair.
// Returns 0 if no connections exist.
// Thread-safe via Actor Model (handled in Run()).
func (h *Hub) CountUserConnections(userID, projectID string) int {
	resp := make(chan int)
	select {
	case h.countReq <- &CountUserConnectionsMessage{UserID: userID, ProjectID: projectID, Resp: resp}:
		return <-resp
	case <-h.done:
		return 0
	}
}

// RegisterIfUnderLimit atomically checks connection limit and registers client if under limit.
// Returns true if client was registered, false if limit exceeded.
// Thread-safe via Actor Model (handled in Run()).
func (h *Hub) RegisterIfUnderLimit(client *Client, projectIDs []string, max int) bool {
	resp := make(chan bool)
	select {
	case h.registerIfLimitReq <- &RegisterIfLimitMessage{Client: client, ProjectIDs: projectIDs, Max: max, Resp: resp}:
		return <-resp
	case <-h.done:
		return false
	}
}

// getUserConnCount is the internal helper called from Run().
func (h *Hub) getUserConnCount(userID, projectID string) int {
	key := userConnKey(userID, projectID)
	return h.userConnCounts[key]
}

func clipProjectIDs(projectIDs []string) []string {
	if len(projectIDs) > maxProjectsPerClient {
		return projectIDs[:maxProjectsPerClient]
	}
	return projectIDs
}

// registerIfUnderLimit is the internal helper called from Run().
// It checks the limit and registers if OK.
func (h *Hub) registerIfUnderLimit(client *Client, projectIDs []string, max int) bool {
	projectIDs = clipProjectIDs(projectIDs)
	for _, pid := range projectIDs {
		key := userConnKey(client.UserID, pid)
		if h.userConnCounts[key] >= max {
			return false
		}
	}

	// Under limit, register client
	h.addClient(client, projectIDs)

	return true
}

// userConnKey builds the map key for userConnCounts.
func userConnKey(userID, projectID string) string {
	return userID + ":" + projectID
}

// SendToProject отправляет сообщение всем клиентам проекта.
// НЕБЛОКИРУЮЩАЯ операция: если канал broadcast переполнен (медленные клиенты),
// сообщение дропается для этого проекта (slow client isolation).
func (h *Hub) SendToProject(projectID, msgType string, payload []byte) error {
	if projectID == "" {
		return ErrEmptyProjectID
	}
	select {
	case h.broadcast <- &Message{ProjectID: projectID, Type: msgType, Payload: payload}:
		return nil
	default:
		return nil
	}
}

// SendToUser отправляет сообщение всем активным клиентам одного пользователя (по всем
// его открытым проектам). НЕБЛОКИРУЮЩАЯ операция: при переполнении канала событие дропается.
//
// Используется для user-scoped событий (например, integration_status в dashboard-redesign §4a.4),
// которые не привязаны к конкретному проекту.
func (h *Hub) SendToUser(userID, msgType string, payload []byte) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	select {
	case h.userBroadcast <- &UserMessage{UserID: userID, Type: msgType, Payload: payload}:
		return nil
	default:
		return nil
	}
}

// SendToClient отправляет сообщение конкретному клиенту по ClientID.
// НЕБЛОКИРУЮЩАЯ операция.
func (h *Hub) SendToClient(clientID, msgType string, payload []byte) error {
	if clientID == "" {
		return ErrEmptyClientID
	}
	select {
	case h.unicast <- &UnicastMessage{ClientID: clientID, Type: msgType, Payload: payload}:
		return nil
	default:
		return nil
	}
}

// addClient регистрирует клиента во всех его проектах.
// ВЫЗЫВАТЬ ТОЛЬКО ИЗ Run()
func (h *Hub) addClient(client *Client, projectIDs []string) {
	projectIDs = clipProjectIDs(projectIDs)

	h.clientsByID[client.ID] = client

	for _, projectID := range projectIDs {
		if h.projects[projectID] == nil {
			h.projects[projectID] = make(map[*Client]bool)
		}
		h.projects[projectID][client] = true
	}

	if h.clientProjects[client] == nil {
		h.clientProjects[client] = make(map[string]bool)
	}
	for _, pid := range projectIDs {
		h.clientProjects[client][pid] = true
		key := userConnKey(client.UserID, pid)
		h.userConnCounts[key]++
	}

	if client.UserID != "" {
		if h.clientsByUser[client.UserID] == nil {
			h.clientsByUser[client.UserID] = make(map[*Client]bool)
		}
		h.clientsByUser[client.UserID][client] = true
	}
}

// removeClient удаляет клиента из Hub и всех его проектов.
// Закрывает канал Send клиента.
// ВЫЗЫВАТЬ ТОЛЬКО ИЗ Run()
// Idempotent: повторный вызов на уже удалённом клиенте — no-op.
func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clientProjects[client]; !ok {
		return
	}

	// Decrement connection counters for this client
	for projectID := range h.clientProjects[client] {
		key := userConnKey(client.UserID, projectID)
		if h.userConnCounts[key] > 0 {
			h.userConnCounts[key]--
		}
	}

	for projectID := range h.clientProjects[client] {
		delete(h.projects[projectID], client)
		if len(h.projects[projectID]) == 0 {
			delete(h.projects, projectID)
		}
	}
	delete(h.clientProjects, client)

	if client.UserID != "" {
		if users := h.clientsByUser[client.UserID]; users != nil {
			delete(users, client)
			if len(users) == 0 {
				delete(h.clientsByUser, client.UserID)
			}
		}
	}

	delete(h.clientsByID, client.ID)

	close(client.Send)
}

// broadcastToProject отправляет сообщение всем клиентам проекта.
// НЕБЛОКИРУЮЩАЯ запись в каждый client.Send.
// ВЫЗЫВАТЬ ТОЛЬКО ИЗ Run()
func (h *Hub) broadcastToProject(msg *Message) {
	clients, ok := h.projects[msg.ProjectID]
	if !ok || len(clients) == 0 {
		return
	}

	for client := range clients {
		select {
		case client.Send <- msg.Payload:
		default:
			h.removeClient(client)
		}
	}
}

// broadcastToUser отправляет сообщение всем активным клиентам одного пользователя.
// НЕБЛОКИРУЮЩАЯ запись в каждый client.Send (slow client isolation).
// ВЫЗЫВАТЬ ТОЛЬКО ИЗ Run()
func (h *Hub) broadcastToUser(msg *UserMessage) {
	clients, ok := h.clientsByUser[msg.UserID]
	if !ok || len(clients) == 0 {
		return
	}

	for client := range clients {
		select {
		case client.Send <- msg.Payload:
		default:
			h.removeClient(client)
		}
	}
}

// sendToClient отправляет сообщение конкретному клиенту.
// НЕБЛОКИРУЮЩАЯ запись.
// ВЫЗЫВАТЬ ТОЛЬКО ИЗ Run()
func (h *Hub) sendToClient(msg *UnicastMessage) {
	client, ok := h.clientsByID[msg.ClientID]
	if !ok {
		return
	}
	select {
	case client.Send <- msg.Payload:
	default:
		h.removeClient(client)
	}
}

// shutdown корректно завершает все подключения при остановке Hub.
func (h *Hub) shutdown() {
	for client := range h.clientProjects {
		select {
		case client.Send <- []byte(`{"type":"close","reason":"server_shutdown"}`):
		default:
		}
		h.removeClient(client)
	}
}
