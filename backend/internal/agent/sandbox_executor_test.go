package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSandboxRunner — мок для sandbox.SandboxRunner.
type MockSandboxRunner struct {
	mock.Mock
}

func (m *MockSandboxRunner) RunTask(ctx context.Context, opts sandbox.SandboxOptions) (*sandbox.SandboxInstance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxInstance), args.Error(1)
}

func (m *MockSandboxRunner) Wait(ctx context.Context, sandboxID string) (*sandbox.SandboxStatus, error) {
	args := m.Called(ctx, sandboxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxStatus), args.Error(1)
}

func (m *MockSandboxRunner) GetStatus(ctx context.Context, sandboxID string) (*sandbox.SandboxStatus, error) {
	args := m.Called(ctx, sandboxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxStatus), args.Error(1)
}

func (m *MockSandboxRunner) StreamLogs(ctx context.Context, sandboxID string) (<-chan sandbox.LogEntry, error) {
	args := m.Called(ctx, sandboxID)
	return args.Get(0).(<-chan sandbox.LogEntry), args.Error(1)
}

func (m *MockSandboxRunner) Stop(ctx context.Context, sandboxID string) error {
	args := m.Called(ctx, sandboxID)
	return args.Error(0)
}

func (m *MockSandboxRunner) StopTask(ctx context.Context, taskID string) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

func (m *MockSandboxRunner) Cleanup(ctx context.Context, sandboxID string) error {
	args := m.Called(ctx, sandboxID)
	return args.Error(0)
}

func TestSandboxAgentExecutor_Execute_Success(t *testing.T) {
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image", nil)

	input := ExecutionInput{
		TaskID:      "task-123",
		ProjectID:   "proj-456",
		GitURL:      "https://github.com/org/repo",
		BranchName:  "feature/test",
		CodeBackend: "claude-code",
	}

	sandboxID := "sandbox-789"
	instance := &sandbox.SandboxInstance{ID: sandboxID}
	status := &sandbox.SandboxStatus{
		ID:     sandboxID,
		Status: sandbox.SandboxStatusCompleted,
		Result: &sandbox.CodeResult{
			Success:    true,
			Output:     "Done",
			Diff:       "some diff",
			CommitHash: "hash123",
			BranchName: "feature/test",
		},
	}

	mockRunner.On("RunTask", mock.Anything, mock.MatchedBy(func(opts sandbox.SandboxOptions) bool {
		return opts.TaskID == input.TaskID && opts.RepoURL == input.GitURL
	})).Return(instance, nil)

	mockRunner.On("Wait", mock.Anything, sandboxID).Return(status, nil)
	mockRunner.On("Cleanup", mock.Anything, sandboxID).Return(nil)

	res, err := executor.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, sandboxID, res.SandboxInstanceID)
	assert.Equal(t, "Done", res.Output)
	assert.Contains(t, string(res.ArtifactsJSON), "hash123")

	mockRunner.AssertExpectations(t)
}

