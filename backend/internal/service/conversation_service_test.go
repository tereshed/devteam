package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"gorm.io/gorm"
)

// --- Mocks ---

type mockConversationRepo struct{ mock.Mock }

func (m *mockConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(repository.ConversationRepository)
}
func (m *mockConversationRepo) Create(ctx context.Context, conv *models.Conversation) error {
	return m.Called(ctx, conv).Error(0)
}
func (m *mockConversationRepo) GetByID(ctx context.Context, projectID, id uuid.UUID, master bool) (*models.Conversation, error) {
	args := m.Called(ctx, projectID, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}
func (m *mockConversationRepo) GetOnlyByID(ctx context.Context, id uuid.UUID, master bool) (*models.Conversation, error) {
	args := m.Called(ctx, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}
func (m *mockConversationRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID, filter repository.ConversationFilter) ([]*models.Conversation, int64, error) {
	args := m.Called(ctx, projectID, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.Conversation), args.Get(1).(int64), args.Error(2)
}
func (m *mockConversationRepo) Update(ctx context.Context, projectID, id uuid.UUID, updates map[string]interface{}) error {
	return m.Called(ctx, projectID, id, updates).Error(0)
}
func (m *mockConversationRepo) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return m.Called(ctx, projectID, id).Error(0)
}

type mockConversationMessageRepo struct{ mock.Mock }

func (m *mockConversationMessageRepo) WithTx(tx *gorm.DB) repository.ConversationMessageRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(repository.ConversationMessageRepository)
}
func (m *mockConversationMessageRepo) Create(ctx context.Context, msg *models.ConversationMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *mockConversationMessageRepo) GetByID(ctx context.Context, conversationID, id uuid.UUID, master bool) (*models.ConversationMessage, error) {
	args := m.Called(ctx, conversationID, id, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationMessage), args.Error(1)
}
func (m *mockConversationMessageRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID, filter repository.MessageFilter) ([]*models.ConversationMessage, int64, error) {
	args := m.Called(ctx, conversationID, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.ConversationMessage), args.Get(1).(int64), args.Error(2)
}
func (m *mockConversationMessageRepo) Update(ctx context.Context, conversationID, id uuid.UUID, updates map[string]interface{}) error {
	return m.Called(ctx, conversationID, id, updates).Error(0)
}
func (m *mockConversationMessageRepo) Delete(ctx context.Context, conversationID, id uuid.UUID) error {
	return m.Called(ctx, conversationID, id).Error(0)
}
func (m *mockConversationMessageRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID, lastID *uuid.UUID, limit int, master bool) ([]*models.ConversationMessage, error) {
	args := m.Called(ctx, projectID, lastID, limit, master)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ConversationMessage), args.Error(1)
}

type mockProjectSvc struct{ mock.Mock }

func (m *mockProjectSvc) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectSvc) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectSvc) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	return nil, 0, nil
}
func (m *mockProjectSvc) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectSvc) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}
func (m *mockProjectSvc) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

type mockTaskSvc struct{ mock.Mock }

func (m *mockTaskSvc) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskSvc) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	return nil, 0, nil
}
func (m *mockTaskSvc) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	return nil
}
func (m *mockTaskSvc) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskSvc) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	return nil, nil
}
func (m *mockTaskSvc) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	return nil, 0, nil
}
func (m *mockTaskSvc) Close() error {
	return nil
}

type mockOrchestratorSvc struct{ mock.Mock }

func (m *mockOrchestratorSvc) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}
func (m *mockOrchestratorSvc) Start(ctx context.Context) error {
	return nil
}

type mockTxManager struct{ mock.Mock }

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockConvEventBus struct{ mock.Mock }

func (m *mockConvEventBus) Publish(ctx context.Context, ev events.DomainEvent) {
	m.Called(ctx, ev)
}
func (m *mockConvEventBus) Subscribe(name string, buffer int) (<-chan events.DomainEvent, func()) {
	args := m.Called(name, buffer)
	return args.Get(0).(<-chan events.DomainEvent), args.Get(1).(func())
}
func (m *mockConvEventBus) Close() {
	m.Called()
}

type mockConversationIndexer struct{ mock.Mock }

