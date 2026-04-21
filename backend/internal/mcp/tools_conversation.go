package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ConversationListParams — параметры conversation_list.
type ConversationListParams struct {
	ProjectID string `json:"project_id" jsonschema:"description=UUID проекта,required"`
	Limit     *int   `json:"limit,omitempty" jsonschema:"description=Лимит (1–100; по умолчанию 20)"`
	Offset    *int   `json:"offset,omitempty" jsonschema:"description=Смещение"`
}

// ConversationGetParams — параметры conversation_get.
type ConversationGetParams struct {
	ConversationID string `json:"conversation_id" jsonschema:"description=UUID чата,required"`
}

// ConversationCreateParams — параметры conversation_create.
type ConversationCreateParams struct {
	ProjectID string `json:"project_id" jsonschema:"description=UUID проекта,required"`
	Title     string `json:"title" jsonschema:"required,description=Название чата"`
}

// MessageSendParams — параметры conversation_send_message.
type MessageSendParams struct {
	ConversationID string `json:"conversation_id" jsonschema:"description=UUID чата,required"`
	Content        string `json:"content" jsonschema:"required,description=Текст сообщения"`
}

// MessageHistoryParams — параметры conversation_history.
type MessageHistoryParams struct {
	ConversationID string `json:"conversation_id" jsonschema:"description=UUID чата,required"`
	Limit          *int   `json:"limit,omitempty" jsonschema:"description=Лимит (1–100; по умолчанию 20)"`
	Offset         *int   `json:"offset,omitempty" jsonschema:"description=Смещение"`
}

// RegisterConversationTools регистрирует MCP-инструменты для чатов.
func RegisterConversationTools(server *mcp.Server, convSvc service.ConversationService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "conversation_list",
		Description: "Список чатов проекта. Пагинация как в API GET /projects/:project_id/conversations.",
	}, makeConversationListHandler(convSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "conversation_get",
		Description: "Получить детали чата по UUID. Как GET /conversations/:id.",
	}, makeConversationGetHandler(convSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "conversation_create",
		Description: "Создать новый чат в проекте. Как POST /projects/:project_id/conversations.",
	}, makeConversationCreateHandler(convSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "conversation_send_message",
		Description: "Отправить сообщение в чат. Автоматически генерирует X-Client-Message-ID. Как POST /conversations/:id/messages.",
	}, makeMessageSendHandler(convSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "conversation_history",
		Description: "История сообщений чата. Как GET /conversations/:id/messages.",
	}, makeMessageHistoryHandler(convSvc))
}

func normalizePagination(limit, offset *int) (int, int) {
	l, o := 20, 0
	if limit != nil {
		l = *limit
		if l <= 0 {
			l = 20
		}
		if l > 100 {
			l = 100
		}
	}
	if offset != nil {
		o = *offset
		if o < 0 {
			o = 0
		}
	}
	return l, o
}

func makeConversationListHandler(convSvc service.ConversationService) func(ctx context.Context, req *mcp.CallToolRequest, params *ConversationListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ConversationListParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" {
			return ValidationErr("project_id is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		pid, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}

		limit, offset := normalizePagination(params.Limit, params.Offset)
		conversations, total, err := convSvc.ListConversations(ctx, uid, pid, limit, offset)
		if err != nil {
			return conversationServiceMCPError(err)
		}

		data := dto.ToConversationListResponse(conversations, total, limit, offset)
		return OK(fmt.Sprintf("found %d conversations", len(data.Conversations)), data)
	}
}

func makeConversationGetHandler(convSvc service.ConversationService) func(ctx context.Context, req *mcp.CallToolRequest, params *ConversationGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ConversationGetParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ConversationID == "" {
			return ValidationErr("conversation_id is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		cid, err := uuid.Parse(params.ConversationID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid conversation_id: %q", params.ConversationID))
		}

		conv, err := convSvc.GetConversation(ctx, uid, cid)
		if err != nil {
			return conversationServiceMCPError(err)
		}

		data := dto.ToConversationResponse(conv)
		return OK(fmt.Sprintf("conversation %q (%s)", data.Title, data.ID), data)
	}
}

func makeConversationCreateHandler(convSvc service.ConversationService) func(ctx context.Context, req *mcp.CallToolRequest, params *ConversationCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ConversationCreateParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ProjectID == "" || params.Title == "" {
			return ValidationErr("project_id and title are required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		pid, err := uuid.Parse(params.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %q", params.ProjectID))
		}

		conv, err := convSvc.CreateConversation(ctx, uid, pid, params.Title)
		if err != nil {
			return conversationServiceMCPError(err)
		}

		data := dto.ToConversationResponse(conv)
		return OK(fmt.Sprintf("conversation %q created (id: %s)", data.Title, data.ID), data)
	}
}

func makeMessageSendHandler(convSvc service.ConversationService) func(ctx context.Context, req *mcp.CallToolRequest, params *MessageSendParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *MessageSendParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ConversationID == "" || params.Content == "" {
			return ValidationErr("conversation_id and content are required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		cid, err := uuid.Parse(params.ConversationID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid conversation_id: %q", params.ConversationID))
		}

		// Генерируем UUID для идемпотентности
		clientMsgID := uuid.New()

		msg, err := convSvc.SendMessage(ctx, uid, cid, params.Content, clientMsgID)
		if err != nil {
			return conversationServiceMCPError(err)
		}

		data := dto.ToMessageResponse(msg)
		return OK(fmt.Sprintf("message sent (id: %s)", data.ID), data)
	}
}

func makeMessageHistoryHandler(convSvc service.ConversationService) func(ctx context.Context, req *mcp.CallToolRequest, params *MessageHistoryParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *MessageHistoryParams) (*mcp.CallToolResult, any, error) {
		if params == nil || params.ConversationID == "" {
			return ValidationErr("conversation_id is required")
		}
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		cid, err := uuid.Parse(params.ConversationID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid conversation_id: %q", params.ConversationID))
		}

		limit, offset := normalizePagination(params.Limit, params.Offset)
		messages, total, err := convSvc.GetHistory(ctx, uid, cid, limit, offset)
		if err != nil {
			return conversationServiceMCPError(err)
		}

		data := dto.ToMessageListResponse(messages, total, limit, offset)
		return OK(fmt.Sprintf("found %d messages", len(data.Messages)), data)
	}
}

func conversationServiceMCPError(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrConversationNotFound):
		return Err("conversation not found", err)
	case errors.Is(err, service.ErrConversationForbidden):
		return Err("access to conversation denied", err)
	case errors.Is(err, service.ErrInvalidConversationTitle),
		errors.Is(err, service.ErrInvalidMessageContent):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrMessageRateLimit):
		return Err("rate limit exceeded", err)
	default:
		return Err("conversation operation failed", err)
	}
}