func TestSandboxAgentExecutor_Execute_InvalidInput(t *testing.T) {
	executor := NewSandboxAgentExecutor(nil, "test-image", nil)
	_, err := executor.Execute(context.Background(), ExecutionInput{})
	assert.ErrorIs(t, err, ErrExecutorNotConfigured)

	mockRunner := new(MockSandboxRunner)
	executor = NewSandboxAgentExecutor(mockRunner, "test-image", nil)
	_, err = executor.Execute(context.Background(), ExecutionInput{TaskID: "1", ProjectID: "1", GitURL: "https://github.com/repo", BranchName: "-bad"})
	assert.ErrorIs(t, err, ErrInvalidExecutionInput)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestSandboxAgentExecutor_Execute_RunError(t *testing.T) {
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image", nil)

	input := ExecutionInput{
		TaskID:     "task-123",
		ProjectID:  "proj-456",
		GitURL:     "https://github.com/org/repo",
		BranchName: "feature/test",
	}

	mockRunner.On("RunTask", mock.Anything, mock.Anything).Return(nil, errors.New("docker error"))

	_, err := executor.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker error")
}

func TestSandboxAgentExecutor_Execute_RunErrorWithID(t *testing.T) {
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image", nil)

	input := ExecutionInput{
		TaskID:     "task-123",
		ProjectID:  "proj-456",
		GitURL:     "https://github.com/org/repo",
		BranchName: "feature/test",
	}

	sandboxID := "sandbox-error-id"
	instance := &sandbox.SandboxInstance{ID: sandboxID}

	// Имитируем ситуацию: контейнер создан, но RunTask упал (например, при старте)
	mockRunner.On("RunTask", mock.Anything, mock.Anything).Return(instance, errors.New("start failed"))
	// Проверяем, что Cleanup все равно вызван
	mockRunner.On("Cleanup", mock.Anything, sandboxID).Return(nil)

	_, err := executor.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start failed")

	mockRunner.AssertExpectations(t)
}

func TestSandboxAgentExecutor_Execute_InvalidGitURL(t *testing.T) {
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image", nil)

	badURLs := []string{
		"file:///etc/passwd",
		"--upload-pack=whoami",
		"ftp://github.com/repo",
		"just-text",
	}

	for _, url := range badURLs {
		input := ExecutionInput{
			TaskID:     "task-1",
			ProjectID:  "proj-1",
			GitURL:     url,
			BranchName: "main",
		}
		_, err := executor.Execute(context.Background(), input)
		assert.ErrorIs(t, err, ErrInvalidExecutionInput, "URL: %s", url)
		assert.Contains(t, err.Error(), "GitURL must start with")
	}
}

func TestSandboxAgentExecutor_Execute_PreservesInstructionContent(t *testing.T) {
	// Проверяем, что инструкция передается без изменений (без деструктивной санитизации)
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image", nil)

	// Инструкция с кодом Python и shell-командами - должна передаваться как есть
	input := ExecutionInput{
		TaskID:     "task-123",
		ProjectID:  "proj-456",
		GitURL:     "https://github.com/org/repo",
		BranchName: "feature/test",
		Title:      "Write python script with curl",
		Description: `Create a python script that uses curl to fetch data.
		Include error handling: if (err != nil) { return err }`,
		PromptUser: `Write a bash script that does:
		curl -s https://api.example.com/data | python3 -c "import json,sys; print(json.load(sys.stdin))"
		Use variables like ${API_KEY} and $(date)`,
	}

	sandboxID := "sandbox-789"
	instance := &sandbox.SandboxInstance{ID: sandboxID}
	status := &sandbox.SandboxStatus{
		ID:     sandboxID,
		Status: sandbox.SandboxStatusCompleted,
		Result: &sandbox.CodeResult{
			Success: true,
			Output:  "Done",
		},
	}

	var capturedOpts sandbox.SandboxOptions
	mockRunner.On("RunTask", mock.Anything, mock.MatchedBy(func(opts sandbox.SandboxOptions) bool {
		capturedOpts = opts
		return true
	})).Return(instance, nil)

	mockRunner.On("Wait", mock.Anything, sandboxID).Return(status, nil)
	mockRunner.On("Cleanup", mock.Anything, sandboxID).Return(nil)

	res, err := executor.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.True(t, res.Success)

	// Проверяем, что инструкция содержит все специальные символы и код
	assert.Contains(t, capturedOpts.Instruction, "python")
	assert.Contains(t, capturedOpts.Instruction, "curl")
	assert.Contains(t, capturedOpts.Instruction, "if (err != nil)")
	assert.Contains(t, capturedOpts.Instruction, "${API_KEY}")
	assert.Contains(t, capturedOpts.Instruction, "$(date)")
	assert.Contains(t, capturedOpts.Instruction, "|")

	mockRunner.AssertExpectations(t)
}

func TestSandboxAgentExecutor_truncateArtifact(t *testing.T) {
	executor := &SandboxAgentExecutor{}

	t.Run("No truncation", func(t *testing.T) {
		s := "short string"
		res := executor.truncateArtifact(s, "Test")
		assert.Equal(t, s, res)
	})

	t.Run("Truncation", func(t *testing.T) {
		limit := 5 * 1024 * 1024
		s := string(make([]byte, limit+100))
		res := executor.truncateArtifact(s, "Test")
		assert.Equal(t, limit+len("\n...[TRUNCATED]"), len(res))
		assert.True(t, strings.HasSuffix(res, "...[TRUNCATED]"))
	})
}

func TestSandboxAgentExecutor_buildInstruction(t *testing.T) {
	executor := &SandboxAgentExecutor{}

	t.Run("Normal input", func(t *testing.T) {
		input := ExecutionInput{
			Title:       "Test Task",
			Description: "Create API endpoint",
			PromptUser:  "Use gin framework",
		}
		res := executor.buildInstruction(input)
		assert.Contains(t, res, "Title: Test Task")
		assert.Contains(t, res, "Description: Create API endpoint")
		assert.Contains(t, res, "Instruction: Use gin framework")
	})

	t.Run("Preserves special characters", func(t *testing.T) {
		// Проверяем, что специальные символы НЕ удаляются (санитизация отключена)
		input := ExecutionInput{
			Title:       "Task with; | & < > $ ( )",
			Description: "Code: if (err != nil) { return $HOME }",
			PromptUser:  "Use ${VAR} and $(cmd)",
		}
		res := executor.buildInstruction(input)

		// Все специальные символы должны сохраниться
		assert.Contains(t, res, ";")
		assert.Contains(t, res, "|")
		assert.Contains(t, res, "&")
		assert.Contains(t, res, "$")
		assert.Contains(t, res, "(")
		assert.Contains(t, res, ")")
		assert.Contains(t, res, "${VAR}")
		assert.Contains(t, res, "$(cmd)")
	})

	t.Run("Empty input", func(t *testing.T) {
		input := ExecutionInput{}
		res := executor.buildInstruction(input)
		assert.Equal(t, "", res)
	})
}
