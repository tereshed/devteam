package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devteam/backend/pkg/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLLMProvider — мок для llm.Provider.
type MockLLMProvider struct {
	mock.Mock
}

func (m *MockLLMProvider) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.Response), args.Error(1)
}

func TestLLMAgentExecutor_Execute_Success(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	executor := NewLLMAgentExecutor(mockProvider)

	input := ExecutionInput{
		TaskID:       "task-123",
		Title:        "Test Task",
		PromptSystem: "You are a helper",
		PromptUser:   "Do something",
	}

	expectedResp := &llm.Response{
		Content: "Hello! Here is your JSON: ```json\n{\"key\": \"value\"}\n```",
		Usage: llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
		},
	}

	mockProvider.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
		return req.SystemPrompt == "You are a helper" &&
			strings.Contains(req.Messages[0].Content, "<task_title>\nTest Task\n</task_title>") &&
			strings.Contains(req.Messages[0].Content, "<user_instruction>\nDo something\n</user_instruction>")
	})).Return(expectedResp, nil)

	res, err := executor.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, expectedResp.Content, res.Output)
	assert.Equal(t, 10, res.PromptTokens)
	assert.Equal(t, 20, res.CompletionTokens)
	assert.JSONEq(t, `{"key": "value"}`, string(res.ArtifactsJSON))
	assert.Equal(t, "Extracted structured artifacts from LLM response", res.Summary)
}

func TestLLMAgentExecutor_Execute_InvalidInput(t *testing.T) {
	executor := NewLLMAgentExecutor(nil)
	_, err := executor.Execute(context.Background(), ExecutionInput{})
	assert.ErrorIs(t, err, ErrExecutorNotConfigured)

	mockProvider := new(MockLLMProvider)
	executor = NewLLMAgentExecutor(mockProvider)
	_, err = executor.Execute(context.Background(), ExecutionInput{TaskID: ""})
	assert.ErrorIs(t, err, ErrInvalidExecutionInput)
}

func TestLLMAgentExecutor_Execute_LLMError(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	executor := NewLLMAgentExecutor(mockProvider)

	input := ExecutionInput{TaskID: "task-123"}

	t.Run("Rate Limit", func(t *testing.T) {
		mockProvider.On("Generate", mock.Anything, mock.Anything).Return(nil, errors.New("rate limit exceeded (429)")).Once()
		_, err := executor.Execute(context.Background(), input)
		assert.ErrorIs(t, err, ErrRateLimit)
	})

	t.Run("Context Too Large", func(t *testing.T) {
		mockProvider.On("Generate", mock.Anything, mock.Anything).Return(nil, errors.New("context_length_exceeded")).Once()
		_, err := executor.Execute(context.Background(), input)
		assert.ErrorIs(t, err, ErrContextTooLarge)
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // сразу отменяем

		mockProvider.On("Generate", mock.Anything, mock.Anything).Return(nil, context.Canceled).Once()
		_, err := executor.Execute(ctx, input)
		assert.ErrorIs(t, err, ErrExecutionCancelled)
	})
}

func TestLLMAgentExecutor_Execute_InvalidJSONArtifact(t *testing.T) {
	mockProvider := new(MockLLMProvider)
	executor := NewLLMAgentExecutor(mockProvider)

	input := ExecutionInput{TaskID: "task-123"}
	expectedResp := &llm.Response{
		Content: "Bad JSON: ```json\n{invalid}\n```",
	}

	mockProvider.On("Generate", mock.Anything, mock.Anything).Return(expectedResp, nil)

	res, err := executor.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "LLM returned invalid JSON in artifacts block", res.Summary)
}
