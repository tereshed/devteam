package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
)

// MockProvider is a mock implementation of llm.Provider
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.Response), args.Error(1)
}

// MockLLMRepository mocks the repository for logging
type MockLLMRepository struct {
	mock.Mock
}

func (m *MockLLMRepository) CreateLog(ctx context.Context, log *models.LLMLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockLLMRepository) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.LLMLog), args.Get(1).(int64), args.Error(2)
}

// MockLLMModelRepository mocks the repository for models
type MockLLMModelRepository struct {
	mock.Mock
}

func (m *MockLLMModelRepository) Upsert(ctx context.Context, models []models.LLMModel) error {
	args := m.Called(ctx, models)
	return args.Error(0)
}

func (m *MockLLMModelRepository) ListActive(ctx context.Context) ([]models.LLMModel, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.LLMModel), args.Error(1)
}

func (m *MockLLMModelRepository) ListAll(ctx context.Context) ([]models.LLMModel, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.LLMModel), args.Error(1)
}

func (m *MockLLMModelRepository) GetByID(ctx context.Context, id string) (*models.LLMModel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LLMModel), args.Error(1)
}

func TestLLMService_Generate(t *testing.T) {
	// We manually construct the llmService struct with mock providers to avoid complex factory mocking.

	mockOpenAI := new(MockProvider)
	mockAnthropic := new(MockProvider)
	mockRepo := new(MockLLMRepository)
	mockModelRepo := new(MockLLMModelRepository)

	service := &llmService{
		providers: map[llm.ProviderType]llm.Provider{
			llm.ProviderOpenAI:    mockOpenAI,
			llm.ProviderAnthropic: mockAnthropic,
		},
		defaultProvider: llm.ProviderOpenAI,
		defaultModels: map[llm.ProviderType]string{
			llm.ProviderOpenAI:    "gpt-4o",
			llm.ProviderAnthropic: "claude-3-5-sonnet-20240620",
		},
		repo:      mockRepo,
		modelRepo: mockModelRepo,
	}

	ctx := context.Background()

	// Since logging happens asynchronously, we need to handle mock expectations carefully.
	mockRepo.On("CreateLog", mock.Anything, mock.Anything).Return(nil)
	// We expect GetByID calls for pricing, but since it's async and might fail silently, we just mock it to return nil (not found) or error.
	mockModelRepo.On("GetByID", mock.Anything, mock.Anything).Return(nil, assert.AnError) // Simulate not found

	t.Run("Default Provider", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
		}
		expectedResp := &llm.Response{Content: "OpenAI Response"}
		mockOpenAI.On("Generate", ctx, req).Return(expectedResp, nil)

		resp, err := service.Generate(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
		mockOpenAI.AssertExpectations(t)
	})

	t.Run("Specific Provider", func(t *testing.T) {
		req := llm.Request{
			Provider: llm.ProviderAnthropic,
			Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
		}
		expectedResp := &llm.Response{Content: "Anthropic Response"}
		mockAnthropic.On("Generate", ctx, req).Return(expectedResp, nil)

		resp, err := service.Generate(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
		mockAnthropic.AssertExpectations(t)
	})

	t.Run("Unknown Provider", func(t *testing.T) {
		req := llm.Request{
			Provider: "unknown",
			Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
		}
		resp, err := service.Generate(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "provider unknown not configured")
	})
}
