package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MockPromptRepository mocks PromptRepository
type MockPromptRepository struct {
	mock.Mock
}

func (m *MockPromptRepository) Create(ctx context.Context, prompt *models.Prompt) error {
	args := m.Called(ctx, prompt)
	return args.Error(0)
}

func (m *MockPromptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptRepository) GetByName(ctx context.Context, name string) (*models.Prompt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *MockPromptRepository) List(ctx context.Context) ([]models.Prompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Prompt), args.Error(1)
}

func (m *MockPromptRepository) Update(ctx context.Context, prompt *models.Prompt) error {
	args := m.Called(ctx, prompt)
	return args.Error(0)
}

func (m *MockPromptRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPromptRepository) Upsert(ctx context.Context, prompt *models.Prompt) error {
	args := m.Called(ctx, prompt)
	return args.Error(0)
}

func TestPromptService_Create(t *testing.T) {
	tests := []struct {
		name          string
		input         dto.CreatePromptRequest
		mockSetup     func(*MockPromptRepository)
		expectedError error
	}{
		{
			name: "successful create",
			input: dto.CreatePromptRequest{
				Name:     "new_prompt",
				Template: "Hello",
			},
			mockSetup: func(repo *MockPromptRepository) {
				repo.On("GetByName", mock.Anything, "new_prompt").Return(nil, gorm.ErrRecordNotFound)
				repo.On("Create", mock.Anything, mock.MatchedBy(func(p *models.Prompt) bool {
					return p.Name == "new_prompt"
				})).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "successful create with json schema",
			input: dto.CreatePromptRequest{
				Name:       "structured_prompt",
				Template:   "Generate json",
				JSONSchema: datatypes.JSON(`{"type": "object", "properties": {"foo": {"type": "string"}}}`),
			},
			mockSetup: func(repo *MockPromptRepository) {
				repo.On("GetByName", mock.Anything, "structured_prompt").Return(nil, gorm.ErrRecordNotFound)
				repo.On("Create", mock.Anything, mock.MatchedBy(func(p *models.Prompt) bool {
					return p.Name == "structured_prompt" && len(p.JSONSchema) > 0
				})).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "already exists",
			input: dto.CreatePromptRequest{
				Name:     "existing_prompt",
				Template: "Hello",
			},
			mockSetup: func(repo *MockPromptRepository) {
				existing := &models.Prompt{Name: "existing_prompt"}
				repo.On("GetByName", mock.Anything, "existing_prompt").Return(existing, nil)
			},
			expectedError: ErrPromptAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockPromptRepository)
			tt.mockSetup(repo)
			service := NewPromptService(repo)

			_, err := service.Create(context.Background(), tt.input)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
			repo.AssertExpectations(t)
		})
	}
}

func TestPromptService_GetByName(t *testing.T) {
	repo := new(MockPromptRepository)
	service := NewPromptService(repo)

	t.Run("found", func(t *testing.T) {
		prompt := &models.Prompt{Name: "test"}
		repo.On("GetByName", mock.Anything, "test").Return(prompt, nil)
		result, err := service.GetByName(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, "test", result.Name)
	})

	t.Run("not found", func(t *testing.T) {
		repo.On("GetByName", mock.Anything, "missing").Return(nil, gorm.ErrRecordNotFound)
		_, err := service.GetByName(context.Background(), "missing")
		assert.ErrorIs(t, err, ErrPromptNotFound)
	})
}
