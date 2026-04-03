//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
)

func TestLLMRepository_ListLogs(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewLLMRepository(db)
	ctx := context.Background()

	// Create Logs (prompt_snapshot и response_snapshot — JSONB, нужен валидный JSON)
	log1 := &models.LLMLog{Provider: "openai", Model: "gpt-4", TotalTokens: 100, PromptSnapshot: "{}", ResponseSnapshot: "{}"}
	repo.CreateLog(ctx, log1)
	log2 := &models.LLMLog{Provider: "anthropic", Model: "claude", TotalTokens: 200, PromptSnapshot: "{}", ResponseSnapshot: "{}"}
	repo.CreateLog(ctx, log2)

	// List
	logs, count, err := repo.ListLogs(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, logs, 2)
}

