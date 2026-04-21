package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Моки ---

type mockConversationRepository struct{ mock.Mock }

func (m *mockConversationRepository) WithTx(tx *gorm.DB) repository.ConversationRepository {
	return m
}
func (m *mockConversationRepository) Create(ctx context.Context, conv *models.Conversation) error {
	return m.Called(ctx, conv).Error(0)
}
func (m *mockConversationRepository) GetByID(ctx context.Context, projectID, id uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, projectID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}
func (m *mockConversationRepository) GetOnlyByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}
func (m *mockConversationRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID, filter repository.ConversationFilter) ([]*models.Conversation, int64, error) {
	args := m.Called(ctx, projectID, filter)
	return args.Get(0).([]*models.Conversation), args.Get(1).(int64), args.Error(2)
}
func (m *mockConversationRepository) Update(ctx context.Context, projectID, id uuid.UUID, updates map[string]interface{}) error {
	return m.Called(ctx, projectID, id, updates).Error(0)
}
func (m *mockConversationRepository) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	return m.Called(ctx, projectID, id).Error(0)
}

type mockConversationMessageRepository struct{ mock.Mock }

func (m *mockConversationMessageRepository) WithTx(tx *gorm.DB) repository.ConversationMessageRepository {
	return m
}
func (m *mockConversationMessageRepository) Create(ctx context.Context, msg *models.ConversationMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *mockConversationMessageRepository) GetByID(ctx context.Context, conversationID, id uuid.UUID) (*models.ConversationMessage, error) {
	args := m.Called(ctx, conversationID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationMessage), args.Error(1)
}
func (m *mockConversationMessageRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID, filter repository.MessageFilter) ([]*models.ConversationMessage, int64, error) {
	args := m.Called(ctx, conversationID, filter)
	return args.Get(0).([]*models.ConversationMessage), args.Get(1).(int64), args.Error(2)
}
func (m *mockConversationMessageRepository) Update(ctx context.Context, conversationID, id uuid.UUID, updates map[string]interface{}) error {
	return m.Called(ctx, conversationID, id, updates).Error(0)
}
func (m *mockConversationMessageRepository) Delete(ctx context.Context, conversationID, id uuid.UUID) error {
	return m.Called(ctx, conversationID, id).Error(0)
}

type mockProjectServiceForConv struct{ mock.Mock }

func (m *mockProjectServiceForConv) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectServiceForConv) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockProjectServiceForConv) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	return nil, 0, nil
}
func (m *mockProjectServiceForConv) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockProjectServiceForConv) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}
func (m *mockProjectServiceForConv) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

type mockOrchestratorServiceForConv struct{ mock.Mock }

func (m *mockOrchestratorServiceForConv) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}
func (m *mockOrchestratorServiceForConv) Start(ctx context.Context) error {
	return nil
}

type mockTaskServiceForConv struct{ mock.Mock }

func (m *mockTaskServiceForConv) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskServiceForConv) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	return nil, 0, nil
}
func (m *mockTaskServiceForConv) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	return nil
}
func (m *mockTaskServiceForConv) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	return nil, nil
}
func (m *mockTaskServiceForConv) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	return nil, 0, nil
}

type mockTransactionManagerForConv struct{}

func (m *mockTransactionManagerForConv) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Тесты ---

func TestGetConversation_Forbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	convID := uuid.New()
	otherUserID := uuid.New()

	convRepo := new(mockConversationRepository)
	convRepo.On("GetOnlyByID", ctx, convID).Return(&models.Conversation{ID: convID, UserID: otherUserID}, nil)

	svc := NewConversationService(convRepo, nil, nil, nil, nil, nil)
	_, err := svc.GetConversation(ctx, userID, convID)

	require.ErrorIs(t, err, ErrConversationForbidden)
}

