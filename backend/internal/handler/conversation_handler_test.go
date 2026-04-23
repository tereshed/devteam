package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testConvUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

type MockConversationService struct {
	mock.Mock
}

func (m *MockConversationService) CreateConversation(ctx context.Context, userID, projectID uuid.UUID, title string) (*models.Conversation, error) {
	args := m.Called(ctx, userID, projectID, title)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationService) GetConversation(ctx context.Context, userID, id uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, userID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationService) ListConversations(ctx context.Context, userID, projectID uuid.UUID, limit, offset int) ([]*models.Conversation, int64, error) {
	args := m.Called(ctx, userID, projectID, limit, offset)
	var convs []*models.Conversation
	if args.Get(0) != nil {
		convs = args.Get(0).([]*models.Conversation)
	}
	return convs, args.Get(1).(int64), args.Error(2)
}

func (m *MockConversationService) SendMessage(ctx context.Context, userID, conversationID uuid.UUID, content string, clientMsgID uuid.UUID) (*models.ConversationMessage, error) {
	args := m.Called(ctx, userID, conversationID, content, clientMsgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationMessage), args.Error(1)
}

func (m *MockConversationService) GetHistory(ctx context.Context, userID, conversationID uuid.UUID, limit, offset int) ([]*models.ConversationMessage, int64, error) {
	args := m.Called(ctx, userID, conversationID, limit, offset)
	var msgs []*models.ConversationMessage
	if args.Get(0) != nil {
		msgs = args.Get(0).([]*models.ConversationMessage)
	}
	return msgs, args.Get(1).(int64), args.Error(2)
}

func (m *MockConversationService) DeleteConversation(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}

func (m *MockConversationService) DeleteMessage(ctx context.Context, userID, conversationID, messageID uuid.UUID) error {
	args := m.Called(ctx, userID, conversationID, messageID)
	return args.Error(0)
}

func (m *MockConversationService) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func setupConversationRouter(mockSvc *MockConversationService, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewConversationHandler(mockSvc)

	if withAuth {
		r.Use(func(c *gin.Context) {
			c.Set("userID", testConvUserID)
			c.Next()
		})
	}

	r.POST("/api/v1/projects/:project_id/conversations", h.Create)
	r.GET("/api/v1/projects/:project_id/conversations", h.List)
	r.GET("/api/v1/conversations/:id", h.GetByID)
	r.POST("/api/v1/conversations/:id/messages", h.SendMessage)
	r.GET("/api/v1/conversations/:id/messages", h.GetHistory)
	r.DELETE("/api/v1/conversations/:id", h.Delete)
	return r
}

func sampleConversation() *models.Conversation {
	id := uuid.New()
	projectID := uuid.New()
	return &models.Conversation{
		ID:        id,
		ProjectID: projectID,
		UserID:    testConvUserID,
		Title:     "Test Conversation",
		Status:    models.ConversationStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func sampleMessage(convID uuid.UUID) *models.ConversationMessage {
	return &models.ConversationMessage{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           models.ConversationRoleUser,
		Content:        "Hello",
		CreatedAt:      time.Now(),
	}
}

func TestConversation_Create_Success(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	mockSvc.On("CreateConversation", mock.Anything, testConvUserID, conv.ProjectID, "Test Conversation").
		Return(conv, nil)

	w := httptest.NewRecorder()
	body := `{"title":"Test Conversation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+conv.ProjectID.String()+"/conversations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.ConversationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, conv.ID, got.ID)
	mockSvc.AssertExpectations(t)
}

func TestConversation_Create_InvalidProjectID(t *testing.T) {
	mockSvc := new(MockConversationService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/invalid-uuid/conversations", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConversation_SendMessage_Success(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	msg := sampleMessage(conv.ID)
	clientMsgID := uuid.New()
	mockSvc.On("SendMessage", mock.Anything, testConvUserID, conv.ID, "Hello", clientMsgID).
		Return(msg, nil)

	w := httptest.NewRecorder()
	body := `{"content":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Message-ID", clientMsgID.String())
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, msg.ID, got.ID)
	mockSvc.AssertExpectations(t)
}

func TestConversation_SendMessage_MissingXClientMessageID(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID.String()+"/messages", bytes.NewBufferString(`{"content":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConversation_SendMessage_Idempotency(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	msg := sampleMessage(conv.ID)
	clientMsgID := uuid.New()
	mockSvc.On("SendMessage", mock.Anything, testConvUserID, conv.ID, "Hello", clientMsgID).
		Return(msg, service.ErrDuplicateMessage)

	w := httptest.NewRecorder()
	body := `{"content":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Message-ID", clientMsgID.String())
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.MessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, msg.ID, got.ID)
	mockSvc.AssertExpectations(t)
}

func TestConversation_SendMessage_InvalidXClientMessageID(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID.String()+"/messages", bytes.NewBufferString(`{"content":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Message-ID", "not-uuid")
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConversation_List_Success(t *testing.T) {
	mockSvc := new(MockConversationService)
	conv := sampleConversation()
	mockSvc.On("ListConversations", mock.Anything, testConvUserID, conv.ProjectID, 10, 0).
		Return([]*models.Conversation{conv}, int64(1), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+conv.ProjectID.String()+"/conversations?limit=10", nil)
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.ConversationListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Conversations, 1)
	assert.Equal(t, int64(1), got.Total)
	mockSvc.AssertExpectations(t)
}

func TestConversation_Delete_Success(t *testing.T) {
	mockSvc := new(MockConversationService)
	id := uuid.New()
	mockSvc.On("DeleteConversation", mock.Anything, testConvUserID, id).
		Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+id.String(), nil)
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestConversation_GetByID_NotFound(t *testing.T) {
	mockSvc := new(MockConversationService)
	id := uuid.New()
	mockSvc.On("GetConversation", mock.Anything, testConvUserID, id).
		Return(nil, service.ErrConversationNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+id.String(), nil)
	setupConversationRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockSvc.AssertExpectations(t)
}
