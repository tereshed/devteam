package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/pkg/secrets"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type mockHub struct {
	sent chan *Message
}

func (h *mockHub) SendToProject(projectID, msgType string, payload []byte) error {
	h.sent <- &Message{ProjectID: projectID, Type: msgType, Payload: payload}
	return nil
}

func TestHubBridge_IntegrationConnectionChanged(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := events.NewInMemoryBus(nil, nil)
	defer bus.Close()

	hub := &Hub{
		broadcast:     make(chan *Message, 10),
		userBroadcast: make(chan *UserMessage, 10),
	}

	scrub := secrets.NewScrubber()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bridge := NewHubBridge(bus, hub, scrub, log, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridge.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	userID := uuid.New()
	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
	connectedAt := time.Now().UTC()

	t.Run("Connected_routes_to_user_channel", func(t *testing.T) {
		ev := events.IntegrationConnectionChanged{
			UserID:      userID,
			Provider:    "anthropic",
			Status:      events.IntegrationStatusConnected,
			ConnectedAt: &connectedAt,
			ExpiresAt:   &expiresAt,
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.userBroadcast:
			assert.Equal(t, userID.String(), msg.UserID)
			assert.Equal(t, string(MessageTypeIntegrationStatus), msg.Type)

			var env UserEnvelope[IntegrationStatusData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			assert.Equal(t, userID, env.UserID)
			assert.Equal(t, MessageTypeIntegrationStatus, env.Type)
			assert.Equal(t, SchemaVersion, env.Version)
			assert.Equal(t, "anthropic", env.Data.Provider)
			assert.Equal(t, "connected", env.Data.Status)
			assert.Equal(t, "", env.Data.Reason)
			require.NotNil(t, env.Data.ConnectedAt)
			require.NotNil(t, env.Data.ExpiresAt)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for user_broadcast")
		}
	})

	t.Run("Error_status_with_reason", func(t *testing.T) {
		ev := events.IntegrationConnectionChanged{
			UserID:   userID,
			Provider: "deepseek",
			Status:   events.IntegrationStatusError,
			Reason:   "auth_failed",
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.userBroadcast:
			var env UserEnvelope[IntegrationStatusData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			assert.Equal(t, "deepseek", env.Data.Provider)
			assert.Equal(t, "error", env.Data.Status)
			assert.Equal(t, "auth_failed", env.Data.Reason)
			assert.Nil(t, env.Data.ConnectedAt)
			assert.Nil(t, env.Data.ExpiresAt)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for user_broadcast")
		}
	})

	t.Run("Nil_userID_dropped", func(t *testing.T) {
		ev := events.IntegrationConnectionChanged{
			UserID:   uuid.Nil,
			Provider: "anthropic",
			Status:   events.IntegrationStatusConnected,
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.userBroadcast:
			t.Fatalf("did not expect message: %#v", msg)
		case <-time.After(50 * time.Millisecond):
			// ok — dropped
		}
	})

	t.Run("Invalid_status_dropped", func(t *testing.T) {
		ev := events.IntegrationConnectionChanged{
			UserID:   userID,
			Provider: "anthropic",
			Status:   events.IntegrationConnectionStatus("garbled"),
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.userBroadcast:
			t.Fatalf("did not expect message: %#v", msg)
		case <-time.After(50 * time.Millisecond):
			// ok — dropped
		}
	})
}

func TestHub_SendToUser_FansOutAndIsolates(t *testing.T) {
	// goleak verified via t.Cleanup внутри newTestHub.
	hub, _ := newTestHub(t)

	userA := uuid.New().String()
	userB := uuid.New().String()
	projectID := uuid.New().String()

	clientA1 := &Client{ID: "a1", UserID: userA, Send: make(chan []byte, 4)}
	clientA2 := &Client{ID: "a2", UserID: userA, Send: make(chan []byte, 4)}
	clientB1 := &Client{ID: "b1", UserID: userB, Send: make(chan []byte, 4)}
	registerSynced(hub, clientA1, []string{projectID})
	registerSynced(hub, clientA2, []string{projectID})
	registerSynced(hub, clientB1, []string{projectID})

	payload, err := MarshalIntegrationStatus(uuid.MustParse(userA), IntegrationStatusData{
		Provider: "anthropic",
		Status:   "connected",
	})
	require.NoError(t, err)

	require.NoError(t, hub.SendToUser(userA, string(MessageTypeIntegrationStatus), payload))

	for _, c := range []*Client{clientA1, clientA2} {
		select {
		case got := <-c.Send:
			assert.Equal(t, payload, got)
		case <-time.After(time.Second):
			t.Fatalf("client %s did not receive user broadcast", c.ID)
		}
	}

	select {
	case got := <-clientB1.Send:
		t.Fatalf("unrelated user received broadcast: %s", string(got))
	case <-time.After(50 * time.Millisecond):
		// ok — isolated
	}
}

func TestHubBridge_Dispatch(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := events.NewInMemoryBus(nil, nil)
	defer bus.Close()

	hub := &Hub{
		broadcast: make(chan *Message, 10),
	}
	
	scrub := secrets.NewScrubber()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bridge := NewHubBridge(bus, hub, scrub, log, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridge.Run(ctx)

	// Даем время горутине подписаться на шину событий
	time.Sleep(10 * time.Millisecond)

	projectID := uuid.New()
	taskID := uuid.New()

	t.Run("TaskStatusChanged", func(t *testing.T) {
		ev := events.TaskStatusChanged{
			ProjectID: projectID,
			TaskID:    taskID,
			Current:   "running",
			Previous:  "pending",
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.broadcast:
			assert.Equal(t, projectID.String(), msg.ProjectID)
			assert.Equal(t, string(MessageTypeTaskStatus), msg.Type)
			
			var env Envelope[TaskStatusData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			assert.Equal(t, "running", env.Data.Status)
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	})

	t.Run("TaskMessageCreated_WithSecrets", func(t *testing.T) {
		ev := events.TaskMessageCreated{
			ProjectID: projectID,
			TaskID:    taskID,
			Content:   "Here is my key: sk-12345678901234567890",
			Metadata: map[string]any{
				"model":       "gpt-4",
				"tokens_used": 100,
				"secret":      "ghp_123456789012345678901234567890123456", // should be filtered out by whitelist
			},
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.broadcast:
			var env Envelope[TaskMessageData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			
			assert.Contains(t, env.Data.Content, secrets.Redacted)
			assert.NotContains(t, env.Data.Content, "sk-1234567890")
			
			assert.Equal(t, "gpt-4", env.Data.Metadata["model"])
			assert.NotContains(t, env.Data.Metadata, "secret")
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	})

	t.Run("SandboxLogEmitted_Scrubbing", func(t *testing.T) {
		ev := events.SandboxLogEmitted{
			ProjectID: projectID,
			TaskID:    taskID,
			Line:      "Exporting GITHUB_TOKEN=ghp_123456789012345678901234567890123456",
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.broadcast:
			var env Envelope[AgentLogData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			assert.Contains(t, env.Data.Line, secrets.Redacted)
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	})

	t.Run("PipelineErrored_Validation", func(t *testing.T) {
		ev := events.PipelineErrored{
			ProjectID: projectID,
			Code:      "invalid_code",
			Message:   "Something went wrong",
		}
		bus.Publish(ctx, ev)

		select {
		case msg := <-hub.broadcast:
			var env Envelope[ErrorData]
			err := json.Unmarshal(msg.Payload, &env)
			require.NoError(t, err)
			assert.Equal(t, ErrorCodeInternalError, env.Data.Code)
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	})
}
