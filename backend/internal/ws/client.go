package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// PongWait is the time allowed to read the next pong message from peer.
	pongWait = 60 * time.Second
	// PingPeriod is the interval between ping messages sent to peer.
	pingPeriod = 54 * time.Second
)

// Client represents a WebSocket client connected to the Hub.
type Client struct {
	ID     string // Server-generated unique ID (never from client)
	UserID string // User ID from verified JWT claims
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Hub
}

// NewClient creates a new Client with server-generated ID and userID from JWT.
func NewClient(id, userID string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:     id,
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    hub,
	}
}

// WritePump pumps messages from the hub to the websocket connection.
// It runs in a goroutine and handles ping messages for keep-alive.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		c.Hub.Unregister(c)
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// Hub closed the channel, send close message
				c.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
// It handles pong responses to update the read deadline.
func (c *Client) ReadPump() error {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	// Set initial read deadline
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.Conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return err
			}
			return nil // Normal closure
		}
	}
}