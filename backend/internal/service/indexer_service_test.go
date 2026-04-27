package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Mocks (idx prefix чтобы не сталкиваться с моками в orchestrator_service_test.go) ---

type mockIdxCodeIndexer struct{ mock.Mock }

func (m *mockIdxCodeIndexer) IndexProject(ctx context.Context, req indexer.IndexingRequest) error {
	return m.Called(ctx, req).Error(0)
}
func (m *mockIdxCodeIndexer) SearchContext(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]indexer.Chunk, error) {
	args := m.Called(ctx, projectID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]indexer.Chunk), args.Error(1)
}

type mockIdxTaskIndexer struct{ mock.Mock }

func (m *mockIdxTaskIndexer) IndexTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}
func (m *mockIdxTaskIndexer) IndexTaskFromModel(ctx context.Context, task *models.Task) error {
	return m.Called(ctx, task).Error(0)
}
func (m *mockIdxTaskIndexer) IndexTaskWithData(ctx context.Context, task *models.Task, messages []models.TaskMessage) error {
	return m.Called(ctx, task, messages).Error(0)
}
func (m *mockIdxTaskIndexer) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}
func (m *mockIdxTaskIndexer) DeleteProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}
func (m *mockIdxTaskIndexer) IndexProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}

type mockIdxConvIndexer struct{ mock.Mock }

func (m *mockIdxConvIndexer) Start(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *mockIdxConvIndexer) Stop() {
	m.Called()
}
func (m *mockIdxConvIndexer) IndexMessage(ctx context.Context, projectID, conversationID, messageID uuid.UUID) error {
	return m.Called(ctx, projectID, conversationID, messageID).Error(0)
}
func (m *mockIdxConvIndexer) IndexMessageFromModel(ctx context.Context, conv *models.Conversation, msg *models.ConversationMessage, userPrompt string) error {
	return m.Called(ctx, conv, msg, userPrompt).Error(0)
}
func (m *mockIdxConvIndexer) DeleteMessage(ctx context.Context, projectID, messageID uuid.UUID) error {
	return m.Called(ctx, projectID, messageID).Error(0)
}
func (m *mockIdxConvIndexer) DeleteConversation(ctx context.Context, projectID, conversationID uuid.UUID) error {
	return m.Called(ctx, projectID, conversationID).Error(0)
}
func (m *mockIdxConvIndexer) IndexProjectConversations(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}

type mockIdxProjectService struct{ mock.Mock }

func (m *mockIdxProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockIdxProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockIdxProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	return nil, 0, nil
}
func (m *mockIdxProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockIdxProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}
func (m *mockIdxProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}
func (m *mockIdxProjectService) Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}

type mockIdxSyncRepo struct{ mock.Mock }

func (m *mockIdxSyncRepo) GetByPath(ctx context.Context, projectID uuid.UUID, filePath string) (*repository.FileSyncState, error) {
	args := m.Called(ctx, projectID, filePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.FileSyncState), args.Error(1)
}
func (m *mockIdxSyncRepo) Upsert(ctx context.Context, state *repository.FileSyncState) error {
	return m.Called(ctx, state).Error(0)
}
func (m *mockIdxSyncRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*repository.FileSyncState, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.FileSyncState), args.Error(1)
}
func (m *mockIdxSyncRepo) Delete(ctx context.Context, projectID uuid.UUID, filePath string) error {
	return m.Called(ctx, projectID, filePath).Error(0)
}
func (m *mockIdxSyncRepo) GetProjectState(ctx context.Context, projectID uuid.UUID) (*repository.ProjectSyncState, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ProjectSyncState), args.Error(1)
}
func (m *mockIdxSyncRepo) UpsertProjectState(ctx context.Context, state *repository.ProjectSyncState) error {
	return m.Called(ctx, state).Error(0)
}
func (m *mockIdxSyncRepo) AddFailedOperation(ctx context.Context, op *repository.FailedOperation) error {
	return m.Called(ctx, op).Error(0)
}
func (m *mockIdxSyncRepo) ListFailedOperations(ctx context.Context, projectID uuid.UUID) ([]*repository.FailedOperation, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.FailedOperation), args.Error(1)
}
func (m *mockIdxSyncRepo) DeleteFailedOperation(ctx context.Context, opID uuid.UUID) error {
	return m.Called(ctx, opID).Error(0)
}

