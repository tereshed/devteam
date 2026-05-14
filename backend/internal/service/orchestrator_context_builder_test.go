package service

import (
	"strings"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

// Sprint 16: resolveInputModel — regress-страховка под ревью #1. Контракт:
// claude-code/aider/custom/nil → YAML > DB (historical); hermes → DB > YAML.
func TestResolveInputModel_HermesPrefersDB(t *testing.T) {
	yaml := "claude-haiku-4-5-20251001"
	db := "anthropic/claude-3.5-haiku"
	cbHermes := models.CodeBackendHermes
	got := resolveInputModel(yaml, &models.Agent{CodeBackend: &cbHermes, Model: &db})
	assert.Equal(t, db, got, "hermes должен брать DB-Model, а не YAML-дефолт")
}

func TestResolveInputModel_HermesFallbacksToYAMLWhenDBEmpty(t *testing.T) {
	yaml := "default-model"
	cbHermes := models.CodeBackendHermes
	got := resolveInputModel(yaml, &models.Agent{CodeBackend: &cbHermes})
	assert.Equal(t, yaml, got, "если DB пуст — даже для hermes падаем на YAML")
}

func TestResolveInputModel_ClaudeCodePreservesHistoricalYAMLWinsPrecedence(t *testing.T) {
	yaml := "claude-haiku-4-5-20251001"
	db := "some-other-model"
	cbCC := models.CodeBackendClaudeCode
	got := resolveInputModel(yaml, &models.Agent{CodeBackend: &cbCC, Model: &db})
	assert.Equal(t, yaml, got, "claude-code: YAML побеждает (Sprint 6.9 контракт)")
}

func TestResolveInputModel_ClaudeCodeFallsBackToDBWhenYAMLEmpty(t *testing.T) {
	db := "explicit-db"
	cbCC := models.CodeBackendClaudeCode
	got := resolveInputModel("", &models.Agent{CodeBackend: &cbCC, Model: &db})
	assert.Equal(t, db, got, "claude-code: DB используется только когда YAML пуст")
}

func TestResolveInputModel_NoCodeBackendPreservesHistorical(t *testing.T) {
	// orchestrator/planner агенты (code_backend=nil) — тоже сохраняют YAML > DB.
	yaml := "yaml-pick"
	db := "db-pick"
	got := resolveInputModel(yaml, &models.Agent{Model: &db})
	assert.Equal(t, yaml, got)
}

func TestResolveInputModel_NilAgent(t *testing.T) {
	got := resolveInputModel("yaml", nil)
	assert.Equal(t, "yaml", got)
}

func TestContextBuilder_WithCodeChunks(t *testing.T) {
	builder := &contextBuilder{}

	t.Run("empty chunks", func(t *testing.T) {
		input := &agent.ExecutionInput{PromptUser: "original prompt"}
		err := builder.WithCodeChunks(input, []indexer.Chunk{})
		assert.NoError(t, err)
		assert.Equal(t, "original prompt", input.PromptUser)
	})

	t.Run("all chunks below threshold", func(t *testing.T) {
		input := &agent.ExecutionInput{PromptUser: "original prompt"}
		chunks := []indexer.Chunk{
			{Content: "low score", Score: 0.5},
			{Content: "very low score", Score: 0.1},
		}
		err := builder.WithCodeChunks(input, chunks)
		assert.NoError(t, err)
		assert.Equal(t, "original prompt", input.PromptUser)
	})

	t.Run("valid chunks with XML tags", func(t *testing.T) {
		input := &agent.ExecutionInput{PromptUser: "original prompt"}
		chunks := []indexer.Chunk{
			{
				Content:   "func hello() {}",
				FilePath:  "main.go",
				Symbol:    "hello",
				StartLine: 1,
				EndLine:   1,
				Score:     0.9,
			},
		}
		err := builder.WithCodeChunks(input, chunks)
		assert.NoError(t, err)
		assert.Contains(t, input.PromptUser, "--- CODE CONTEXT ---")
		assert.Contains(t, input.PromptUser, "<code_chunk file=\"main.go\" symbol=\"hello\" lines=\"1-1\">")
		assert.Contains(t, input.PromptUser, "func hello() {}")
		assert.Contains(t, input.PromptUser, "</code_chunk>")
	})

	t.Run("character limit approximation", func(t *testing.T) {
		input := &agent.ExecutionInput{PromptUser: ""}
		// Создаем много чанков, чтобы превысить лимит 16000 символов
		longContent := strings.Repeat("a", 5000)
		chunks := []indexer.Chunk{
			{Content: longContent, FilePath: "1.go", Score: 0.9},
			{Content: longContent, FilePath: "2.go", Score: 0.9},
			{Content: longContent, FilePath: "3.go", Score: 0.9},
			{Content: longContent, FilePath: "4.go", Score: 0.9}, // Этот уже должен быть за лимитом
		}
		err := builder.WithCodeChunks(input, chunks)
		assert.NoError(t, err)
		
		// Проверяем, что добавлено меньше 4 чанков
		assert.Contains(t, input.PromptUser, "1.go")
		assert.Contains(t, input.PromptUser, "3.go")
		assert.NotContains(t, input.PromptUser, "4.go")
		assert.Less(t, len(input.PromptUser), 17000) // 16000 + заголовок
	})

	t.Run("no symbol field", func(t *testing.T) {
		input := &agent.ExecutionInput{PromptUser: ""}
		chunks := []indexer.Chunk{
			{
				Content:   "plain code",
				FilePath:  "README.md",
				StartLine: 10,
				EndLine:   12,
				Score:     0.8,
			},
		}
		err := builder.WithCodeChunks(input, chunks)
		assert.NoError(t, err)
		assert.Contains(t, input.PromptUser, "<code_chunk file=\"README.md\" lines=\"10-12\">")
		assert.NotContains(t, input.PromptUser, "symbol=")
	})
}
