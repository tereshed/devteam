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
