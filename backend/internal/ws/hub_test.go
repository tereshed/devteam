package ws

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type mockConn struct {
	writeMu sync.Mutex
}

func (m *mockConn) WriteMessage(_ int, data []byte) error {
	return nil
}

func (m *mockConn) ReadMessage() (int, []byte, error) {
	return 0, nil, websocket.ErrCloseSent
}

func (m *mockConn) Close() error {
	return nil
}

func newTestClient(id string, hub *Hub) *Client {
	wsConn := &websocket.Conn{}
	return &Client{
		ID:   id,
		Conn: wsConn,
		Send: make(chan []byte, 256),
		Hub:  hub,
	}
}

func TestHubRegister(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1", "p2"})

	time.Sleep(10 * time.Millisecond)

	if _, ok := hub.clientsByID["c1"]; !ok {
		t.Error("client c1 should be registered in clientsByID")
	}

	if _, ok := hub.projects["p1"][client]; !ok {
		t.Error("client c1 should be in project p1")
	}

	if _, ok := hub.projects["p2"][client]; !ok {
		t.Error("client c1 should be in project p2")
	}
}

func TestHubUnregister(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	if _, ok := hub.clientsByID["c1"]; ok {
		t.Error("client c1 should be removed from clientsByID")
	}

	if pr, ok := hub.projects["p1"]; ok {
		if _, ok := pr[client]; ok {
			t.Error("client c1 should be removed from project p1")
		}
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"type":"test","data":"hello"}`)
	err := hub.SendToProject("p1", "test", payload)
	if err != nil {
		t.Errorf("SendToProject returned error: %v", err)
	}

	select {
	case msg := <-client.Send:
		if string(msg) != string(payload) {
			t.Errorf("expected payload %s, got %s", payload, msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client should receive broadcast message")
	}
}

func TestHubBroadcastEmptyProjectID(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	err := hub.SendToProject("", "test", []byte("data"))
	if err != ErrEmptyProjectID {
		t.Errorf("expected ErrEmptyProjectID, got %v", err)
	}
}

func TestHubSlowClientDisconnect(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	client.Send = make(chan []byte, 1)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	hub.SendToProject("p1", "msg", []byte("first"))
	hub.SendToProject("p1", "msg", []byte("second"))
	time.Sleep(50 * time.Millisecond)

	select {
	case <-client.Send:
		// ok, at least one message went through
	default:
	}

	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.clientsByID["c1"]; ok {
		t.Error("slow client should be disconnected")
	}
}

func TestHubSendToClient(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	payload := []byte(`{"type":"unicast"}`)
	err := hub.SendToClient("c1", "unicast", payload)
	if err != nil {
		t.Errorf("SendToClient returned error: %v", err)
	}

	select {
	case msg := <-client.Send:
		var got map[string]interface{}
		json.Unmarshal(msg, &got)
		pl, _ := json.Marshal(got["type"])
		if string(pl) != `"unicast"` {
			t.Errorf("expected type unicast, got %v", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client should receive unicast message")
	}
}

func TestHubShutdown(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	cancel()
	time.Sleep(20 * time.Millisecond)

	if _, ok := hub.clientsByID["c1"]; ok {
		t.Error("client should be removed after shutdown")
	}
}

func TestHubIdempotentRemoveClient(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := newTestClient("c1", hub)
	hub.Register(client, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	hub.removeClient(client)
	hub.removeClient(client)

	if _, ok := hub.clientsByID["c1"]; ok {
		t.Error("second removeClient should be no-op")
	}
}

func TestHubMultipleClientsSameProject(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	c1 := newTestClient("c1", hub)
	c2 := newTestClient("c2", hub)
	hub.Register(c1, []string{"p1"})
	hub.Register(c2, []string{"p1"})
	time.Sleep(10 * time.Millisecond)

	payload := []byte("broadcast")
	hub.SendToProject("p1", "msg", payload)

	select {
	case <-c1.Send:
	case <-time.After(100 * time.Millisecond):
		t.Error("c1 should receive broadcast")
	}

	select {
	case <-c2.Send:
	case <-time.After(100 * time.Millisecond):
		t.Error("c2 should receive broadcast")
	}
}