func (m *mockConversationIndexer) Start(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *mockConversationIndexer) Stop() {
	m.Called()
}
func (m *mockConversationIndexer) IndexMessage(ctx context.Context, projectID, conversationID, messageID uuid.UUID) error {
	return m.Called(ctx, projectID, conversationID, messageID).Error(0)
}
func (m *mockConversationIndexer) IndexMessageFromModel(ctx context.Context, conv *models.Conversation, msg *models.ConversationMessage, userPrompt string) error {
	return m.Called(ctx, conv, msg, userPrompt).Error(0)
}
func (m *mockConversationIndexer) DeleteMessage(ctx context.Context, projectID, messageID uuid.UUID) error {
	return m.Called(ctx, projectID, messageID).Error(0)
}
func (m *mockConversationIndexer) DeleteConversation(ctx context.Context, projectID, conversationID uuid.UUID) error {
	return m.Called(ctx, projectID, conversationID).Error(0)
}
func (m *mockConversationIndexer) IndexProjectConversations(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}

// --- Harness ---

type mockDeps struct {
	convRepo        *mockConversationRepo
	msgRepo         *mockConversationMessageRepo
	projectSvc      *mockProjectSvc
	taskSvc         *mockTaskSvc
	orchestratorSvc *mockOrchestratorSvc
	indexer         *mockConversationIndexer
	txManager       *mockTxManager
	eventBus        *mockConvEventBus
}

func newTestConversationHarness(t *testing.T) (*conversationService, *mockDeps) {
	deps := &mockDeps{
		convRepo:        new(mockConversationRepo),
		msgRepo:         new(mockConversationMessageRepo),
		projectSvc:      new(mockProjectSvc),
		taskSvc:         new(mockTaskSvc),
		orchestratorSvc: new(mockOrchestratorSvc),
		indexer:         new(mockConversationIndexer),
		txManager:       new(mockTxManager),
		eventBus:        new(mockConvEventBus),
	}

	svc := NewConversationService(
		deps.convRepo,
		deps.msgRepo,
		deps.projectSvc,
		deps.taskSvc,
		deps.orchestratorSvc,
		deps.indexer,
		deps.txManager,
		deps.eventBus,
	).(*conversationService)

	t.Cleanup(func() {
		_ = svc.Shutdown(context.Background())
		deps.convRepo.AssertExpectations(t)
		deps.msgRepo.AssertExpectations(t)
		deps.projectSvc.AssertExpectations(t)
		deps.taskSvc.AssertExpectations(t)
		deps.orchestratorSvc.AssertExpectations(t)
	})

	return svc, deps
}

// --- Tests ---

func TestCreateConversation(t *testing.T) {
	userID := uuid.New()
	projectID := uuid.New()
	errDB := errors.New("db error")

	tests := []struct {
		name        string
		title       string
		setupMocks  func(svc *conversationService, deps *mockDeps)
		expectedErr error
	}{
		{
			name:  "TestCreateConversation_Success",
			title: "Valid Title",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(nil)
				deps.convRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Conversation")).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:        "TestCreateConversation_InvalidTitle",
			title:       "",
			setupMocks:  func(svc *conversationService, deps *mockDeps) {},
			expectedErr: ErrInvalidConversationTitle,
		},
		{
			name:        "TestCreateConversation_InvalidTitle_TooLong",
			title:       strings.Repeat("a", 256),
			setupMocks:  func(svc *conversationService, deps *mockDeps) {},
			expectedErr: ErrInvalidConversationTitle,
		},
		{
			name:  "TestCreateConversation_ProjectForbidden",
			title: "Valid Title",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(ErrProjectForbidden)
			},
			expectedErr: ErrConversationForbidden,
		},
		{
			name:  "TestCreateConversation_ProjectNotFound",
			title: "Valid Title",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(ErrProjectNotFound)
			},
			expectedErr: ErrConversationNotFound,
		},
		{
			name:  "TestCreateConversation_RepoError",
			title: "Valid Title",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(nil)
				deps.convRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Conversation")).Return(errDB)
			},
			expectedErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			conv, err := svc.CreateConversation(context.Background(), userID, projectID, tt.title)
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, conv)
			} else {
				require.NoError(t, err)
				require.NotNil(t, conv)
				assert.Equal(t, tt.title, conv.Title)
				assert.Equal(t, projectID, conv.ProjectID)
				assert.Equal(t, userID, conv.UserID)
			}
		})
	}
}

func TestGetConversation(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		setupMocks  func(svc *conversationService, deps *mockDeps)
		expectedErr error
	}{
		{
			name:   "TestGetConversation_Success",
			userID: userID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(&models.Conversation{UserID: userID}, nil)
			},
			expectedErr: nil,
		},
		{
			name:   "TestGetConversation_Forbidden",
			userID: otherUserID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(&models.Conversation{UserID: userID}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
		{
			name:   "TestGetConversation_EmptyUserID",
			userID: uuid.Nil,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(&models.Conversation{UserID: userID}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
		{
			name:   "TestGetConversation_NotFound",
			userID: userID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(nil, repository.ErrConversationNotFound)
			},
			expectedErr: ErrConversationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			conv, err := svc.GetConversation(context.Background(), tt.userID, convID)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, conv)
			} else {
				require.NoError(t, err)
				require.NotNil(t, conv)
			}
		})
	}
}

