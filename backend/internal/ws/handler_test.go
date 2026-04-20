package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"log/slog"
)

// mockProjectService implements the ProjectService interface for testing
type mockProjectService struct {
	access map[string]bool // key = "userID:projectID"
}

func (m *mockProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	key := userID.String() + ":" + projectID.String()
	if m.access[key] {
		return nil
	}
	return service.ErrProjectForbidden
}

// Unused interface methods - implement to satisfy ProjectService interface
func (m *mockProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	return nil, 0, nil
}
func (m *mockProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}

// Ensure mockProjectService implements the interface
var _ service.ProjectService = (*mockProjectService)(nil)

// TestCheckOrigin tests the CheckOrigin function behavior
func TestCheckOrigin_EmptyOrigin(t *testing.T) {
	cfg := HandlerConfig{
		MaxConnsPerUserProject: 5,
		AllowedOrigins:         []string{"https://example.com"},
	}

	hub := NewHub()
	handler := NewWebSocketHandler(hub, &mockProjectService{access: map[string]bool{}}, cfg, slog.Default())

	upgrader := websocket.Upgrader{
		CheckOrigin: handler.upgrader.CheckOrigin,
	}

	req := httptest.NewRequest("GET", "/projects/"+uuid.NewString()+"/ws", nil)
	assert.True(t, upgrader.CheckOrigin(req))
}

func TestCheckOrigin_Whitelisted(t *testing.T) {
	cfg := HandlerConfig{
		MaxConnsPerUserProject: 5,
		AllowedOrigins:         []string{"https://example.com"},
	}

	hub := NewHub()
	handler := NewWebSocketHandler(hub, &mockProjectService{access: map[string]bool{}}, cfg, slog.Default())

	upgrader := websocket.Upgrader{
		CheckOrigin: handler.upgrader.CheckOrigin,
	}

	req := httptest.NewRequest("GET", "/projects/"+uuid.NewString()+"/ws", nil)
	req.Header.Set("Origin", "https://example.com")

	assert.True(t, upgrader.CheckOrigin(req))
}

func TestCheckOrigin_NotWhitelisted(t *testing.T) {
	cfg := HandlerConfig{
		MaxConnsPerUserProject: 5,
		AllowedOrigins:         []string{"https://example.com"},
	}

	hub := NewHub()
	handler := NewWebSocketHandler(hub, &mockProjectService{access: map[string]bool{}}, cfg, slog.Default())

	upgrader := websocket.Upgrader{
		CheckOrigin: handler.upgrader.CheckOrigin,
	}

	req := httptest.NewRequest("GET", "/projects/"+uuid.NewString()+"/ws", nil)
	req.Header.Set("Origin", "https://evil.com")

	assert.False(t, upgrader.CheckOrigin(req))
}

func TestWebSocketHandler_Connect_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	projectID := uuid.New()

	ps := &mockProjectService{
		access: map[string]bool{
			userID.String() + ":" + projectID.String(): true,
		},
	}

	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	cfg := HandlerConfig{
		AllowedOrigins:         []string{"https://example.com"},
		MaxConnsPerUserProject: 5,
		ReadBufferSize:         1024,
		WriteBufferSize:        1024,
	}

	handler := NewWebSocketHandler(hub, ps, cfg, slog.Default())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		c.Params = gin.Params{{Key: "id", Value: projectID.String()}}
		c.Set("userID", userID)
		c.Set("userRole", "user")
		handler.Connect(c)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/projects/" + projectID.String() + "/ws"
	header := http.Header{}
	header.Set("Origin", "https://example.com")
	header.Set("Sec-WebSocket-Protocol", "bearer.test-token")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	assert.Equal(t, "bearer.test-token", resp.Header.Get("Sec-WebSocket-Protocol"))

	// Check if client is registered in hub
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.CountUserConnections(userID.String(), projectID.String()))
}

func TestWebSocketHandler_Connect_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	projectID := uuid.New()

	// No access in mock
	ps := &mockProjectService{
		access: map[string]bool{},
	}

	hub := NewHub()
	cfg := HandlerConfig{
		MaxConnsPerUserProject: 5,
	}

	handler := NewWebSocketHandler(hub, ps, cfg, slog.Default())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: projectID.String()}}
	c.Set("userID", userID)
	c.Set("userRole", "user")

	handler.Connect(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebSocketHandler_Connect_LimitExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	projectID := uuid.New()

	ps := &mockProjectService{
		access: map[string]bool{
			userID.String() + ":" + projectID.String(): true,
		},
	}

	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	cfg := HandlerConfig{
		MaxConnsPerUserProject: 1, // Limit 1
	}

	handler := NewWebSocketHandler(hub, ps, cfg, slog.Default())

	// Register one client manually
	client1 := NewClient("c1", userID.String(), nil, hub)
	client1.Send = make(chan []byte, 1)
	hub.RegisterIfUnderLimit(client1, []string{projectID.String()}, 5)
	time.Sleep(50 * time.Millisecond)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: projectID.String()}}
	c.Set("userID", userID)
	c.Set("userRole", "user")

	handler.Connect(c)

	if w.Code != 429 {
		t.Errorf("expected status 429, got %d. body: %s", w.Code, w.Body.String())
	}
}

