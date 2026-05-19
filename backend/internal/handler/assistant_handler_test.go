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
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testAssistantUserID    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testAssistantSessionID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

type MockAssistantService struct {
	mock.Mock
}

func (m *MockAssistantService) CreateSession(ctx context.Context, userID uuid.UUID) (*models.AssistantSession, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AssistantSession), args.Error(1)
}

func (m *MockAssistantService) ListSessions(ctx context.Context, userID uuid.UUID, includeArchived bool, limit int) ([]*models.AssistantSession, error) {
	args := m.Called(ctx, userID, includeArchived, limit)
	var sessions []*models.AssistantSession
	if args.Get(0) != nil {
		sessions = args.Get(0).([]*models.AssistantSession)
	}
	return sessions, args.Error(1)
}

func (m *MockAssistantService) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*models.AssistantSession, error) {
	args := m.Called(ctx, sessionID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AssistantSession), args.Error(1)
}

func (m *MockAssistantService) ArchiveSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	args := m.Called(ctx, sessionID, userID)
	return args.Error(0)
}

func (m *MockAssistantService) GetHistory(ctx context.Context, sessionID, userID uuid.UUID, limit int, beforeCreatedAt time.Time, beforeID uuid.UUID) ([]*models.AssistantMessage, error) {
	args := m.Called(ctx, sessionID, userID, limit, beforeCreatedAt, beforeID)
	var msgs []*models.AssistantMessage
	if args.Get(0) != nil {
		msgs = args.Get(0).([]*models.AssistantMessage)
	}
	return msgs, args.Error(1)
}

func (m *MockAssistantService) SendMessage(ctx context.Context, sessionID, userID uuid.UUID, content string, clientMsgID string) (*models.AssistantMessage, bool, error) {
	args := m.Called(ctx, sessionID, userID, content, clientMsgID)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.AssistantMessage), args.Bool(1), args.Error(2)
}

func (m *MockAssistantService) ConfirmToolCall(ctx context.Context, sessionID, userID uuid.UUID, toolCallID string, approved bool) error {
	args := m.Called(ctx, sessionID, userID, toolCallID, approved)
	return args.Error(0)
}

func (m *MockAssistantService) ListActiveTasks(ctx context.Context, userID uuid.UUID) ([]service.ActiveTaskSummary, error) {
	args := m.Called(ctx, userID)
	var tasks []service.ActiveTaskSummary
	if args.Get(0) != nil {
		tasks = args.Get(0).([]service.ActiveTaskSummary)
	}
	return tasks, args.Error(1)
}

func (m *MockAssistantService) GetStatus(ctx context.Context, userID uuid.UUID) (*dto.AssistantStatusResponse, error) {
	args := m.Called(ctx, userID)
	var resp *dto.AssistantStatusResponse
	if args.Get(0) != nil {
		resp = args.Get(0).(*dto.AssistantStatusResponse)
	}
	return resp, args.Error(1)
}

func (m *MockAssistantService) StartStaleRecovery(ctx context.Context) {
	m.Called(ctx)
}

func setupAssistantRouter(mockSvc *MockAssistantService, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAssistantHandler(mockSvc)

	authFn := func(c *gin.Context) {
		c.Set("userID", testAssistantUserID)
		c.Set("userRole", string(models.RoleUser))
		c.Next()
	}

	assistant := r.Group("/assistant")
	if withAuth {
		assistant.Use(authFn)
	}
	{
		assistant.GET("/active-tasks", h.ListActiveTasks)
		assistant.POST("/sessions", h.CreateSession)
		assistant.GET("/sessions", h.ListSessions)
		assistant.GET("/sessions/:id", h.GetSession)
		assistant.DELETE("/sessions/:id", h.ArchiveSession)
		assistant.GET("/sessions/:id/messages", h.GetMessages)
		assistant.POST("/sessions/:id/messages", h.SendMessage)
		assistant.POST("/sessions/:id/confirm", h.ConfirmToolCall)
	}
	return r
}

func TestAssistant_CreateSession_Success(t *testing.T) {
	mockSvc := new(MockAssistantService)
	sess := &models.AssistantSession{ID: testAssistantSessionID, UserID: testAssistantUserID, Status: models.AssistantSessionStatusActive}
	mockSvc.On("CreateSession", mock.Anything, testAssistantUserID).Return(sess, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions", nil)
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.AssistantSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, testAssistantSessionID.String(), got.ID.String())
}

