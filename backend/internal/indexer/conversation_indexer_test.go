package indexer

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// MockConversationRepo is a mock of ConversationRepository
type MockConversationRepo struct {
	mock.Mock
}

func (m *MockConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository {
	return m
}

func (m *MockConversationRepo) Create(ctx context.Context, conv *models.Conversation) error {
	args := m.Called(ctx, conv)
	return args.Error(0)
}

func (m *MockConversationRepo) GetByID(ctx context.Context, projectID, id uuid.UUID, master bool) (*models.Conversation, error) {
	args := m.Called(ctx, projectID, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepo) GetOnlyByID(ctx context.Context, id uuid.UUID, master bool) (*models.Conversation, error) {
	args := m.Called(ctx, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID, filter repository.ConversationFilter) ([]*models.Conversation, int64, error) {
	args := m.Called(ctx, projectID, filter)
	return args.Get(0).([]*models.Conversation), args.Get(1).(int64), args.Error(2)
}

func (m *MockConversationRepo) Update(ctx context.Context, projectID, id uuid.UUID, updates map[string]interface{}) error {
	args := m.Called(ctx, projectID, id, updates)
	return args.Error(0)
}

func (m *MockConversationRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	args := m.Called(ctx, projectID, id)
	return args.Error(0)
}

// MockConversationMessageRepo is a mock of ConversationMessageRepository
type MockConversationMessageRepo struct {
	mock.Mock
}

func (m *MockConversationMessageRepo) WithTx(tx *gorm.DB) repository.ConversationMessageRepository {
	return m
}

func (m *MockConversationMessageRepo) Create(ctx context.Context, msg *models.ConversationMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockConversationMessageRepo) GetByID(ctx context.Context, conversationID, id uuid.UUID, master bool) (*models.ConversationMessage, error) {
	args := m.Called(ctx, conversationID, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationMessage), args.Error(1)
}

func (m *MockConversationMessageRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID, filter repository.MessageFilter) ([]*models.ConversationMessage, int64, error) {
	args := m.Called(ctx, conversationID, filter)
	return args.Get(0).([]*models.ConversationMessage), args.Get(1).(int64), args.Error(2)
}

func (m *MockConversationMessageRepo) Update(ctx context.Context, conversationID, id uuid.UUID, updates map[string]interface{}) error {
	args := m.Called(ctx, conversationID, id, updates)
	return args.Error(0)
}

func (m *MockConversationMessageRepo) Delete(ctx context.Context, conversationID, id uuid.UUID) error {
	args := m.Called(ctx, conversationID, id)
	return args.Error(0)
}

func (m *MockConversationMessageRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID, lastID *uuid.UUID, limit int, master bool) ([]*models.ConversationMessage, error) {
	args := m.Called(ctx, projectID, lastID, limit, master)
	return args.Get(0).([]*models.ConversationMessage), args.Error(1)
}

// MockEventBus is a mock of EventBus
type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, ev events.DomainEvent) {
	m.Called(ctx, ev)
}

func (m *MockEventBus) Subscribe(name string, buffer int) (<-chan events.DomainEvent, func()) {
	args := m.Called(name, buffer)
	return args.Get(0).(<-chan events.DomainEvent), args.Get(1).(func())
}

func TestConversationIndexer_FormatChunk(t *testing.T) {
	idx, _ := NewConversationIndexer(nil, nil, nil, nil, nil)
	ci := idx.(*conversationIndexer)

	t.Run("Assistant with user prompt", func(t *testing.T) {
		got := ci.formatChunk("My answer", models.ConversationRoleAssistant, "My question")
		assert.Contains(t, got, "Question: My question")
		assert.Contains(t, got, "Answer: My answer")
	})

	t.Run("User without prompt", func(t *testing.T) {
		got := ci.formatChunk("My question", models.ConversationRoleUser, "")
		assert.Equal(t, "My question", got)
	})
}

func TestConversationIndexer_IndexMessage(t *testing.T) {
	mockConvRepo := new(MockConversationRepo)
	mockMsgRepo := new(MockConversationMessageRepo)
	mockVectorRepo := new(MockVectorRepo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	idx, _ := NewConversationIndexer(mockConvRepo, mockMsgRepo, mockVectorRepo, nil, logger)
	ctx := context.Background()
	projectID := uuid.New()
	convID := uuid.New()
	msgID := uuid.New()
	userID := uuid.New()

	conv := &models.Conversation{
		ID:        convID,
		ProjectID: projectID,
		UserID:    userID,
	}

	msg := &models.ConversationMessage{
		ID:             msgID,
		ConversationID: convID,
		Role:           models.ConversationRoleUser,
		Content:        "Hello world",
		CreatedAt:      time.Now(),
	}

	mockConvRepo.On("GetByID", ctx, projectID, convID, true).Return(conv, nil)
	mockMsgRepo.On("GetByID", ctx, convID, msgID, true).Return(msg, nil)
	mockVectorRepo.On("Create", ctx, projectID.String(), mock.MatchedBy(func(doc *models.VectorDocument) bool {
		return doc.ContentID == msgID.String() && 
			doc.Metadata["user_id"] == userID.String() &&
			doc.Content == "Hello world"
	})).Return("vector-id", nil)

	err := idx.IndexMessage(ctx, projectID, convID, msgID)

	assert.NoError(t, err)
	mockConvRepo.AssertExpectations(t)
	mockMsgRepo.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}
