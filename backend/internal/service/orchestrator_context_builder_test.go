package service

import (
	"strings"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/indexer"
	"github.com/stretchr/testify/assert"
)

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