func TestAssistant_ListSessions_WithLimit(t *testing.T) {
	mockSvc := new(MockAssistantService)
	mockSvc.On("ListSessions", mock.Anything, testAssistantUserID, true, 10).Return([]*models.AssistantSession{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assistant/sessions?include_archived=true&limit=10", nil)
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestAssistant_GetMessages_CursorParsing(t *testing.T) {
	mockSvc := new(MockAssistantService)
	beforeID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	beforeAt := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	
	mockSvc.On("GetHistory", mock.Anything, testAssistantSessionID, testAssistantUserID, 30, beforeAt, beforeID).Return([]*models.AssistantMessage{}, nil)

	w := httptest.NewRecorder()
	url := "/assistant/sessions/" + testAssistantSessionID.String() + "/messages?before_id=" + beforeID.String() + "&before_created_at=" + beforeAt.Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestAssistant_GetMessages_InvalidCursor(t *testing.T) {
	mockSvc := new(MockAssistantService)
	w := httptest.NewRecorder()
	url := "/assistant/sessions/" + testAssistantSessionID.String() + "/messages?before_id=bad-uuid&before_created_at=bad-time"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "GetHistory")
}

func TestAssistant_SendMessage_Success(t *testing.T) {
	mockSvc := new(MockAssistantService)
	msg := &models.AssistantMessage{ID: uuid.New(), Content: func(s string) *string { return &s }("hello")}
	mockSvc.On("SendMessage", mock.Anything, testAssistantSessionID, testAssistantUserID, "hello", "").
		Return(msg, false, nil)

	w := httptest.NewRecorder()
	body := `{"content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions/"+testAssistantSessionID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var got dto.SendAssistantMessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.False(t, got.Duplicate)
	assert.Equal(t, "hello", *got.Message.Content)
}

func TestAssistant_SendMessage_Duplicate(t *testing.T) {
	mockSvc := new(MockAssistantService)
	msg := &models.AssistantMessage{ID: uuid.New(), Content: func(s string) *string { return &s }("hello")}
	clientMsgID := uuid.New().String()
	mockSvc.On("SendMessage", mock.Anything, testAssistantSessionID, testAssistantUserID, "hello", clientMsgID).
		Return(msg, true, nil)

	w := httptest.NewRecorder()
	body := `{"content":"hello", "client_message_id":"` + clientMsgID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions/"+testAssistantSessionID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var got dto.SendAssistantMessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.True(t, got.Duplicate)
}

func TestAssistant_SendMessage_SessionBusy(t *testing.T) {
	mockSvc := new(MockAssistantService)
	mockSvc.On("SendMessage", mock.Anything, testAssistantSessionID, testAssistantUserID, "hello", "").
		Return((*models.AssistantMessage)(nil), false, service.ErrAssistantSessionBusy)

	w := httptest.NewRecorder()
	body := `{"content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions/"+testAssistantSessionID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "session_busy", er.Error)
}

func TestAssistant_ConfirmToolCall_Success(t *testing.T) {
	mockSvc := new(MockAssistantService)
	mockSvc.On("ConfirmToolCall", mock.Anything, testAssistantSessionID, testAssistantUserID, "call_123", true).
		Return(nil)

	w := httptest.NewRecorder()
	body := `{"tool_call_id":"call_123", "approved":true}`
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions/"+testAssistantSessionID.String()+"/confirm", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestAssistant_ConfirmToolCall_NoPending(t *testing.T) {
	mockSvc := new(MockAssistantService)
	mockSvc.On("ConfirmToolCall", mock.Anything, testAssistantSessionID, testAssistantUserID, "call_123", true).
		Return(service.ErrAssistantNoPendingConfirmation)

	w := httptest.NewRecorder()
	body := `{"tool_call_id":"call_123", "approved":true}`
	req := httptest.NewRequest(http.MethodPost, "/assistant/sessions/"+testAssistantSessionID.String()+"/confirm", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupAssistantRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, "no_pending_confirmation", er.Error)
}
