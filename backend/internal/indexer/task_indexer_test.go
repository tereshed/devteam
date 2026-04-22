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
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVectorRepo is a mock of VectorRepository
type MockVectorRepo struct {
	mock.Mock
}

func (m *MockVectorRepo) Create(ctx context.Context, projectID string, doc *models.VectorDocument) (string, error) {
	args := m.Called(ctx, projectID, doc)
	return args.String(0), args.Error(1)
}

func (m *MockVectorRepo) Get(ctx context.Context, projectID string, id string) (*models.VectorDocument, error) {
	args := m.Called(ctx, projectID, id)
	return args.Get(0).(*models.VectorDocument), args.Error(1)
}

func (m *MockVectorRepo) Update(ctx context.Context, projectID string, id string, doc *models.VectorDocument) error {
	args := m.Called(ctx, projectID, id, doc)
	return args.Error(0)
}

func (m *MockVectorRepo) Delete(ctx context.Context, projectID string, id string) error {
	args := m.Called(ctx, projectID, id)
	return args.Error(0)
}

func (m *MockVectorRepo) BatchCreate(ctx context.Context, projectID string, docs []*models.VectorDocument) (*vectordb.IndexStats, error) {
	args := m.Called(ctx, projectID, docs)
	return args.Get(0).(*vectordb.IndexStats), args.Error(1)
}

func (m *MockVectorRepo) DeleteByContentID(ctx context.Context, projectID string, contentID string) error {
	args := m.Called(ctx, projectID, contentID)
	return args.Error(0)
}

func (m *MockVectorRepo) DeleteByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) error {
	args := m.Called(ctx, projectID, contentType, category)
	return args.Error(0)
}

func (m *MockVectorRepo) Search(ctx context.Context, projectID string, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, params)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) SemanticSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, query, category, limit)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) KeywordSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	args := m.Called(ctx, projectID, query, category, limit)
	return args.Get(0).([]*vectordb.SearchResult), args.Error(1)
}

func (m *MockVectorRepo) CountByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) (int64, error) {
	args := m.Called(ctx, projectID, contentType, category)
	return args.Get(0).(int64), args.Error(1)
}

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

func (m *MockTaskRepo) Update(ctx context.Context, task *models.Task, expectedStatus models.TaskStatus, expectedUpdatedAt time.Time) error {
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

func TestTaskIndexer_SanitizeText(t *testing.T) {
	idx := &taskIndexer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API Key",
			input:    `My apiKey: "abcdef1234567890"`,
			expected: `My apiKey: "********"`,
		},
		{
			name:     "Bearer Token",
			input:    `Authorization: Bearer abcdef12345678901234567890`,
			expected: `Authorization: Bearer ********`,
		},
		{
			name:     "Password",
			input:    `db_password=supersecret123`,
			expected: `db_password=********`,
		},
		{
			name:     "Multiple secrets",
			input:    `secret: "val1", secret: "val2"`,
			expected: `secret: "********", secret: "********"`,
		},
		{
			name:     "No secrets",
			input:    "Just a normal text",
			expected: "Just a normal text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.sanitizeText(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
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
		Status:    models.TaskStatusCompleted,
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
