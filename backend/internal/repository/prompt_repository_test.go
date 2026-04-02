//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/datatypes"
)

func TestPromptRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPromptRepository(db)
	ctx := context.Background()

	// 1. Test Create
	prompt := &models.Prompt{
		Name:        "test_prompt",
		Description: "Test Description",
		Template:    "Hello {{.Name}}",
		JSONSchema:  datatypes.JSON(`{"type": "object"}`),
		IsActive:    true,
	}

	err := repo.Create(ctx, prompt)
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt.ID)

	// 2. Test GetByID
	fetched, err := repo.GetByID(ctx, prompt.ID)
	assert.NoError(t, err)
	assert.Equal(t, prompt.Name, fetched.Name)
	assert.Equal(t, prompt.Template, fetched.Template)

	// 3. Test GetByName
	fetchedByName, err := repo.GetByName(ctx, "test_prompt")
	assert.NoError(t, err)
	assert.Equal(t, prompt.ID, fetchedByName.ID)

	// 4. Test Update
	prompt.Description = "Updated Description"
	prompt.IsActive = false
	err = repo.Update(ctx, prompt)
	assert.NoError(t, err)

	updated, err := repo.GetByID(ctx, prompt.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Description", updated.Description)
	assert.False(t, updated.IsActive)

	// 5. Test List
	list, err := repo.List(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)

	// 6. Test Delete
	err = repo.Delete(ctx, prompt.ID)
	assert.NoError(t, err)

	_, err = repo.GetByID(ctx, prompt.ID)
	assert.Error(t, err) // Should return error (record not found)
}

func TestPromptRepository_CreateDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPromptRepository(db)
	ctx := context.Background()

	prompt1 := &models.Prompt{
		Name:     "unique_name",
		Template: "Template 1",
	}
	err := repo.Create(ctx, prompt1)
	assert.NoError(t, err)

	prompt2 := &models.Prompt{
		Name:     "unique_name",
		Template: "Template 2",
	}
	err = repo.Create(ctx, prompt2)
	assert.Error(t, err) // Should fail due to unique constraint
}

