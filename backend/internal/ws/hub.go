package ws

import (
	"context"
)

// Hub — центральный менеджер WebSocket-подключений, организованный по project_id.
// Использует Actor Model: ВСЕ мутации состояния происходят в единственной горутине Run().
// Это исключает race conditions при работе с clients.
//
// Контракт:
//   - Register: добавляет Client в группу project_id (отправляет в канал register)
//   - Unregister: удаляет Client из всех групп (отправляет в канал unregister)
//   - SendToProject: отправляет сообщение всем клиентам в проекте (неблокирующая запись)
//   - SendToClient: отправляет сообщение конкретному клиенту (неблокирующая запись)
type Hub struct {
	register       chan *RegisterMessage
	unregister     chan *Client
	broadcast      chan *Message
	unicast        chan *UnicastMessage
	done           chan struct{}
	projects       map[string]map[*Client]bool
	clientProjects map[*Client]map[string]bool
	clientsByID    map[string]*Client
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

// RegisterMessage — сообщение для регистрации клиента (Actor Model: данные летят вместе с клиентом).
type RegisterMessage struct {
	Client     *Client
	ProjectIDs []string
}

// NewHub создаёт новый Hub.
func NewHub() *Hub {
	return &Hub{
		register:       make(chan *RegisterMessage),
		unregister:     make(chan *Client),
		broadcast:      make(chan *Message, 256),
		unicast:        make(chan *UnicastMessage, 256),
		done:           make(chan struct{}),
		projects:       make(map[string]map[*Client]bool),
		clientProjects: make(map[*Client]map[string]bool),
		clientsByID:    make(map[string]*Client),
	}
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
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.broadcastToProject(msg)
		case msg := <-h.unicast:
			h.sendToClient(msg)
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

// SendToClient отправляет сообщение конкретному клиенту по ClientID.
// НЕБЛОКИРУЮЩАЯ операция.
func (h *Hub) SendToClient(clientID, msgType string, payload []byte) error {
	if clientID == "" {
		return ErrEmptyProjectID
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
	h.clientsByID[client.ID] = client

	for _, projectID := range projectIDs {
		if h.projects[projectID] == nil {
			h.projects[projectID] = make(map[*Client]bool)
		}
		h.projects[projectID][client] = true
	}

	h.clientProjects[client] = make(map[string]bool)
	for _, pid := range projectIDs {
		h.clientProjects[client][pid] = true
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

	for projectID := range h.clientProjects[client] {
		delete(h.projects[projectID], client)
		if len(h.projects[projectID]) == 0 {
			delete(h.projects, projectID)
		}
	}
	delete(h.clientProjects, client)

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
