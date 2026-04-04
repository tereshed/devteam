//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func createTaskMessageTestTask(t *testing.T, db *gorm.DB) (*models.User, *models.Task) {
	t.Helper()
	ctx := context.Background()
	email := "tm-" + uuid.NewString() + "@example.com"
	user := createProjectTestUser(t, db, email)

	repo := NewProjectRepository(db)
	p := &models.Project{
		Name:        "tm-proj",
		GitProvider: models.GitProviderLocal,
		UserID:      user.ID,
		Status:      models.ProjectStatusActive,
	}
	require.NoError(t, repo.Create(ctx, p))

	task := &models.Task{
		ProjectID:     p.ID,
		Title:         "task",
		Description:   "",
		Status:        models.TaskStatusPending,
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   user.ID,
	}
	require.NoError(t, db.WithContext(ctx).Create(task).Error)
	return user, task
}

func TestTaskMessageRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	meta := datatypes.JSON([]byte(`{"tokens":42,"model":"x"}`))
	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeUser,
		SenderID:    user.ID,
		Content:     "hello",
		MessageType: models.MessageTypeInstruction,
		Metadata:    meta,
	}
	require.NoError(t, repo.Create(ctx, msg))
	assert.NotEqual(t, uuid.Nil, msg.ID)

	got, err := repo.GetByID(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.TaskID)
	assert.Equal(t, models.SenderTypeUser, got.SenderType)
	assert.Equal(t, user.ID, got.SenderID)
	assert.Equal(t, "hello", got.Content)
	assert.Equal(t, models.MessageTypeInstruction, got.MessageType)
	assert.JSONEq(t, `{"tokens":42,"model":"x"}`, string(got.Metadata))
}

func TestTaskMessageRepository_Create_InvalidTaskID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	msg := &models.TaskMessage{
		TaskID:      uuid.New(),
		SenderType:  models.SenderTypeUser,
		SenderID:    uuid.New(),
		Content:     "x",
		MessageType: models.MessageTypeResult,
		Metadata:    datatypes.JSON([]byte("{}")),
	}
	err := repo.Create(ctx, msg)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskMessageRepository_Create_InvalidMessageType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeUser,
		SenderID:    uuid.New(),
		Content:     "x",
		MessageType: models.MessageType("not_valid"),
		Metadata:    datatypes.JSON([]byte("{}")),
	}
	err := repo.Create(ctx, msg)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrTaskNotFound))
}

func TestTaskMessageRepository_Create_InvalidSenderType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderType("bot"),
		SenderID:    user.ID,
		Content:     "x",
		MessageType: models.MessageTypeResult,
		Metadata:    datatypes.JSON([]byte("{}")),
	}
	err := repo.Create(ctx, msg)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrTaskNotFound))
}

func TestTaskMessageRepository_GetByID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeAgent,
		SenderID:    uuid.New(),
		Content:     "done",
		MessageType: models.MessageTypeResult,
		Metadata:    datatypes.JSON([]byte(`{"k":1}`)),
	}
	require.NoError(t, repo.Create(ctx, msg))

	got, err := repo.GetByID(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, msg.ID, got.ID)
	assert.Equal(t, task.ID, got.TaskID)
	assert.Equal(t, models.SenderTypeAgent, got.SenderType)
	assert.Equal(t, "done", got.Content)
	assert.Equal(t, models.MessageTypeResult, got.MessageType)
}

func TestTaskMessageRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	repo := NewTaskMessageRepository(db)
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrTaskMessageNotFound)
}

func TestTaskMessageRepository_ListByTaskID_Chronological(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		msg := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeUser,
			SenderID:    uuid.New(),
			Content:     string(rune('a' + i)),
			MessageType: models.MessageTypeInstruction,
			Metadata:    datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(ctx, msg))
		time.Sleep(3 * time.Millisecond)
	}

	list, total, err := repo.ListByTaskID(ctx, task.ID, TaskMessageFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	require.Len(t, list, 5)
	for i := 0; i < 5; i++ {
		assert.Equal(t, string(rune('a'+i)), list[i].Content)
	}
}