func TestListConversations(t *testing.T) {
	userID := uuid.New()
	projectID := uuid.New()

	tests := []struct {
		name           string
		limit          int
		offset         int
		setupMocks     func(svc *conversationService, deps *mockDeps)
		expectedLimit  int
		expectedOffset int
		expectedErr    error
	}{
		{
			name:           "TestListConversations_Success",
			limit:          10,
			offset:         5,
			expectedLimit:  10,
			expectedOffset: 5,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(nil)
				deps.convRepo.On("ListByProjectID", mock.Anything, projectID, mock.MatchedBy(func(f repository.ConversationFilter) bool {
					return f.Limit == 10 && f.Offset == 5
				})).Return([]*models.Conversation{}, int64(0), nil)
			},
			expectedErr: nil,
		},
		{
			name:           "TestListConversations_NegativePagination",
			limit:          -5,
			offset:         -10,
			expectedLimit:  20,
			expectedOffset: 0,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(nil)
				deps.convRepo.On("ListByProjectID", mock.Anything, projectID, mock.MatchedBy(func(f repository.ConversationFilter) bool {
					return f.Limit == 20 && f.Offset == 0
				})).Return([]*models.Conversation{}, int64(0), nil)
			},
			expectedErr: nil,
		},
		{
			name:           "TestListConversations_ProjectForbidden",
			limit:          10,
			offset:         0,
			expectedLimit:  10,
			expectedOffset: 0,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.projectSvc.On("HasAccess", mock.Anything, userID, models.RoleUser, projectID).Return(ErrProjectForbidden)
			},
			expectedErr: ErrConversationForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			convs, total, err := svc.ListConversations(context.Background(), userID, projectID, tt.limit, tt.offset)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, convs)
			} else {
				require.NoError(t, err)
				require.NotNil(t, convs)
				assert.Equal(t, int64(0), total)
			}
		})
	}
}

func TestSendMessage(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	projectID := uuid.New()
	clientMsgID := uuid.New()
	errDB := errors.New("db error")

	tests := []struct {
		name        string
		content     string
		clientMsgID uuid.UUID
		setupMocks  func(svc *conversationService, deps *mockDeps)
		expectedErr error
	}{
		{
			name:        "TestSendMessage_Success",
			content:     "Hello",
			clientMsgID: clientMsgID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
				deps.msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationMessage")).Return(nil)
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
				deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
				deps.msgRepo.On("ListByConversationID", mock.Anything, convID, mock.Anything).Return([]*models.ConversationMessage{}, int64(0), nil)
				deps.eventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.ConversationMessageCreated")).Return()
				deps.indexer.On("IndexMessageFromModel", mock.Anything, mock.AnythingOfType("*models.Conversation"), mock.AnythingOfType("*models.ConversationMessage"), "").Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:        "TestSendMessage_InvalidContent",
			content:     "",
			clientMsgID: clientMsgID,
			setupMocks:  func(svc *conversationService, deps *mockDeps) {},
			expectedErr: ErrInvalidMessageContent,
		},
		{
			name:        "TestSendMessage_InvalidContent_TooLong",
			content:     strings.Repeat("a", 4097),
			clientMsgID: clientMsgID,
			setupMocks:  func(svc *conversationService, deps *mockDeps) {},
			expectedErr: ErrInvalidMessageContent,
		},
		{
			name:        "TestSendMessage_WhitespaceContent",
			content:     "\n\t ",
			clientMsgID: clientMsgID,
			setupMocks:  func(svc *conversationService, deps *mockDeps) {},
			expectedErr: ErrInvalidMessageContent,
		},
		{
			name:        "TestSendMessage_Forbidden",
			content:     "Hello",
			clientMsgID: clientMsgID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: uuid.New()}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
		{
			name:        "TestSendMessage_Idempotency_Duplicate",
			content:     "Hello",
			clientMsgID: clientMsgID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				svc.processedMessagesMu.Lock()
				svc.processedMessages[clientMsgID] = &models.ConversationMessage{}
				svc.processedMessagesMu.Unlock()
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
			},
			expectedErr: ErrDuplicateMessage,
		},
		{
			name:        "TestSendMessage_Idempotency_NilUUID",
			content:     "Hello",
			clientMsgID: uuid.Nil,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
				deps.msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationMessage")).Return(nil)
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
				deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
				deps.msgRepo.On("ListByConversationID", mock.Anything, convID, mock.Anything).Return([]*models.ConversationMessage{}, int64(0), nil)
				deps.eventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.ConversationMessageCreated")).Return()
				deps.indexer.On("IndexMessageFromModel", mock.Anything, mock.AnythingOfType("*models.Conversation"), mock.AnythingOfType("*models.ConversationMessage"), "").Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:        "TestSendMessage_RepoError",
			content:     "Hello",
			clientMsgID: clientMsgID,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
				deps.msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationMessage")).Return(errDB)
			},
			expectedErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t) })

			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			msg, err := svc.SendMessage(context.Background(), userID, convID, tt.content, tt.clientMsgID)
			
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedErr)
				if tt.expectedErr != ErrDuplicateMessage { require.Nil(t, msg) } else { require.NotNil(t, msg) }
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				
				if tt.clientMsgID != uuid.Nil {
					svc.processedMessagesMu.RLock()
					_, ok := svc.processedMessages[tt.clientMsgID]
					svc.processedMessagesMu.RUnlock()
					assert.True(t, ok, "message should be in processedMessages map")
				}
			}
		})
	}
}