type mockIdxVectorDeleter struct{ mock.Mock }

func (m *mockIdxVectorDeleter) DeleteByContentID(ctx context.Context, projectID string, contentID string) error {
	return m.Called(ctx, projectID, contentID).Error(0)
}

type mockIdxLocker struct{ mock.Mock }

func (m *mockIdxLocker) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	args := m.Called(ctx, key, ttl)
	return args.String(0), args.Error(1)
}
func (m *mockIdxLocker) Unlock(ctx context.Context, key, lockID string) error {
	return m.Called(ctx, key, lockID).Error(0)
}
func (m *mockIdxLocker) Refresh(ctx context.Context, key, lockID string, ttl time.Duration) error {
	return m.Called(ctx, key, lockID, ttl).Error(0)
}

// --- Helpers ---

// idxHarness объединяет моки и сервис для удобства setup'а в тестах.
type idxHarness struct {
	svc      *indexerService
	code     *mockIdxCodeIndexer
	task     *mockIdxTaskIndexer
	conv     *mockIdxConvIndexer
	project  *mockIdxProjectService
	syncRepo *mockIdxSyncRepo
	vec      *mockIdxVectorDeleter
	locker   *mockIdxLocker
}

func newIdxHarness(t *testing.T) *idxHarness {
	t.Helper()
	h := &idxHarness{
		code:     new(mockIdxCodeIndexer),
		task:     new(mockIdxTaskIndexer),
		conv:     new(mockIdxConvIndexer),
		project:  new(mockIdxProjectService),
		syncRepo: new(mockIdxSyncRepo),
		vec:      new(mockIdxVectorDeleter),
		locker:   new(mockIdxLocker),
	}
	h.svc = &indexerService{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		vectorDB:   h.vec,
		codeIdx:    h.code,
		taskIdx:    h.task,
		convIdx:    h.conv,
		projectSvc: h.project,
		syncRepo:   h.syncRepo,
		locker:     h.locker,
	}
	return h
}

// withFastRetries делает retry-бэкофф мгновенным для теста и восстанавливает значения после.
func withFastRetries(t *testing.T) {
	t.Helper()
	prevRetries, prevInit, prevMax := MaxRetries, InitialRetryDelay, MaxRetryDelay
	MaxRetries = 3
	InitialRetryDelay = 1 * time.Millisecond
	MaxRetryDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		MaxRetries = prevRetries
		InitialRetryDelay = prevInit
		MaxRetryDelay = prevMax
	})
}

// withLongWatchdog делает интервал watchdog'а заведомо больше длительности теста,
// чтобы локер.Refresh не вызывался в быстрых сценариях FullIndex.
func withLongWatchdog(t *testing.T) {
	t.Helper()
	prev := LockWatchdogInterval
	LockWatchdogInterval = 1 * time.Hour
	t.Cleanup(func() { LockWatchdogInterval = prev })
}

// --- sanitizePath ---

