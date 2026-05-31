package ws

import (
	"context"
	"encoding/json"
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

func (m noopBridgeMetrics) IncDispatched(msgType string) {}
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
		// User-scoped fan-out для Tasks-tab правой панели ассистента (Sprint 21 §7).
		// Делаем до возможного `return` ниже, чтобы ошибка маршала project-конверта
		// не блокировала assistant.task_update — это разные подписчики.
		b.fanOutAssistantTaskUpdate(e)

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

	case events.RouterDecisionCreated:
		msgType = MessageTypeRouterDecision
		data := RouterDecisionData{
			TaskID:       e.TaskID,
			StepNo:       e.StepNo,
			ChosenAgents: e.ChosenAgents,
			Done:         e.Done,
			Outcome:      e.Outcome,
			Reason:       b.scrub.Scrub(e.Reason),
		}
		payload, err = MarshalRouterDecision(projectID, data)

	case events.ArtifactCreated:
		msgType = MessageTypeArtifact
		data := ArtifactData{
			TaskID:        e.TaskID,
			ProducerAgent: e.ProducerAgent,
			Kind:          e.Kind,
			Status:        e.Status,
			Summary:       b.scrub.Scrub(e.Summary),
			ParentID:      e.ParentID,
		}
		if e.ArtifactID != (uuid.UUID{}) {
			id := e.ArtifactID
			data.ArtifactID = &id
		}
		payload, err = MarshalArtifact(projectID, data)

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

	case events.ConversationMessageCreated:
		msgType = MessageTypeConversationMessage
		content := b.scrub.Scrub(e.Content)
		content = ScrubAndStripContent(content)
		var meta json.RawMessage
		if e.Metadata != "" {
			meta = json.RawMessage(e.Metadata)
		}
		data := ConversationMessageData{
			ID:             e.MessageID,
			ConversationID: e.ConversationID,
			Role:           e.Role,
			Content:        content,
			LinkedTaskIDs:  e.LinkedTaskIDs,
			Metadata:       meta,
			CreatedAt:      e.CreatedAt,
		}
		payload, err = MarshalConversationMessage(projectID, data)

	case events.ConversationMessageUpdated:
		msgType = MessageTypeConversationMessage
		content := b.scrub.Scrub(e.Content)
		content = ScrubAndStripContent(content)
		var meta json.RawMessage
		if e.Metadata != "" {
			meta = json.RawMessage(e.Metadata)
		}
		data := ConversationMessageData{
			ID:             e.MessageID,
			ConversationID: e.ConversationID,
			Role:           e.Role,
			Content:        content,
			LinkedTaskIDs:  e.LinkedTaskIDs,
			Metadata:       meta,
			CreatedAt:      e.CreatedAt,
		}
		payload, err = MarshalConversationMessage(projectID, data)

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

// fanOutAssistantTaskUpdate шлёт user-scoped дубль TaskStatusChanged-события
// под типом assistant.task_update — для Tasks-tab правой панели (Sprint 21 §7).
//
// Контракт: project-scoped доставка (MessageTypeTaskStatus) идёт независимо
// и НЕ должна страдать от ошибок этого fan-out'а. Если продюсер не успел
// разрезолвить user_id (например, при отказе projectRepo) — событие просто
// дропается с пометкой в метриках; на корректность task_status это не влияет.
func (b *HubBridge) fanOutAssistantTaskUpdate(e events.TaskStatusChanged) {
	if e.UserID == (uuid.UUID{}) {
		b.metrics.IncDispatchError("assistant_task_update_nil_user_id")
		return
	}

	data := AssistantTaskUpdateData{
		ProjectID: e.ProjectID,
		TaskID:    e.TaskID,
		State:     e.Current,
		Title:     e.Title,
		UpdatedAt: e.OccurredAt,
	}
	payload, err := MarshalAssistantTaskUpdate(e.UserID, data)
	if err != nil {
		b.log.Error("failed to marshal assistant.task_update envelope",
			"error", err, "user_id", e.UserID, "task_id", e.TaskID)
		b.metrics.IncDispatchError("marshal_error")
		return
	}
	if err := b.hub.SendToUser(e.UserID.String(), string(MessageTypeAssistantTaskUpdate), payload); err != nil {
		b.log.Error("failed to send assistant.task_update to hub",
			"error", err, "user_id", e.UserID, "task_id", e.TaskID)
		b.metrics.IncDispatchError("hub_send_error")
		return
	}
	b.metrics.IncDispatched(string(MessageTypeAssistantTaskUpdate))
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
