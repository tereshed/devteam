package indexer

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTaskRepo is a mock of TaskRepository
type MockTaskRepo struct {
	mock.Mock
}

func (m *MockTaskRepo) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepo) List(ctx context.Context, filter repository.TaskFilter) ([]models.Task, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]models.Task), args.Get(1).(int64), args.Error(2)
}

func (m *MockTaskRepo) Update(ctx context.Context, task *models.Task, expectedStatus models.TaskState, expectedUpdatedAt time.Time) error {
	args := m.Called(ctx, task, expectedStatus, expectedUpdatedAt)
	return args.Error(0)
}

func (m *MockTaskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepo) CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepo) ListByParentID(ctx context.Context, parentTaskID uuid.UUID) ([]models.Task, error) {
	args := m.Called(ctx, parentTaskID)
	return args.Get(0).([]models.Task), args.Error(1)
}

// MockTaskMessageRepo is a mock of TaskMessageRepository
type MockTaskMessageRepo struct {
	mock.Mock
}

func (m *MockTaskMessageRepo) Create(ctx context.Context, msg *models.TaskMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockTaskMessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskMessage, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}

func (m *MockTaskMessageRepo) ListByTaskID(ctx context.Context, taskID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, taskID, filter)
	return args.Get(0).([]models.TaskMessage), args.Get(1).(int64), args.Error(2)
}

func (m *MockTaskMessageRepo) ListBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, senderType, senderID, filter)
	return args.Get(0).([]models.TaskMessage), args.Get(1).(int64), args.Error(2)
}

func (m *MockTaskMessageRepo) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(int64), args.Error(1)
}

func TestTaskIndexer_BuildTaskDocuments(t *testing.T) {
	idx := &taskIndexer{}
	res := "Task completed successfully"
	task := &models.Task{
		Title:       "Test Task",
		Description: "Do something with secret: 12345678",
		Result:      &res,
	}
	messages := []models.TaskMessage{
		{
			SenderType: models.SenderTypeUser,
			Content:    "Hello agent",
		},
		{
			SenderType: models.SenderTypeAgent,
			Content:    "Hello user, secret: 87654321",
		},
	}

	docs := idx.buildTaskDocuments(task, messages)

	assert.Len(t, docs, 1)
	doc := docs[0]
	assert.Contains(t, doc, "--- TASK TITLE ---")
	assert.Contains(t, doc, "Test Task")
	assert.Contains(t, doc, "--- TASK PROMPT ---")
	assert.Contains(t, doc, "Do something with secret: ********")
	assert.Contains(t, doc, "--- AGENT RESULT ---")
	assert.Contains(t, doc, "Task completed successfully")
	assert.Contains(t, doc, "--- DISCUSSION ---")
	assert.Contains(t, doc, "[User]: Hello agent")
	assert.Contains(t, doc, "[Agent]: Hello user, secret: ********")
}

func TestTaskIndexer_BuildTaskDocuments_Chunking(t *testing.T) {
	idx := &taskIndexer{}
	longText := strings.Repeat("a", 30000)
	task := &models.Task{
		Title:       "Long Task",
		Description: longText,
	}

	docs := idx.buildTaskDocuments(task, nil)

	assert.True(t, len(docs) > 1)
	assert.Contains(t, docs[0], "--- TASK TITLE ---")
	assert.Contains(t, docs[0], "Long Task")
}

func TestTaskIndexer_BuildTaskDocuments_UTF8(t *testing.T) {
	idx := &taskIndexer{}
	// Русский текст, где каждый символ занимает 2 байта
	russianText := strings.Repeat("привет", 5000) // 6 * 5000 = 30000 рун
	task := &models.Task{
		Title:       "Русская задача",
		Description: russianText,
	}

	docs := idx.buildTaskDocuments(task, nil)

	assert.True(t, len(docs) > 1)
	// Проверяем, что первый чанк не содержит битых символов (валидный UTF-8)
	for _, doc := range docs {
		assert.True(t, utf8.ValidString(doc))
	}
}

func TestTaskIndexer_IndexTask(t *testing.T) {
	mockTaskRepo := new(MockTaskRepo)
	mockMessageRepo := new(MockTaskMessageRepo)
	mockVectorRepo := new(MockVectorRepo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	idx := NewTaskIndexer(mockTaskRepo, mockMessageRepo, mockVectorRepo, logger)
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()

	task := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Test Task",
		State:     models.TaskStateDone,
	}

	mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
	mockMessageRepo.On("ListByTaskID", ctx, taskID, mock.Anything).Return([]models.TaskMessage{}, int64(0), nil)
	mockVectorRepo.On("DeleteByContentID", ctx, projectID.String(), taskID.String()).Return(nil)
	mockVectorRepo.On("Create", ctx, projectID.String(), mock.MatchedBy(func(doc *models.VectorDocument) bool {
		return doc.ContentID == taskID.String() && doc.Metadata["project_id"] == projectID.String()
	})).Return("vector-uuid", nil)

	err := idx.IndexTask(ctx, taskID)

	assert.NoError(t, err)
	mockTaskRepo.AssertExpectations(t)
	mockMessageRepo.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}

func TestTaskIndexer_DeleteTask(t *testing.T) {
	mockTaskRepo := new(MockTaskRepo)
	mockVectorRepo := new(MockVectorRepo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	idx := NewTaskIndexer(mockTaskRepo, nil, mockVectorRepo, logger)
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()

	task := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
	}

	mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
	mockVectorRepo.On("DeleteByContentID", ctx, projectID.String(), taskID.String()).Return(nil)

	err := idx.DeleteTask(ctx, taskID)

	assert.NoError(t, err)
	mockTaskRepo.AssertExpectations(t)
	mockVectorRepo.AssertExpectations(t)
}
