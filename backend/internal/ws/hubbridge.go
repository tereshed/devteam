package ws

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/pkg/secrets"
	"github.com/google/uuid"
)

// HubBridge — подписчик на EventBus, который транслирует доменные события в WebSocket Hub.
type HubBridge struct {
	bus     events.EventBus
	hub     *Hub
	scrub   secrets.Scrubber
	log     *slog.Logger
	metrics BridgeMetrics
}

// BridgeMetrics — интерфейс для сбора метрик моста.
type BridgeMetrics interface {
	IncDispatched(msgType string)
	IncDispatchError(kind string)
}

type noopBridgeMetrics struct{}

func (m noopBridgeMetrics) IncDispatched(msgType string)    {}
func (m noopBridgeMetrics) IncDispatchError(kind string) {}

// NewHubBridge создает новый экземпляр HubBridge.
func NewHubBridge(bus events.EventBus, hub *Hub, scrub secrets.Scrubber, log *slog.Logger, metrics BridgeMetrics) *HubBridge {
	if metrics == nil {
		metrics = noopBridgeMetrics{}
	}
	return &HubBridge{
		bus:     bus,
		hub:     hub,
		scrub:   scrub,
		log:     log,
		metrics: metrics,
	}
}

// Run запускает цикл обработки событий. Блокируется до закрытия контекста.
func (b *HubBridge) Run(ctx context.Context) {
	ch, unsub := b.bus.Subscribe("hub_bridge", 256)
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b.dispatch(ev)
		}
	}
}

func (b *HubBridge) dispatch(ev events.DomainEvent) {
	// User-scoped события маршрутизируются в Hub.SendToUser, не в проектный канал.
	if e, ok := ev.(events.IntegrationConnectionChanged); ok {
		b.dispatchIntegrationConnectionChanged(e)
		return
	}

	var (
		payload []byte
		err     error
		msgType MessageType
	)

	projectID := ev.GetProjectID()

	switch e := ev.(type) {
	case events.TaskStatusChanged:
		msgType = MessageTypeTaskStatus
		data := TaskStatusData{
			TaskID:          e.TaskID,
			ParentTaskID:    e.ParentTaskID,
			PreviousStatus:  e.Previous,
			Status:          e.Current,
			AssignedAgentID: e.AssignedAgentID,
			AgentRole:       e.AgentRole,
			ErrorMessage:    e.ErrorMessage,
		}
		payload, err = MarshalTaskStatus(projectID, data)

	case events.TaskMessageCreated:
		msgType = MessageTypeTaskMessage
		// Sanitize content
		content := b.scrub.Scrub(e.Content)
		content = ScrubAndStripContent(content)

		// Sanitize metadata
		metadata, filterErr := ValidateAndFilterMetadata(e.Metadata)
		if filterErr != nil {
			b.log.Warn("failed to filter metadata", "error", filterErr, "project_id", projectID)
		}

		// Scrub string values in metadata
		if metadata != nil {
			for k, v := range metadata {
				if s, ok := v.(string); ok {
					metadata[k] = b.scrub.Scrub(s)
				}
			}
		}

		data := TaskMessageData{
			TaskID:      e.TaskID,
			MessageID:   e.MessageID,
			SenderType:  e.SenderType,
			SenderID:    e.SenderID,
			SenderRole:  e.SenderRole,
			MessageType: e.MessageType,
			Content:     content,
			Metadata:    metadata,
		}
		payload, err = MarshalTaskMessage(projectID, data)

	case events.SandboxLogEmitted:
		msgType = MessageTypeAgentLog
		stream := "stdout"
		if e.Stderr {
			stream = "stderr"
		}
		data := AgentLogData{
			TaskID:    e.TaskID,
			SandboxID: e.SandboxID,
			Stream:    stream,
			Line:      b.scrub.Scrub(e.Line),
			Seq:       e.Seq,
			Truncated: e.Truncated,
		}
		payload, err = MarshalAgentLog(projectID, data)

	case events.PipelineErrored:
		msgType = MessageTypeError
		code := ErrorCode(e.Code)
		// Validate error code
		switch code {
		case ErrorCodeStreamOverflow, ErrorCodeTaskNotFound, ErrorCodeInternalError, ErrorCodeForbidden, ErrorCodeServerShutdown:
			// OK
		default:
			b.log.Warn("unknown error code in PipelineErrored, falling back to internal_error", "code", e.Code)
			code = ErrorCodeInternalError
		}

		data := ErrorData{
			Code:    code,
			Message: b.scrub.Scrub(e.Message),
		}
		payload, err = MarshalError(projectID, data)

	default:
		b.log.Warn("unknown domain event type", "type", fmt.Sprintf("%T", ev))
		b.metrics.IncDispatchError("unknown_event_type")
		return
	}

	if err != nil {
		b.log.Error("failed to marshal ws envelope", "error", err, "type", msgType)
		b.metrics.IncDispatchError("marshal_error")
		return
	}

	if err := b.hub.SendToProject(projectID.String(), string(msgType), payload); err != nil {
		b.log.Error("failed to send message to hub", "error", err, "project_id", projectID)
		b.metrics.IncDispatchError("hub_send_error")
		return
	}

	b.metrics.IncDispatched(string(msgType))
}

func (b *HubBridge) dispatchIntegrationConnectionChanged(e events.IntegrationConnectionChanged) {
	const msgType = MessageTypeIntegrationStatus

	if e.UserID == (uuid.UUID{}) {
		b.log.Warn("integration_connection_changed dropped: nil UserID", "provider", e.Provider)
		b.metrics.IncDispatchError("nil_user_id")
		return
	}
	if !events.IsValidIntegrationConnectionStatus(e.Status) {
		b.log.Warn("integration_connection_changed dropped: invalid status",
			"user_id", e.UserID, "provider", e.Provider, "status", e.Status)
		b.metrics.IncDispatchError("invalid_status")
		return
	}

	data := IntegrationStatusData{
		Provider:    e.Provider,
		Status:      string(e.Status),
		Reason:      e.Reason,
		ConnectedAt: e.ConnectedAt,
		ExpiresAt:   e.ExpiresAt,
	}
	payload, err := MarshalIntegrationStatus(e.UserID, data)
	if err != nil {
		b.log.Error("failed to marshal integration_status envelope", "error", err)
		b.metrics.IncDispatchError("marshal_error")
		return
	}

	if err := b.hub.SendToUser(e.UserID.String(), string(msgType), payload); err != nil {
		b.log.Error("failed to send integration_status to hub", "error", err, "user_id", e.UserID)
		b.metrics.IncDispatchError("hub_send_error")
		return
	}

	b.metrics.IncDispatched(string(msgType))
}