func TestSendMessage_ConcurrentAccess(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	svc, deps := newTestConversationHarness(t)
	userID := uuid.New()
	convID := uuid.New()
	projectID := uuid.New()

	deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
	deps.msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationMessage")).Return(nil)
	deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
	deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
	deps.msgRepo.On("ListByConversationID", mock.Anything, convID, mock.Anything).Return([]*models.ConversationMessage{}, int64(0), nil)
	deps.eventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.ConversationMessageCreated")).Return()
	deps.indexer.On("IndexMessageFromModel", mock.Anything, mock.AnythingOfType("*models.Conversation"), mock.AnythingOfType("*models.ConversationMessage"), "").Return(nil)
	deps.indexer.On("IndexMessage", mock.Anything, projectID, convID, mock.AnythingOfType("uuid.UUID")).Return(nil)

	var wg sync.WaitGroup
	numGoroutines := 50

	// Test concurrent access with same and different clientMsgIDs
	sameMsgID := uuid.New()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			msgID := sameMsgID
			if i%2 == 0 {
				msgID = uuid.New()
			}
			
			_, _ = svc.SendMessage(context.Background(), userID, convID, "Concurrent test", msgID)
		}(i)
	}

	wg.Wait()
}

func TestGetHistory(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()

	tests := []struct {
		name           string
		limit          int
		offset         int
		setupMocks     func(svc *conversationService, deps *mockDeps)
		expectedLimit  int
		expectedOffset int
		expectedErr    error
	}{
		{
			name:           "TestGetHistory_Success",
			limit:          10,
			offset:         5,
			expectedLimit:  10,
			expectedOffset: 5,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(&models.Conversation{UserID: userID}, nil)
				deps.msgRepo.On("ListByConversationID", mock.Anything, convID, mock.MatchedBy(func(f repository.MessageFilter) bool {
					return f.Limit == 10 && f.Offset == 5
				})).Return([]*models.ConversationMessage{}, int64(0), nil)
			},
			expectedErr: nil,
		},
		{
			name:           "TestGetHistory_Forbidden",
			limit:          10,
			offset:         0,
			expectedLimit:  10,
			expectedOffset: 0,
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, false).Return(&models.Conversation{UserID: uuid.New()}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			msgs, total, err := svc.GetHistory(context.Background(), userID, convID, tt.limit, tt.offset)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, msgs)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msgs)
				assert.Equal(t, int64(0), total)
			}
		})
	}
}