func TestHubRegisterIfUnderLimit(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := &Client{
		ID:     "test-client",
		UserID: "user1",
		Send:   make(chan []byte, 1),
	}

	ok := hub.RegisterIfUnderLimit(client, []string{"project1"}, 5)
	assert.True(t, ok)

	time.Sleep(10 * time.Millisecond)
	count := hub.CountUserConnections("user1", "project1")
	assert.Equal(t, 1, count)
}

func TestHubRegisterIfUnderLimit_Exceed(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	for i := 0; i < 3; i++ {
		client := &Client{
			ID:     "client-" + string(rune('a'+i)),
			UserID: "user1",
			Send:   make(chan []byte, 1),
		}
		ok := hub.RegisterIfUnderLimit(client, []string{"project1"}, 3)
		assert.True(t, ok)
	}
	time.Sleep(10 * time.Millisecond)

	client := &Client{
		ID:     "client-overflow",
		UserID: "user1",
		Send:   make(chan []byte, 1),
	}
	ok := hub.RegisterIfUnderLimit(client, []string{"project1"}, 3)
	assert.False(t, ok)
}

func TestHubCountUserConnections(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	count := hub.CountUserConnections("user1", "project1")
	assert.Equal(t, 0, count)

	for i := 0; i < 3; i++ {
		client := &Client{
			ID:     "client-" + string(rune('a'+i)),
			UserID: "user1",
			Send:   make(chan []byte, 1),
		}
		hub.RegisterIfUnderLimit(client, []string{"project1"}, 5)
	}
	time.Sleep(10 * time.Millisecond)

	count = hub.CountUserConnections("user1", "project1")
	assert.Equal(t, 3, count)

	count = hub.CountUserConnections("user1", "project2")
	assert.Equal(t, 0, count)
}

func TestHubCountUserConnections_MultipleProjects(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := &Client{
		ID:     "client-multi",
		UserID: "user1",
		Send:   make(chan []byte, 1),
	}
	hub.RegisterIfUnderLimit(client, []string{"project1", "project2"}, 5)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.CountUserConnections("user1", "project1"))
	assert.Equal(t, 1, hub.CountUserConnections("user1", "project2"))
}

func TestHubUnregister_DecrementCounter(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	client := &Client{
		ID:     "client-unregister",
		UserID: "user1",
		Send:   make(chan []byte, 1),
	}
	hub.RegisterIfUnderLimit(client, []string{"project1"}, 5)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.CountUserConnections("user1", "project1"))

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.CountUserConnections("user1", "project1"))
}

func TestHubRegisterIfUnderLimit_DifferentUsers(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	// User1 connects to project1 (limit 5)
	client1 := &Client{ID: "c1", UserID: "user1", Send: make(chan []byte, 1)}
	ok := hub.RegisterIfUnderLimit(client1, []string{"project1"}, 5)
	assert.True(t, ok)

	// User2 connects to project1 (different user, separate limit)
	client2 := &Client{ID: "c2", UserID: "user2", Send: make(chan []byte, 1)}
	ok = hub.RegisterIfUnderLimit(client2, []string{"project1"}, 5)
	assert.True(t, ok)

	// Both should be registered
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.CountUserConnections("user1", "project1"))
	assert.Equal(t, 1, hub.CountUserConnections("user2", "project1"))
}

func TestHubRegisterIfUnderLimit_MultipleProjects(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	// User1 connects to project1 and project2
	client := &Client{ID: "c1", UserID: "user1", Send: make(chan []byte, 1)}
	ok := hub.RegisterIfUnderLimit(client, []string{"project1", "project2"}, 5)
	assert.True(t, ok)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.CountUserConnections("user1", "project1"))
	assert.Equal(t, 1, hub.CountUserConnections("user1", "project2"))

	// User1 tries to connect to project1 again (second connection to same project)
	client2 := &Client{ID: "c2", UserID: "user1", Send: make(chan []byte, 1)}
	ok = hub.RegisterIfUnderLimit(client2, []string{"project1"}, 5)
	assert.True(t, ok) // Still under limit of 5

	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, hub.CountUserConnections("user1", "project1"))
}