func TestCreateConversation_ProjectAccess(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	projectID := uuid.New()

	projectSvc := new(mockProjectServiceForConv)
	projectSvc.On("HasAccess", ctx, userID, models.RoleUser, projectID).Return(ErrProjectForbidden)

	svc := NewConversationService(nil, nil, projectSvc, nil, nil, nil)
	_, err := svc.CreateConversation(ctx, userID, projectID, "Title")

	require.ErrorIs(t, err, ErrConversationForbidden)
}

func TestSendMessage_ValidationError(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	convID := uuid.New()

	svc := NewConversationService(nil, nil, nil, nil, nil, nil)

	// Пустой контент
	_, err := svc.SendMessage(ctx, userID, convID, "")
	require.ErrorIs(t, err, ErrInvalidMessageContent)

	// Слишком длинный контент
	longContent := strings.Repeat("a", 4097)
	_, err = svc.SendMessage(ctx, userID, convID, longContent)
	require.ErrorIs(t, err, ErrInvalidMessageContent)
}

func TestSendMessage_Idempotency(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	convID := uuid.New()
	content := "Hello"

	convRepo := new(mockConversationRepository)
	convRepo.On("GetOnlyByID", ctx, convID).Return(&models.Conversation{ID: convID, UserID: userID, ProjectID: uuid.New()}, nil)

	msgRepo := new(mockConversationMessageRepository)
	msgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	txManager := new(mockTransactionManagerForConv)
	taskSvc := new(mockTaskServiceForConv)
	taskSvc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&models.Task{ID: uuid.New()}, nil)

	svc := NewConversationService(convRepo, msgRepo, nil, taskSvc, nil, txManager)

	// Первый вызов
	_, err := svc.SendMessage(ctx, userID, convID, content)
	require.NoError(t, err)

	// Второй вызов сразу же (дубликат)
	_, err = svc.SendMessage(ctx, userID, convID, content)
	require.ErrorIs(t, err, ErrMessageRateLimit)
}

func TestGetHistory_LimitNormalization(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	convID := uuid.New()

	convRepo := new(mockConversationRepository)
	convRepo.On("GetOnlyByID", ctx, convID).Return(&models.Conversation{ID: convID, UserID: userID}, nil)

	msgRepo := new(mockConversationMessageRepository)
	// Лимит 1000 должен быть нормализован до 100
	msgRepo.On("ListByConversationID", ctx, convID, mock.MatchedBy(func(f repository.MessageFilter) bool {
		return f.Limit == 100
	})).Return([]*models.ConversationMessage{}, int64(0), nil)

	svc := NewConversationService(convRepo, msgRepo, nil, nil, nil, nil)
	_, _, err := svc.GetHistory(ctx, userID, convID, 1000, 0)
	require.NoError(t, err)
}

func TestSendMessage_AsyncPanicRecovery(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	projectID := uuid.New()
	convID := uuid.New()
	content := "Hello"

	orchestratorSvc := new(mockOrchestratorServiceForConv)
	orchestratorSvc.On("ProcessTask", mock.Anything, mock.Anything).Panic("something went wrong")

	taskSvc := new(mockTaskServiceForConv)
	taskSvc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&models.Task{ID: uuid.New()}, nil)

	svc := &conversationService{
		orchestratorSvc: orchestratorSvc,
		taskSvc:         taskSvc,
	}

	require.NotPanics(t, func() {
		svc.runOrchestrator(ctx, userID, projectID, convID, content)
	})
}

func TestConversationService_Shutdown(t *testing.T) {
	ctx := context.Background()
	svc := &conversationService{
		stopChan: make(chan struct{}),
	}

	// Проверяем Shutdown без активных горутин
	err := svc.Shutdown(ctx)
	require.NoError(t, err)

	// Проверяем Shutdown с активной горутиной
	svc.wg.Add(1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		svc.wg.Done()
	}()

	err = svc.Shutdown(ctx)
	require.NoError(t, err)

	// Проверяем Shutdown по таймауту
	svc.wg.Add(1)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = svc.Shutdown(timeoutCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	svc.wg.Done() // Очистка для предотвращения утечки в тестах
}
