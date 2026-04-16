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
	executor := NewSandboxAgentExecutor(mockRunner, "test-image")

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
	executor := NewSandboxAgentExecutor(nil, "test-image")
	_, err := executor.Execute(context.Background(), ExecutionInput{})
	assert.ErrorIs(t, err, ErrExecutorNotConfigured)

	mockRunner := new(MockSandboxRunner)
	executor = NewSandboxAgentExecutor(mockRunner, "test-image")
	_, err = executor.Execute(context.Background(), ExecutionInput{TaskID: "1", ProjectID: "1", GitURL: "https://github.com/repo", BranchName: "-bad"})
	assert.ErrorIs(t, err, ErrInvalidExecutionInput)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestSandboxAgentExecutor_Execute_RunError(t *testing.T) {
	mockRunner := new(MockSandboxRunner)
	executor := NewSandboxAgentExecutor(mockRunner, "test-image")

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
	executor := NewSandboxAgentExecutor(mockRunner, "test-image")

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
	executor := NewSandboxAgentExecutor(mockRunner, "test-image")

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