func TestDeleteConversation(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	projectID := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(svc *conversationService, deps *mockDeps)
		expectedErr error
	}{
		{
			name: "TestDeleteConversation_Success",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
				deps.convRepo.On("Delete", mock.Anything, projectID, convID).Return(nil)
				deps.eventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.ConversationDeleted")).Return()
				deps.indexer.On("DeleteConversation", mock.Anything, projectID, convID).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name: "TestDeleteConversation_Forbidden",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: uuid.New(), ProjectID: projectID}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			err := svc.DeleteConversation(context.Background(), userID, convID)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunOrchestrator(t *testing.T) {
	userID := uuid.New()
	projectID := uuid.New()
	convID := uuid.New()
	content := "Test content"

	tests := []struct {
		name       string
		setupMocks func(svc *conversationService, deps *mockDeps)
	}{
		{
			name: "TestRunOrchestrator_Success",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
				deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
				deps.msgRepo.On("ListByConversationID", mock.Anything, convID, mock.Anything).Return([]*models.ConversationMessage{{ID: uuid.New(), Role: models.ConversationRoleAssistant}}, int64(1), nil)
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{ID: convID, ProjectID: projectID}, nil)
				deps.indexer.On("IndexMessageFromModel", mock.Anything, mock.AnythingOfType("*models.Conversation"), mock.AnythingOfType("*models.ConversationMessage"), content).Return(nil)
			},
		},
		{
			name: "TestRunOrchestrator_TaskCreateFails",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(nil, errors.New("task error"))
			},
		},
		{
			name: "TestRunOrchestrator_OrchestratorFails",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
				deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(errors.New("orchestrator error"))
			},
		},
		{
			name: "TestRunOrchestrator_RecoversFromPanic",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.taskSvc.On("Create", mock.Anything, userID, models.RoleUser, projectID, mock.AnythingOfType("dto.CreateTaskRequest")).Return(&models.Task{ID: uuid.New()}, nil)
				deps.orchestratorSvc.On("ProcessTask", mock.Anything, mock.AnythingOfType("uuid.UUID")).Run(func(args mock.Arguments) {
					panic("test panic")
				}).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t) })

			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			svc.wg.Add(1)
			
			// Run synchronously to ensure panic is caught and wg is done
			svc.runOrchestrator(context.Background(), userID, projectID, convID, content)
			
			// Wait to ensure wg.Done was called
			done := make(chan struct{})
			go func() {
				svc.wg.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				// Success
			case <-time.After(1 * time.Second):
				t.Fatal("wg.Wait() timed out, wg.Done() was not called")
			}
		})
	}
}

func TestDeleteMessage(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	projectID := uuid.New()
	msgID := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(svc *conversationService, deps *mockDeps)
		expectedErr error
	}{
		{
			name: "TestDeleteMessage_Success",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: userID, ProjectID: projectID}, nil)
				deps.msgRepo.On("Delete", mock.Anything, convID, msgID).Return(nil)
				deps.eventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.ConversationMessageDeleted")).Return()
				deps.indexer.On("DeleteMessage", mock.Anything, projectID, msgID).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name: "TestDeleteMessage_Forbidden",
			setupMocks: func(svc *conversationService, deps *mockDeps) {
				deps.convRepo.On("GetOnlyByID", mock.Anything, convID, true).Return(&models.Conversation{UserID: uuid.New(), ProjectID: projectID}, nil)
			},
			expectedErr: ErrConversationForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestConversationHarness(t)
			tt.setupMocks(svc, deps)

			err := svc.DeleteMessage(context.Background(), userID, convID, msgID)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCleanupLoop_RemovesOldMessages(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	svc, _ := newTestConversationHarness(t)
	
	oldID := uuid.New()
	newID := uuid.New()
	
	svc.processedMessagesMu.Lock()
	svc.processedMessages[oldID] = &models.ConversationMessage{CreatedAt: time.Now().Add(-11 * time.Minute)}
	svc.processedMessages[newID] = &models.ConversationMessage{CreatedAt: time.Now()}
	svc.processedMessagesMu.Unlock()

	svc.cleanupOldMessages()
	
	svc.processedMessagesMu.RLock()
	_, oldExists := svc.processedMessages[oldID]
	_, newExists := svc.processedMessages[newID]
	svc.processedMessagesMu.RUnlock()
	
	assert.False(t, oldExists, "old message should be removed")
	assert.True(t, newExists, "new message should be kept")
}

func TestShutdown_StopsCleanupLoop(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	svc, _ := newTestConversationHarness(t)
	
	err := svc.Shutdown(context.Background())
	require.NoError(t, err)
	
	// Wait a bit to ensure the loop actually stops
	time.Sleep(50 * time.Millisecond)
}

func TestShutdown_WaitsForOrchestrators(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	svc, _ := newTestConversationHarness(t)
	
	svc.wg.Add(1)
	
	go func() {
		time.Sleep(100 * time.Millisecond)
		svc.wg.Done()
	}()
	
	start := time.Now()
	err := svc.Shutdown(context.Background())
	require.NoError(t, err)
	
	assert.True(t, time.Since(start) >= 100*time.Millisecond, "Shutdown should wait for wg")
}

func TestShutdown_ContextTimeout(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	svc, _ := newTestConversationHarness(t)
	
	svc.wg.Add(1)
	// We never call wg.Done() to simulate a hung orchestrator
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	err := svc.Shutdown(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	
	// Clean up the wg so the test can finish cleanly
	svc.wg.Done()
}
