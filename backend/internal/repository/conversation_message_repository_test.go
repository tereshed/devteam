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

func createMsgTestConv(t *testing.T, db *gorm.DB, email string) (*models.User, *models.Project, *models.Conversation) {
	t.Helper()
	user := createConvTestUser(t, db, email)
	project := createConvTestProject(t, db, user.ID, "proj-"+email)
	conv := &models.Conversation{
		ProjectID: project.ID,
		UserID:    user.ID,
		Title:     "Test Conv",
		Status:    models.ConversationStatusActive,
	}
	require.NoError(t, db.Create(conv).Error)
	return user, project, conv
}

func TestConversationMessageRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m1@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "Hello, AI!",
		Metadata:       []byte(`{"source": "web"}`),
	}

	err := repo.Create(ctx, msg)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, msg.ID)

	got, err := repo.GetByID(ctx, conv.ID, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationRoleUser, got.Role)
	assert.Equal(t, "Hello, AI!", got.Content)
	assert.Equal(t, conv.ID, got.ConversationID)
}

func TestConversationMessageRepository_Create_InvalidConversationID(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: uuid.New(), // Non-existent
		Role:           models.ConversationRoleUser,
		Content:        "Invalid Conv",
	}

	err := repo.Create(ctx, msg)
	assert.ErrorIs(t, err, ErrConversationNotFound)
}

func TestConversationMessageRepository_Create_InvalidRole(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m2@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           "invalid_role",
		Content:        "Invalid Role",
	}

	err := repo.Create(ctx, msg)
	assert.ErrorIs(t, err, ErrInvalidMessageRole)
}

func TestConversationMessageRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m3@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	// Random ID
	_, err := repo.GetByID(ctx, conv.ID, uuid.New())
	assert.ErrorIs(t, err, ErrMessageNotFound)

	// Wrong ConversationID
	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "Test",
	}
	require.NoError(t, repo.Create(ctx, msg))

	_, err = repo.GetByID(ctx, uuid.New(), msg.ID)
	assert.ErrorIs(t, err, ErrMessageNotFound)
}

func TestConversationMessageRepository_ListByConversationID(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m4@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		role := models.ConversationRoleUser
		if i%2 == 0 {
			role = models.ConversationRoleAssistant
		}
		msg := &models.ConversationMessage{
			ConversationID: conv.ID,
			Role:           role,
			Content:        fmt.Sprintf("Msg %d", i),
			CreatedAt:      time.Now().Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repo.Create(ctx, msg))
	}

	t.Run("Pagination", func(t *testing.T) {
		list, total, err := repo.ListByConversationID(ctx, conv.ID, MessageFilter{
			Limit: 2, Offset: 0, OrderBy: OrderByMessageCreatedAt, OrderDir: "ASC",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, list, 2)
		assert.Equal(t, "Msg 0", list[0].Content)
	})

	t.Run("FilterRole", func(t *testing.T) {
		role := models.ConversationRoleAssistant
		list, total, err := repo.ListByConversationID(ctx, conv.ID, MessageFilter{
			Role: &role,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total) // 0, 2, 4
		assert.Len(t, list, 3)
	})

	t.Run("Security_Normalization", func(t *testing.T) {
		list, total, err := repo.ListByConversationID(ctx, conv.ID, MessageFilter{
			Limit:  -1,
			Offset: -1,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, list, 5) // Default limit is 20
	})
}

func TestConversationMessageRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m5@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "Old Content",
	}
	require.NoError(t, repo.Create(ctx, msg))

	updates := map[string]interface{}{
		"content": "New Content",
	}

	err := repo.Update(ctx, conv.ID, msg.ID, updates)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, conv.ID, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Content", got.Content)
}

func TestConversationMessageRepository_Update_EmptyMap(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m6@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "Content",
	}
	require.NoError(t, repo.Create(ctx, msg))

	err := repo.Update(ctx, conv.ID, msg.ID, nil)
	require.NoError(t, err)

	err = repo.Update(ctx, conv.ID, msg.ID, map[string]interface{}{})
	require.NoError(t, err)
}

func TestConversationMessageRepository_Update_ProtectedFields(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m7@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "Content",
	}
	require.NoError(t, repo.Create(ctx, msg))

	newConvID := uuid.New()
	updates := map[string]interface{}{
		"conversation_id": newConvID,
		"content":         "Updated Content",
	}

	err := repo.Update(ctx, conv.ID, msg.ID, updates)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, conv.ID, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, got.ConversationID) // Should NOT change
	assert.Equal(t, "Updated Content", got.Content)
}

func TestConversationMessageRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, _, conv := createMsgTestConv(t, tx, "m8@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           models.ConversationRoleUser,
		Content:        "To Delete",
	}
	require.NoError(t, repo.Create(ctx, msg))

	err := repo.Delete(ctx, conv.ID, msg.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, conv.ID, msg.ID)
	assert.ErrorIs(t, err, ErrMessageNotFound)
}

func TestConversationMessageRepository_ListByProjectID_Success(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	_, project, conv := createMsgTestConv(t, tx, "m9@example.com")
	repo := NewConversationMessageRepository(tx)
	ctx := context.Background()

	// Create 5 messages
	var lastMsgID uuid.UUID
	for i := 0; i < 5; i++ {
		msg := &models.ConversationMessage{
			ConversationID: conv.ID,
			Role:           models.ConversationRoleUser,
			Content:        fmt.Sprintf("Msg %d", i),
		}
		require.NoError(t, repo.Create(ctx, msg))
		if i == 1 {
			lastMsgID = msg.ID
		}
	}

	t.Run("All messages", func(t *testing.T) {
		list, err := repo.ListByProjectID(ctx, project.ID, nil, 10)
		require.NoError(t, err)
		assert.Len(t, list, 5)
	})

	t.Run("With cursor", func(t *testing.T) {
		list, err := repo.ListByProjectID(ctx, project.ID, &lastMsgID, 10)
		require.NoError(t, err)
		assert.Len(t, list, 3) // Messages after the 2nd one (index 1)
	})
}