func TestTaskMessageRepository_ListByTaskID_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		msg := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeUser,
			SenderID:    uuid.New(),
			Content:     "m",
			MessageType: models.MessageTypeFeedback,
			Metadata:    datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(ctx, msg))
		time.Sleep(2 * time.Millisecond)
	}

	list, total, err := repo.ListByTaskID(ctx, task.ID, TaskMessageFilter{Limit: 3, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	require.Len(t, list, 3)
}

func TestTaskMessageRepository_ListByTaskID_FilterByMessageType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	types := []models.MessageType{
		models.MessageTypeInstruction,
		models.MessageTypeError,
		models.MessageTypeResult,
		models.MessageTypeError,
	}
	for _, mt := range types {
		msg := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeAgent,
			SenderID:    uuid.New(),
			Content:     "x",
			MessageType: mt,
			Metadata:    datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(ctx, msg))
	}

	mtErr := models.MessageTypeError
	list, total, err := repo.ListByTaskID(ctx, task.ID, TaskMessageFilter{MessageType: &mtErr, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	for _, m := range list {
		assert.Equal(t, models.MessageTypeError, m.MessageType)
	}
}

func TestTaskMessageRepository_ListByTaskID_FilterBySenderType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeUser, SenderID: user.ID,
		Content: "u", MessageType: models.MessageTypeQuestion, Metadata: datatypes.JSON([]byte("{}")),
	}))
	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeAgent, SenderID: uuid.New(),
		Content: "a", MessageType: models.MessageTypeQuestion, Metadata: datatypes.JSON([]byte("{}")),
	}))

	st := models.SenderTypeAgent
	list, total, err := repo.ListByTaskID(ctx, task.ID, TaskMessageFilter{SenderType: &st, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].Content)
}

func TestTaskMessageRepository_ListBySender(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	agent1 := uuid.New()
	agent2 := uuid.New()

	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeAgent, SenderID: agent1,
		Content: "a1-first", MessageType: models.MessageTypeResult, Metadata: datatypes.JSON([]byte("{}")),
	}))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeAgent, SenderID: agent2,
		Content: "a2", MessageType: models.MessageTypeResult, Metadata: datatypes.JSON([]byte("{}")),
	}))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeAgent, SenderID: agent1,
		Content: "a1-second", MessageType: models.MessageTypeResult, Metadata: datatypes.JSON([]byte("{}")),
	}))

	list, total, err := repo.ListBySender(ctx, models.SenderTypeAgent, agent1, TaskMessageFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	assert.Equal(t, "a1-second", list[0].Content)
	assert.Equal(t, "a1-first", list[1].Content)
}

func TestTaskMessageRepository_CountByTaskID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	const n = 7
	for i := 0; i < n; i++ {
		require.NoError(t, repo.Create(ctx, &models.TaskMessage{
			TaskID: task.ID, SenderType: models.SenderTypeUser, SenderID: uuid.New(),
			Content: "c", MessageType: models.MessageTypeInstruction, Metadata: datatypes.JSON([]byte("{}")),
		}))
	}

	count, err := repo.CountByTaskID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(n), count)
}

func TestTaskMessageRepository_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task.ID, SenderType: models.SenderTypeUser, SenderID: uuid.New(),
		Content: "x", MessageType: models.MessageTypeError, Metadata: datatypes.JSON([]byte("{}")),
	}))

	require.NoError(t, db.WithContext(ctx).Delete(&models.Task{}, "id = ?", task.ID).Error)

	list, total, err := repo.ListByTaskID(ctx, task.ID, TaskMessageFilter{Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, list, 0)
}

func TestTaskMessageRepository_Isolation_DifferentTasks(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, task1 := createTaskMessageTestTask(t, db)
	_, task2 := createTaskMessageTestTask(t, db)
	repo := NewTaskMessageRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task1.ID, SenderType: models.SenderTypeUser, SenderID: uuid.New(),
		Content: "t1", MessageType: models.MessageTypeInstruction, Metadata: datatypes.JSON([]byte("{}")),
	}))
	require.NoError(t, repo.Create(ctx, &models.TaskMessage{
		TaskID: task2.ID, SenderType: models.SenderTypeUser, SenderID: uuid.New(),
		Content: "t2", MessageType: models.MessageTypeInstruction, Metadata: datatypes.JSON([]byte("{}")),
	}))

	list, total, err := repo.ListByTaskID(ctx, task1.ID, TaskMessageFilter{Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "t1", list[0].Content)
}
