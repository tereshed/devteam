package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

type stubUserLlmCredentialService struct {
	service.UserLlmCredentialService
	key string
	err error
}

func (s *stubUserLlmCredentialService) GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error) {
	return s.key, s.err
}

type stubClaudeCodeAuthService struct {
	service.ClaudeCodeAuthService
	token string
	err   error
}

func (s *stubClaudeCodeAuthService) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.token, s.err
}

type stubAntigravityAuthService struct {
	service.AntigravityAuthService
	token string
	err   error
}

func (s *stubAntigravityAuthService) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.token, s.err
}

func TestLLMHandler_Chat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockLLMService)
	handler := NewLLMHandler(
		mockService,
		&stubUserLlmCredentialService{},
		&stubClaudeCodeAuthService{},
		&stubAntigravityAuthService{},
		nil,
	)

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
	h := NewLLMHandler(
		mockService,
		&stubUserLlmCredentialService{},
		&stubClaudeCodeAuthService{},
		&stubAntigravityAuthService{},
		nil,
	)

	logs := []models.LLMLog{{ID: uuid.New()}}
	mockService.On("ListLogs", mock.Anything, 50, 0).Return(logs, int64(1), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/llm/logs", nil)

	h.ListLogs(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestLLMHandler_ListModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockLLMService)
	stubCreds := &stubUserLlmCredentialService{key: "test-key"}
	stubClaude := &stubClaudeCodeAuthService{token: "claude-token"}
	stubAntigravity := &stubAntigravityAuthService{token: "antigravity-token"}
	h := NewLLMHandler(mockService, stubCreds, stubClaude, stubAntigravity, nil)

	t.Run("Missing Provider Parameter", func(t *testing.T) {
		router := gin.New()
		router.GET("/llm/models", h.ListModels)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm/models", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Unknown Provider Parameter", func(t *testing.T) {
		router := gin.New()
		router.GET("/llm/models", h.ListModels)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm/models?provider=invalid", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Unauthorized - Returns Fallback", func(t *testing.T) {
		router := gin.New()
		router.GET("/llm/models", h.ListModels)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm/models?provider=anthropic", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var models []string
		err := json.Unmarshal(w.Body.Bytes(), &models)
		assert.NoError(t, err)
		assert.Contains(t, models, "claude-3-5-sonnet-latest")
	})

	t.Run("Authorized But HTTP Fails - Returns Fallback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm/models?provider=anthropic", nil)

		router := gin.New()
		router.GET("/llm/models", func(c *gin.Context) {
			c.Set("userID", uuid.New())
			h.ListModels(c)
		})

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var models []string
		err := json.Unmarshal(w.Body.Bytes(), &models)
		assert.NoError(t, err)
		assert.Contains(t, models, "claude-3-5-sonnet-latest")
	})

	t.Run("Fallback to System Config", func(t *testing.T) {
		router := gin.New()
		cfg := &config.Config{}
		cfg.LLM.Anthropic.APIKey = "system-key"
		hWithCfg := NewLLMHandler(mockService, &stubUserLlmCredentialService{key: ""}, stubClaude, stubAntigravity, cfg)

		router.GET("/llm/models", func(c *gin.Context) {
			c.Set("userID", uuid.New())
			hWithCfg.ListModels(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm/models?provider=anthropic", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var models []string
		err := json.Unmarshal(w.Body.Bytes(), &models)
		assert.NoError(t, err)
		assert.Contains(t, models, "claude-3-5-sonnet-latest")
	})
}
