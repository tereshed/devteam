package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
)

// MockLLMService is a mock implementation of service.LLMService
type MockLLMService struct {
	mock.Mock
}

func (m *MockLLMService) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.Response), args.Error(1)
}

func (m *MockLLMService) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.LLMLog), args.Get(1).(int64), args.Error(2)
}

func TestLLMHandler_Chat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockLLMService)
	handler := NewLLMHandler(mockService)

	router := gin.New()
	router.POST("/chat", handler.Chat)

	t.Run("Success", func(t *testing.T) {
		reqBody := llm.Request{
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "Hello"},
			},
		}
		jsonBody, _ := json.Marshal(reqBody)

		expectedResp := &llm.Response{
			Content: "Hi there!",
		}
		mockService.On("Generate", mock.Anything, reqBody).Return(expectedResp, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/chat", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp llm.Response
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, expectedResp.Content, resp.Content)
	})

	t.Run("Invalid Request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/chat", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestLLMHandler_ListLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := new(MockLLMService)
	h := NewLLMHandler(mockService)

	logs := []models.LLMLog{{ID: uuid.New()}}
	mockService.On("ListLogs", mock.Anything, 50, 0).Return(logs, int64(1), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/llm/logs", nil)

	h.ListLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}