func TestSanitizePath(t *testing.T) {
	h := newIdxHarness(t)
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"clean relative", "src/foo.go", "src/foo.go", false},
		{"with dot segment", "./src/foo.go", "src/foo.go", false},
		{"normalized double slash", "src//foo.go", "src/foo.go", false},
		{"absolute rejected", "/etc/passwd", "", true},
		{"parent traversal rejected", "../etc/passwd", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.svc.sanitizePath(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- withRetry ---

func TestWithRetry_SuccessFirstAttempt(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	calls := 0
	err := h.svc.withRetry(context.Background(), "op", func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	calls := 0
	err := h.svc.withRetry(context.Background(), "op", func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestWithRetry_FailsAfterMaxRetries(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	calls := int32(0)
	err := h.svc.withRetry(context.Background(), "op", func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("nope")
	})
	require.Error(t, err)
	assert.Equal(t, int32(MaxRetries), atomic.LoadInt32(&calls))
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	h := newIdxHarness(t)
	prevInit := InitialRetryDelay
	InitialRetryDelay = 200 * time.Millisecond
	t.Cleanup(func() { InitialRetryDelay = prevInit })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	calls := 0
	err := h.svc.withRetry(ctx, "op", func() error {
		calls++
		return errors.New("transient")
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "retry должен прерваться cancel'ом до второй попытки")
}

// --- IndexTask / DeleteTask / DLQ ---

func TestIndexTask_Success(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	taskID := uuid.New()
	projectID := uuid.New()
	h.task.On("IndexTask", mock.Anything, taskID).Return(nil).Once()

	err := h.svc.IndexTask(context.Background(), projectID.String(), taskID.String())
	require.NoError(t, err)
	h.task.AssertExpectations(t)
}

func TestIndexTask_FailureGoesToDLQ(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	taskID := uuid.New()
	projectID := uuid.New()
	h.task.On("IndexTask", mock.Anything, taskID).Return(errors.New("boom")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.MatchedBy(func(op *repository.FailedOperation) bool {
		return op.ProjectID == projectID && op.Operation == "index_task" && op.EntityID == taskID.String()
	})).Return(nil).Once()

	err := h.svc.IndexTask(context.Background(), projectID.String(), taskID.String())
	require.Error(t, err)
	h.task.AssertExpectations(t)
	h.syncRepo.AssertExpectations(t)
}

func TestIndexTask_InvalidID(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.IndexTask(context.Background(), uuid.New().String(), "not-a-uuid")
	require.Error(t, err)
}

func TestDeleteTask_Success(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	taskID := uuid.New()
	h.task.On("DeleteTask", mock.Anything, taskID).Return(nil).Once()

	err := h.svc.DeleteTask(context.Background(), uuid.New().String(), taskID.String())
	require.NoError(t, err)
	h.task.AssertExpectations(t)
}

func TestDeleteTask_FailureGoesToDLQ(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	taskID := uuid.New()
	projectID := uuid.New()
	h.task.On("DeleteTask", mock.Anything, taskID).Return(errors.New("vector down")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.MatchedBy(func(op *repository.FailedOperation) bool {
		return op.Operation == "delete_task" && op.EntityID == taskID.String()
	})).Return(nil).Once()

	err := h.svc.DeleteTask(context.Background(), projectID.String(), taskID.String())
	require.Error(t, err)
}

func TestIndexTasks_BulkSuccess(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	t1, t2 := uuid.New(), uuid.New()
	projectID := uuid.New()
	h.task.On("IndexTask", mock.Anything, t1).Return(nil).Once()
	h.task.On("IndexTask", mock.Anything, t2).Return(nil).Once()

	err := h.svc.IndexTasks(context.Background(), projectID.String(), []string{t1.String(), t2.String()})
	require.NoError(t, err)
	h.task.AssertExpectations(t)
}

func TestIndexTasks_StopsOnFirstFailure(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	t1, t2 := uuid.New(), uuid.New()
	projectID := uuid.New()
	h.task.On("IndexTask", mock.Anything, t1).Return(errors.New("boom")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.Anything).Return(nil).Once()

	err := h.svc.IndexTasks(context.Background(), projectID.String(), []string{t1.String(), t2.String()})
	require.Error(t, err)
	h.task.AssertNotCalled(t, "IndexTask", mock.Anything, t2)
}

// --- DeleteMessage ---

func TestDeleteMessage_Success(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	msgID := uuid.New()
	projectID := uuid.New()
	h.conv.On("DeleteMessage", mock.Anything, projectID, msgID).Return(nil).Once()

	err := h.svc.DeleteMessage(context.Background(), projectID.String(), msgID.String())
	require.NoError(t, err)
	h.conv.AssertExpectations(t)
}

func TestDeleteMessage_FailureGoesToDLQ(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	msgID := uuid.New()
	projectID := uuid.New()
	h.conv.On("DeleteMessage", mock.Anything, projectID, msgID).Return(errors.New("boom")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.MatchedBy(func(op *repository.FailedOperation) bool {
		return op.Operation == "delete_message" && op.EntityID == msgID.String()
	})).Return(nil).Once()

	err := h.svc.DeleteMessage(context.Background(), projectID.String(), msgID.String())
	require.Error(t, err)
}

// --- IndexMessage (текущая реализация — заглушка, проверяем что не падает) ---

func TestIndexMessage_StubReturnsNil(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)
	err := h.svc.IndexMessage(context.Background(), uuid.New().String(), uuid.New().String())
	require.NoError(t, err)
}

func TestIndexMessages_Bulk(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)
	err := h.svc.IndexMessages(context.Background(), uuid.New().String(), []string{uuid.New().String(), uuid.New().String()})
	require.NoError(t, err)
}

// --- DeleteCode ---

func TestDeleteCode_PathNotFoundIsNoOp(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/missing.go").Return(nil, gorm.ErrRecordNotFound).Once()

	err := h.svc.DeleteCode(context.Background(), projectID.String(), "src/missing.go")
	require.NoError(t, err)
	h.vec.AssertNotCalled(t, "DeleteByContentID", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteCode_HappyPath(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	stateID := uuid.New()
	state := &repository.FileSyncState{ID: stateID, ProjectID: projectID, FilePath: "src/foo.go"}

	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/foo.go").Return(state, nil).Once()
	h.vec.On("DeleteByContentID", mock.Anything, projectID.String(), stateID.String()).Return(nil).Once()
	h.syncRepo.On("Delete", mock.Anything, projectID, "src/foo.go").Return(nil).Once()

	err := h.svc.DeleteCode(context.Background(), projectID.String(), "src/foo.go")
	require.NoError(t, err)
	h.syncRepo.AssertExpectations(t)
	h.vec.AssertExpectations(t)
}

func TestDeleteCode_VectorFailureGoesToDLQ(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	stateID := uuid.New()
	state := &repository.FileSyncState{ID: stateID, ProjectID: projectID, FilePath: "src/foo.go"}

	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/foo.go").Return(state, nil).Times(MaxRetries)
	h.vec.On("DeleteByContentID", mock.Anything, projectID.String(), stateID.String()).Return(errors.New("weaviate down")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.MatchedBy(func(op *repository.FailedOperation) bool {
		return op.Operation == "delete_code" && op.EntityID == "src/foo.go"
	})).Return(nil).Once()

	err := h.svc.DeleteCode(context.Background(), projectID.String(), "src/foo.go")
	require.Error(t, err)
}

func TestDeleteCode_PathTraversalRejected(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.DeleteCode(context.Background(), uuid.New().String(), "../etc/passwd")
	require.Error(t, err)
	h.syncRepo.AssertNotCalled(t, "GetByPath", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteCodes_StopsOnInvalidPath(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.DeleteCodes(context.Background(), uuid.New().String(), []string{"src/ok.go", "/abs/bad.go"})
	require.Error(t, err)
	h.syncRepo.AssertNotCalled(t, "GetByPath", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteCodes_BulkSuccess(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/a.go").Return(nil, gorm.ErrRecordNotFound).Once()
	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/b.go").Return(nil, gorm.ErrRecordNotFound).Once()

	err := h.svc.DeleteCodes(context.Background(), projectID.String(), []string{"src/a.go", "src/b.go"})
	require.NoError(t, err)
	h.syncRepo.AssertExpectations(t)
}

// --- IndexCode / IndexCodes ---

func TestIndexCode_Success(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Once()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(nil).Once()

	err := h.svc.IndexCode(context.Background(), projectID.String(), "src/foo.go")
	require.NoError(t, err)
}

func TestIndexCode_PathTraversalRejected(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.IndexCode(context.Background(), uuid.New().String(), "../etc/passwd")
	require.Error(t, err)
	h.code.AssertNotCalled(t, "IndexProject", mock.Anything, mock.Anything)
}

func TestIndexCodes_BulkSuccess(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Twice()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(nil).Twice()

	err := h.svc.IndexCodes(context.Background(), projectID.String(), []string{"src/a.go", "src/b.go"})
	require.NoError(t, err)
}

func TestIndexCodes_StopsOnInvalidPath(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.IndexCodes(context.Background(), uuid.New().String(), []string{"src/a.go", "../bad.go"})
	require.Error(t, err)
	h.code.AssertNotCalled(t, "IndexProject", mock.Anything, mock.Anything)
}

// --- Smoke: конструктор должен возвращать рабочий сервис ---

func TestNewIndexerService_Constructs(t *testing.T) {
	h := newIdxHarness(t)
	svc := NewIndexerService(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil, // *vectordb.Client (не используется в нашей операции ниже)
		h.code, h.task, h.conv,
		h.project, h.syncRepo, h.locker,
	)
	require.NotNil(t, svc)
	// GetIndexStatus не трогает vectorDB → можно безопасно вызвать.
	pid := uuid.New()
	h.syncRepo.On("GetProjectState", mock.Anything, pid).Return(nil, gorm.ErrRecordNotFound).Once()
	st, err := svc.GetIndexStatus(context.Background(), pid.String())
	require.NoError(t, err)
	assert.Equal(t, StateIdle, st.State)
}

// --- MoveCode ---

func TestMoveCode_HappyPath(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	// IndexCode -> projectSvc.GetByID + codeIdx.IndexProject
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Once()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(nil).Once()
	// DeleteCode -> syncRepo lookup miss → no-op
	h.syncRepo.On("GetByPath", mock.Anything, projectID, "src/old.go").Return(nil, gorm.ErrRecordNotFound).Once()

	err := h.svc.MoveCode(context.Background(), projectID.String(), "src/old.go", "src/new.go")
	require.NoError(t, err)
}

func TestMoveCode_IndexFailureSkipsDelete(t *testing.T) {
	h := newIdxHarness(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Times(MaxRetries)
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(errors.New("boom")).Times(MaxRetries)
	h.syncRepo.On("AddFailedOperation", mock.Anything, mock.Anything).Return(nil).Once()

	err := h.svc.MoveCode(context.Background(), projectID.String(), "src/old.go", "src/new.go")
	require.Error(t, err)
	h.syncRepo.AssertNotCalled(t, "GetByPath", mock.Anything, mock.Anything, mock.Anything)
}

// --- GetIndexStatus ---

func TestGetIndexStatus_NoStateReturnsIdle(t *testing.T) {
	h := newIdxHarness(t)
	projectID := uuid.New()
	h.syncRepo.On("GetProjectState", mock.Anything, projectID).Return(nil, gorm.ErrRecordNotFound).Once()

	st, err := h.svc.GetIndexStatus(context.Background(), projectID.String())
	require.NoError(t, err)
	assert.Equal(t, StateIdle, st.State)
}

func TestGetIndexStatus_ReturnsStoredState(t *testing.T) {
	h := newIdxHarness(t)
	projectID := uuid.New()
	now := time.Now()
	h.syncRepo.On("GetProjectState", mock.Anything, projectID).Return(&repository.ProjectSyncState{
		ProjectID:    projectID,
		ActiveRunID:  "run-123",
		CurrentState: StateIndexing,
		Progress:     0.42,
		StartTime:    now,
		LastError:    "",
	}, nil).Once()

	st, err := h.svc.GetIndexStatus(context.Background(), projectID.String())
	require.NoError(t, err)
	assert.Equal(t, StateIndexing, st.State)
	assert.Equal(t, "run-123", st.RunID)
	assert.InDelta(t, 0.42, st.Progress, 1e-9)
}

func TestGetIndexStatus_InvalidProjectID(t *testing.T) {
	h := newIdxHarness(t)
	_, err := h.svc.GetIndexStatus(context.Background(), "not-a-uuid")
	require.Error(t, err)
}

// --- FullIndex (errgroup, locker, race-safe) ---

func TestFullIndex_HappyPath(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, "indexer_lock_"+projectID.String(), LockTTL).Return("lock-1", nil).Once()
	h.locker.On("Unlock", mock.Anything, "indexer_lock_"+projectID.String(), "lock-1").Return(nil).Once()

	// Дважды UpsertProjectState: сначала indexing, затем idle.
	var states []string
	var mu sync.Mutex
	h.syncRepo.On("UpsertProjectState", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s := args.Get(1).(*repository.ProjectSyncState)
		mu.Lock()
		defer mu.Unlock()
		states = append(states, s.CurrentState)
	}).Return(nil).Twice()

	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Once()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(nil).Once()
	h.task.On("IndexProjectTasks", mock.Anything, projectID).Return(nil).Once()
	h.conv.On("IndexProjectConversations", mock.Anything, projectID).Return(nil).Once()

	err := h.svc.FullIndex(context.Background(), projectID.String())
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, states, 2)
	assert.Equal(t, StateIndexing, states[0])
	assert.Equal(t, StateIdle, states[1])
}

func TestFullIndex_LockAcquireFails(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("locked by another worker")).Once()

	err := h.svc.FullIndex(context.Background(), projectID.String())
	require.Error(t, err)
	h.code.AssertNotCalled(t, "IndexProject", mock.Anything, mock.Anything)
	h.task.AssertNotCalled(t, "IndexProjectTasks", mock.Anything, mock.Anything)
	h.conv.AssertNotCalled(t, "IndexProjectConversations", mock.Anything, mock.Anything)
}

func TestFullIndex_PartialFailureWaitsAndReturnsErr(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, mock.Anything, mock.Anything).Return("lock-2", nil).Once()
	h.locker.On("Unlock", mock.Anything, mock.Anything, "lock-2").Return(nil).Once()

	// 1й upsert при старте (indexing), 2й — финальный (failed). Используем Background для финального записи ошибки.
	h.syncRepo.On("UpsertProjectState", mock.Anything, mock.Anything).Return(nil).Twice()

	// Code-индексатор фейлит — таск/чат могут как успеть, так и не успеть выполниться (errgroup отменяет ctx). Используем Maybe.
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Maybe()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(errors.New("code indexer failed")).Once()
	h.task.On("IndexProjectTasks", mock.Anything, projectID).Return(nil).Maybe()
	h.conv.On("IndexProjectConversations", mock.Anything, projectID).Return(nil).Maybe()

	err := h.svc.FullIndex(context.Background(), projectID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code indexer failed")
}

func TestFullIndex_ContextCancellation(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, mock.Anything, mock.Anything).Return("lock-3", nil).Once()
	h.locker.On("Unlock", mock.Anything, mock.Anything, "lock-3").Return(nil).Once()
	h.syncRepo.On("UpsertProjectState", mock.Anything, mock.Anything).Return(nil)

	// Индексаторы блокируются до отмены контекста.
	block := func(ctx context.Context, args ...interface{}) error {
		<-ctx.Done()
		return ctx.Err()
	}
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Maybe()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		_ = block(args.Get(0).(context.Context))
	}).Return(context.Canceled).Maybe()
	h.task.On("IndexProjectTasks", mock.Anything, projectID).Run(func(args mock.Arguments) {
		_ = block(args.Get(0).(context.Context))
	}).Return(context.Canceled).Maybe()
	h.conv.On("IndexProjectConversations", mock.Anything, projectID).Run(func(args mock.Arguments) {
		_ = block(args.Get(0).(context.Context))
	}).Return(context.Canceled).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := h.svc.FullIndex(ctx, projectID.String())
	require.Error(t, err)
}

func TestFullIndex_InvalidProjectID(t *testing.T) {
	h := newIdxHarness(t)
	err := h.svc.FullIndex(context.Background(), "not-a-uuid")
	require.Error(t, err)
	h.locker.AssertNotCalled(t, "Lock", mock.Anything, mock.Anything, mock.Anything)
}

func TestFullIndex_UpsertInitialStateFails(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, mock.Anything, mock.Anything).Return("lock-4", nil).Once()
	h.locker.On("Unlock", mock.Anything, mock.Anything, "lock-4").Return(nil).Once()
	h.syncRepo.On("UpsertProjectState", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	err := h.svc.FullIndex(context.Background(), projectID.String())
	require.Error(t, err)
	h.code.AssertNotCalled(t, "IndexProject", mock.Anything, mock.Anything)
}

// --- Тест на отсутствие гонок: запускаем FullIndex и одновременно DeleteTask из разных горутин ---

func TestConcurrent_FullIndexAndDeleteTask_NoRace(t *testing.T) {
	h := newIdxHarness(t)
	withLongWatchdog(t)
	withFastRetries(t)

	projectID := uuid.New()
	h.locker.On("Lock", mock.Anything, mock.Anything, mock.Anything).Return("lock-c", nil).Once()
	h.locker.On("Unlock", mock.Anything, mock.Anything, "lock-c").Return(nil).Once()
	h.syncRepo.On("UpsertProjectState", mock.Anything, mock.Anything).Return(nil)
	h.project.On("GetByID", mock.Anything, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil).Maybe()
	h.code.On("IndexProject", mock.Anything, mock.Anything).Return(nil).Maybe()
	h.task.On("IndexProjectTasks", mock.Anything, projectID).Return(nil).Maybe()
	h.conv.On("IndexProjectConversations", mock.Anything, projectID).Return(nil).Maybe()

	// Параллельные DeleteTask
	taskID := uuid.New()
	h.task.On("DeleteTask", mock.Anything, taskID).Return(nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = h.svc.FullIndex(context.Background(), projectID.String())
	}()
	go func() {
		defer wg.Done()
		_ = h.svc.DeleteTask(context.Background(), projectID.String(), taskID.String())
	}()
	wg.Wait()
}

