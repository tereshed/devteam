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
	"github.com/wibe-flutter-gin-template/backend/internal/handler/dto"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/service"
)

// MockPromptService mocks PromptService
type MockPromptService struct {
	mock.Mock
}

func (m *MockPromptService) Create(ctx context.Context, req dto.CreatePromptRequest) (*models.Prompt, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptService) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptService) GetByName(ctx context.Context, name string) (*models.Prompt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptService) List(ctx context.Context) ([]models.Prompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Prompt), args.Error(1)
}

func (m *MockPromptService) Update(ctx context.Context, id uuid.UUID, req dto.UpdatePromptRequest) (*models.Prompt, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptService) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestPromptHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		input          dto.CreatePromptRequest
		mockSetup      func(*MockPromptService)
		expectedStatus int
	}{
		{
			name: "success",
			input: dto.CreatePromptRequest{
				Name:     "test",
				Template: "template",
			},
			mockSetup: func(s *MockPromptService) {
				s.On("Create", mock.Anything, mock.Anything).Return(&models.Prompt{ID: uuid.New(), Name: "test"}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "conflict",
			input: dto.CreatePromptRequest{
				Name:     "exists",
				Template: "template",
			},
			mockSetup: func(s *MockPromptService) {
				s.On("Create", mock.Anything, mock.Anything).Return(nil, service.ErrPromptAlreadyExists)
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockPromptService)
			tt.mockSetup(mockService)
			handler := NewPromptHandler(mockService)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/prompts", handler.Create)

			body, _ := json.Marshal(tt.input)
			req, _ := http.NewRequest("POST", "/prompts", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestPromptHandler_GetByID(t *testing.T) {
	mockService := new(MockPromptService)
	handler := NewPromptHandler(mockService)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/prompts/:id", handler.GetByID)

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		mockService.On("GetByID", mock.Anything, id).Return(&models.Prompt{ID: id}, nil)

		req, _ := http.NewRequest("GET", "/prompts/"+id.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		id := uuid.New()
		mockService.On("GetByID", mock.Anything, id).Return(nil, service.ErrPromptNotFound)

		req, _ := http.NewRequest("GET", "/prompts/"+id.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

