package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// CreateConversationRequest запрос на создание чата
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// SendMessageRequest запрос на отправку сообщения
type SendMessageRequest struct {
	Content string `json:"content"`
}

// ConversationResponse ответ с данными чата
type ConversationResponse struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MessageResponse ответ с данными сообщения
type MessageResponse struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	LinkedTaskIDs  []uuid.UUID    `json:"linked_task_ids,omitempty"`
	Metadata       datatypes.JSON `json:"metadata,omitempty" swaggertype:"string"`
	CreatedAt      time.Time      `json:"created_at"`
}

// ConversationListResponse пагинированный список чатов
type ConversationListResponse struct {
	Conversations []ConversationResponse `json:"conversations"`
	Total         int64                  `json:"total"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
	HasNext       bool                   `json:"has_next"`
}

// MessageListResponse пагинированный список сообщений
type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
	HasNext  bool              `json:"has_next"`
}

// ToConversationResponse маппинг models.Conversation → ConversationResponse
func ToConversationResponse(c *models.Conversation) ConversationResponse {
	if c == nil {
		return ConversationResponse{}
	}
	return ConversationResponse{
		ID:        c.ID,
		ProjectID: c.ProjectID,
		Title:     c.Title,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// ToMessageResponse маппинг models.ConversationMessage → MessageResponse
func ToMessageResponse(m *models.ConversationMessage) MessageResponse {
	if m == nil {
		return MessageResponse{}
	}
	return MessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		Role:           string(m.Role),
		Content:        m.Content,
		LinkedTaskIDs:  []uuid.UUID(m.LinkedTaskIDs),
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

// ToConversationListResponse обёртка списка чатов
func ToConversationListResponse(conversations []*models.Conversation, total int64, limit, offset int) ConversationListResponse {
	out := make([]ConversationResponse, 0, len(conversations))
	for _, c := range conversations {
		out = append(out, ToConversationResponse(c))
	}
	return ConversationListResponse{
		Conversations: out,
		Total:         total,
		Limit:         limit,
		Offset:        offset,
		HasNext:       int64(offset+limit) < total,
	}
}

// ToMessageListResponse обёртка списка сообщений
func ToMessageListResponse(messages []*models.ConversationMessage, total int64, limit, offset int) MessageListResponse {
	out := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		out = append(out, ToMessageResponse(m))
	}
	return MessageListResponse{
		Messages: out,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		HasNext:  int64(offset+limit) < total,
	}
}
