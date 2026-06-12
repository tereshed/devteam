package service

import (
	"context"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestContextBuilder_Build_DBIsSourceOfTruth(t *testing.T) {
	model := "claude-sonnet-4-6"
	temp := 0.3
	maxTok := 8192
	prompt := "Ты — планировщик."
	provider := models.AgentProviderKindAnthropic

	builder := NewContextBuilder(nil, nil)
	input, err := builder.Build(context.Background(),
		&models.Task{Title: "test"},
		&models.Agent{
			Name:          "planner",
			Role:          models.AgentRolePlanner,
			ExecutionKind: models.AgentExecutionKindLLM,
			Model:         &model,
			Temperature:   &temp,
			MaxTokens:     &maxTok,
			SystemPrompt:  &prompt,
			ProviderKind:  &provider,
		},
		&models.Project{},
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, model, input.Model)
	assert.Equal(t, &temp, input.Temperature)
	assert.Equal(t, &maxTok, input.MaxTokens)
	assert.Equal(t, prompt, input.PromptSystem)
	assert.Equal(t, "anthropic", input.Provider)
}

func TestContextBuilder_Build_EmptyModelStaysEmpty(t *testing.T) {
	builder := NewContextBuilder(nil, nil)
	input, err := builder.Build(context.Background(),
		&models.Task{Title: "test"},
		&models.Agent{
			Name:          "unconfigured",
			Role:          models.AgentRoleAssistant,
			ExecutionKind: models.AgentExecutionKindLLM,
		},
		&models.Project{},
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, "", input.Model)
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
		longContent := strings.Repeat("a", 5000)
		chunks := []indexer.Chunk{
			{Content: longContent, FilePath: "1.go", Score: 0.9},
			{Content: longContent, FilePath: "2.go", Score: 0.9},
			{Content: longContent, FilePath: "3.go", Score: 0.9},
			{Content: longContent, FilePath: "4.go", Score: 0.9},
		}
		err := builder.WithCodeChunks(input, chunks)
		assert.NoError(t, err)

		assert.Contains(t, input.PromptUser, "1.go")
		assert.Contains(t, input.PromptUser, "3.go")
		assert.NotContains(t, input.PromptUser, "4.go")
		assert.Less(t, len(input.PromptUser), 17000)
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

func TestContextBuilder_Build_SandboxAgentModelFromSettings(t *testing.T) {
	builder := NewContextBuilder(nil, nil)
	input, err := builder.Build(context.Background(),
		&models.Task{Title: "test"},
		&models.Agent{
			Name:                "tester",
			Role:                models.AgentRoleTester,
			ExecutionKind:       models.AgentExecutionKindSandbox,
			Model:               nil,
			CodeBackendSettings: []byte(`{"model": "deepseek/deepseek-v4-flash"}`),
		},
		&models.Project{},
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, "deepseek/deepseek-v4-flash", input.Model)
}

// fakeOverrideReader — стаб ProjectAgentOverrideReader для теста подмешивания
// проектного оверрайда (фаза 2 энхансера).
type fakeOverrideReader struct {
	override *models.ProjectAgentOverride
	err      error
}

func (f *fakeOverrideReader) GetActiveOverride(ctx context.Context, projectID, agentID uuid.UUID) (*models.ProjectAgentOverride, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.override, nil
}

func TestContextBuilder_Build_EnhancerOverrideAppended(t *testing.T) {
	prompt := "Ты — разработчик."
	projectID := uuid.New()
	agentID := uuid.New()

	builder := NewContextBuilderFull(nil, nil, nil, nil,
		WithEnhancerOverridesOption(&fakeOverrideReader{override: &models.ProjectAgentOverride{
			ProjectID:      projectID,
			AgentID:        agentID,
			PromptAddendum: "Всегда указывай repo_slug в подзадачах.",
			IsActive:       true,
		}}),
	)
	input, err := builder.Build(context.Background(),
		&models.Task{Title: "test", ProjectID: projectID},
		&models.Agent{
			ID:            agentID,
			Name:          "developer",
			Role:          models.AgentRoleDeveloper,
			ExecutionKind: models.AgentExecutionKindLLM,
			SystemPrompt:  &prompt,
		},
		&models.Project{ID: projectID},
		nil,
	)
	assert.NoError(t, err)
	// Базовый промпт сохранён, добавка дописана отдельным блоком в конце.
	assert.True(t, strings.HasPrefix(input.PromptSystem, prompt))
	assert.Contains(t, input.PromptSystem, "=== PROJECT RULES (enhancer) ===")
	assert.Contains(t, input.PromptSystem, "Всегда указывай repo_slug")
}

func TestContextBuilder_Build_NoOverrideKeepsPromptIntact(t *testing.T) {
	prompt := "Ты — разработчик."
	builder := NewContextBuilderFull(nil, nil, nil, nil,
		WithEnhancerOverridesOption(&fakeOverrideReader{err: repository.ErrProjectAgentOverrideNotFound}),
	)
	input, err := builder.Build(context.Background(),
		&models.Task{Title: "test"},
		&models.Agent{
			ID:            uuid.New(),
			Name:          "developer",
			Role:          models.AgentRoleDeveloper,
			ExecutionKind: models.AgentExecutionKindLLM,
			SystemPrompt:  &prompt,
		},
		&models.Project{ID: uuid.New()},
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, prompt, input.PromptSystem)
}
