//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createConvTestUser создает уникального пользователя для теста.
func createConvTestUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	u := &models.User{
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

// createConvTestProject создает уникальный проект для теста.
func createConvTestProject(t *testing.T, db *gorm.DB, userID uuid.UUID, name string) *models.Project {
	t.Helper()
	p := &models.Project{
		Name:        name,
		UserID:      userID,
		Status:      models.ProjectStatusActive,
		GitProvider: models.GitProviderLocal,
	}
	require.NoError(t, db.Create(p).Error)
	return p
}

func TestConversationRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c1@example.com")
	project := createConvTestProject(t, tx, user.ID, "p1")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "Test Conversation",
		Status:    models.ConversationStatusActive,
	}

	err := repo.Create(ctx, conv)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, conv.ID)

	got, err := repo.GetByID(ctx, project.ID, conv.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "Test Conversation", got.Title)
	assert.Equal(t, models.ConversationStatusActive, got.Status)
	assert.Equal(t, project.ID, got.ProjectID)
	assert.Equal(t, user.ID, got.UserID)
}

func TestConversationRepository_Create_InvalidProjectID(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c2@example.com")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	conv := &models.Conversation{
		ProjectID: uuid.New(), // Несуществующий ID
		UserID:    user.ID,
		Title:     "Invalid Project",
	}

	err := repo.Create(ctx, conv)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestConversationRepository_Create_InvalidUserID(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c3@example.com")
	project := createConvTestProject(t, tx, user.ID, "p3")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    uuid.New(), // Несуществующий ID
		Title:     "Invalid User",
	}

	err := repo.Create(ctx, conv)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestConversationRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c4@example.com")
	project := createConvTestProject(t, tx, user.ID, "p4")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	// Случайный ID
	_, err := repo.GetByID(ctx, project.ID, uuid.New(), false)
	assert.ErrorIs(t, err, ErrConversationNotFound)

	// Чужой ProjectID
	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "Test",
	}
	require.NoError(t, repo.Create(ctx, conv))

	_, err = repo.GetByID(ctx, uuid.New(), conv.ID, false)
	assert.ErrorIs(t, err, ErrConversationNotFound)
}

func TestConversationRepository_ListByProjectID(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c5@example.com")
	project := createConvTestProject(t, tx, user.ID, "p5")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		conv := &models.Conversation{
			ProjectID: project.ID,
			UserID:    user.ID,
			Title:     fmt.Sprintf("Conv %d", i),
			Status:    models.ConversationStatusActive,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if i == 4 {
			conv.Status = models.ConversationStatusArchived
			conv.Title = "SearchMe"
		}
		require.NoError(t, repo.Create(ctx, conv))
	}

	t.Run("Pagination", func(t *testing.T) {
		list, total, err := repo.ListByProjectID(ctx, project.ID, ConversationFilter{
			Limit: 2, Offset: 0, OrderBy: OrderByCreatedAt, OrderDir: "ASC",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, list, 2)
		assert.Equal(t, "Conv 0", list[0].Title)
	})

	t.Run("FilterStatus", func(t *testing.T) {
		status := models.ConversationStatusArchived
		list, total, err := repo.ListByProjectID(ctx, project.ID, ConversationFilter{
			Status: &status,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, list, 1)
		assert.Equal(t, "SearchMe", list[0].Title)
	})

	t.Run("Search", func(t *testing.T) {
		search := "searchme"
		list, total, err := repo.ListByProjectID(ctx, project.ID, ConversationFilter{
			Search: &search,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, list, 1)
		assert.Equal(t, "SearchMe", list[0].Title)
	})

	t.Run("OrderByWhitelist", func(t *testing.T) {
		list, total, err := repo.ListByProjectID(ctx, project.ID, ConversationFilter{
			OrderBy: "invalid_column; DROP TABLE conversations; --",
			Limit:   10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, list, 5)
	})
}

func TestConversationRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c6@example.com")
	project := createConvTestProject(t, tx, user.ID, "p6")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "Old Title",
		Status:    models.ConversationStatusActive,
	}
	require.NoError(t, repo.Create(ctx, conv))

	updates := map[string]interface{}{
		"title":  "New Title",
		"status": models.ConversationStatusCompleted,
	}

	err := repo.Update(ctx, project.ID, conv.ID, updates)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, project.ID, conv.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "New Title", got.Title)
	assert.Equal(t, models.ConversationStatusCompleted, got.Status)
}

func TestConversationRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c7@example.com")
	project := createConvTestProject(t, tx, user.ID, "p7")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	err := repo.Update(ctx, project.ID, uuid.New(), map[string]interface{}{"title": "New"})
	assert.ErrorIs(t, err, ErrConversationNotFound)
}

func TestConversationRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c8@example.com")
	project := createConvTestProject(t, tx, user.ID, "p8")
	repo := NewConversationRepository(tx)
	ctx := context.Background()

	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "To Delete",
	}
	require.NoError(t, repo.Create(ctx, conv))

	err := repo.Delete(ctx, project.ID, conv.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, project.ID, conv.ID, false)
	assert.ErrorIs(t, err, ErrConversationNotFound)
}

func TestConversationRepository_ContextTimeout(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	user := createConvTestUser(t, tx, "c9@example.com")
	project := createConvTestProject(t, tx, user.ID, "p9")
	repo := NewConversationRepository(tx)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "Timeout",
	}

	err := repo.Create(ctx, conv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
