package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============ ResultProcessor Tests ============

func ptrInt(i int) *int {
	return &i
}

func TestNewResultProcessor(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		cfg := ResultProcessorConfig{}
		rp := NewResultProcessor(cfg, nil).(*resultProcessorImpl)

		assert.NotNil(t, rp)
		assert.NotNil(t, rp.cfg.MaxReviewIterations)
		assert.Equal(t, defaultMaxReviewIterations, *rp.cfg.MaxReviewIterations)
		assert.NotNil(t, rp.cfg.MaxTestIterations)
		assert.Equal(t, defaultMaxTestIterations, *rp.cfg.MaxTestIterations)
		assert.Equal(t, defaultOutputLimit, rp.cfg.OutputLimit)

		// Проверяем что процессоры зарегистрированы
		assert.Len(t, rp.processors, 4)
		assert.Contains(t, rp.processors, "planner")
		assert.Contains(t, rp.processors, "developer")
		assert.Contains(t, rp.processors, "reviewer")
		assert.Contains(t, rp.processors, "tester")
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := ResultProcessorConfig{
			MaxReviewIterations: ptrInt(5),
			MaxTestIterations:   ptrInt(2),
			OutputLimit:         1024,
		}
		rp := NewResultProcessor(cfg, nil).(*resultProcessorImpl)

		assert.Equal(t, 5, *rp.cfg.MaxReviewIterations)
		assert.Equal(t, 2, *rp.cfg.MaxTestIterations)
		assert.Equal(t, 1024, rp.cfg.OutputLimit)
	})

	t.Run("with zero iterations config", func(t *testing.T) {
		cfg := ResultProcessorConfig{
			MaxReviewIterations: ptrInt(0),
			MaxTestIterations:   ptrInt(0),
		}
		rp := NewResultProcessor(cfg, nil).(*resultProcessorImpl)

		assert.Equal(t, 0, *rp.cfg.MaxReviewIterations)
		assert.Equal(t, 0, *rp.cfg.MaxTestIterations)
	})

	t.Run("with custom processors", func(t *testing.T) {
		customProcessor := &mockRoleProcessor{role: "custom"}
		processors := map[string]RoleProcessor{
			"custom": customProcessor,
		}
		rp := NewResultProcessor(ResultProcessorConfig{}, processors).(*resultProcessorImpl)

		assert.Contains(t, rp.processors, "custom")
		// Стандартные процессоры тоже должны быть зарегистрированы
		assert.Contains(t, rp.processors, "planner")
	})
}

func TestResultProcessor_Process(t *testing.T) {
	rp := NewResultProcessor(ResultProcessorConfig{}, nil)
	ctx := context.Background()

	t.Run("nil execution result", func(t *testing.T) {
		result, err := rp.Process(ctx, "planner", "planning", nil, IterationCounters{})

		assert.Error(t, err)
		assert.Equal(t, ErrNilExecutionResult, err)
		assert.Equal(t, DecisionFail, result.Decision)
		assert.Equal(t, string(models.TaskStatusFailed), result.NewStatus)
	})

	t.Run("unknown role", func(t *testing.T) {
		execResult := &agent.ExecutionResult{Success: true}
		result, err := rp.Process(ctx, "unknown_role", "planning", execResult, IterationCounters{})

		assert.Error(t, err)
		assert.Equal(t, ErrUnknownRole, err)
		assert.Equal(t, DecisionFail, result.Decision)
		assert.Equal(t, string(models.TaskStatusFailed), result.NewStatus)
	})

	t.Run("successful planner processing", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"steps": []string{"step1", "step2"},
		})
		execResult := &agent.ExecutionResult{
			Success:       true,
			Output:        "Plan created",
			ArtifactsJSON: artifacts,
		}

		result, err := rp.Process(ctx, "planner", "planning", execResult, IterationCounters{})

		assert.NoError(t, err)
		assert.Equal(t, DecisionNextStep, result.Decision)
		assert.Equal(t, string(models.AgentRoleDeveloper), result.NextRole)
		assert.Equal(t, string(models.TaskStatusInProgress), result.NewStatus)
	})

	t.Run("OOM protection - stateless (original not mutated)", func(t *testing.T) {
		cfg := ResultProcessorConfig{OutputLimit: 10}
		rp := NewResultProcessor(cfg, nil)

		largeOutput := "This is a very long output string"
		execResult := &agent.ExecutionResult{
			Success: true,
			Output:  largeOutput,
		}

		_, err := rp.Process(ctx, "developer", "in_progress", execResult, IterationCounters{})

		assert.NoError(t, err)
		// Проверяем что ОРИГИНАЛ НЕ мутирован
		assert.Equal(t, largeOutput, execResult.Output)
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Отменяем сразу

		execResult := &agent.ExecutionResult{Success: true}
		_, err := rp.Process(cancelCtx, "planner", "planning", execResult, IterationCounters{})

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestMaskSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: [REDACTED]", // паттерн заменяет Bearer token
		},
		{
			name:     "GitHub token",
			input:    "Token: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expected: "[REDACTED]", // первый паттерн (token) заменяет всё целиком
		},
		{
			name:     "API key",
			input:    "api_key: secret12345678",
			expected: "[REDACTED]", // паттерн заменяет api_key: value целиком
		},
		{
			name:     "Password",
			input:    "password: mySuperSecret123",
			expected: "[REDACTED]", // паттерн заменяет password: value целиком
		},
		{
			name:     "Multiple secrets",
			input:    "bearer: abc123secretlong\nPassword: secretlonger\nAPI_KEY: key12345678",
			expected: "[REDACTED]\n[REDACTED]\n[REDACTED]",
		},
		{
			name:     "No secrets",
			input:    "This is just a normal log message",
			expected: "This is just a normal log message",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Short secret - masked",
			input:    "password: short",
			expected: "[REDACTED]", // Общий паттерн password заменяет любую длину
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSecrets(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateArtifactPath(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		workspaceRoot string
		wantErr       bool
		errIs         error
	}{
		{
			name:          "valid relative path",
			path:          "backend/internal/service/test.go",
			workspaceRoot: "",
			wantErr:       false,
		},
		{
			name:          "empty path",
			path:          "",
			workspaceRoot: "",
			wantErr:       false,
		},
		{
			name:          "absolute path",
			path:          "/etc/passwd",
			workspaceRoot: "",
			wantErr:       true,
			errIs:         ErrPathTraversal,
		},
		{
			name:          "path traversal with dot dot",
			path:          "../../../etc/passwd",
			workspaceRoot: "",
			wantErr:       true,
			errIs:         ErrPathTraversal,
		},
		{
			name:          "path traversal in middle",
			path:          "backend/../../etc/passwd",
			workspaceRoot: "",
			wantErr:       true,
			errIs:         ErrPathTraversal,
		},
		{
			name:          "path outside workspace",
			path:          "../other-project/file.go",
			workspaceRoot: "/workspace/my-project",
			wantErr:       true,
			errIs:         ErrPathTraversal,
		},
		{
			name:          "valid path in workspace",
			path:          "src/main.go",
			workspaceRoot: "/workspace",
			wantErr:       false,
		},
		{
			name:          "path with double dots but safe",
			path:          "backend/internal/utils/file.go",
			workspaceRoot: "/workspace",
			wantErr:       false,
		},
		{
			name:          "prefix attack - workspace_hacked",
			path:          "../workspace_hacked/file.go",
			workspaceRoot: "/tmp/workspace",
			wantErr:       true,
			errIs:         ErrPathTraversal,
		},
		{
			name:          "valid path inside workspace subdirectory",
			path:          "subproject/file.go",
			workspaceRoot: "/tmp/workspace",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactPath(tt.path, tt.workspaceRoot)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============ PlannerProcessor Tests ============

func TestPlannerProcessor_Process(t *testing.T) {
	cfg := ResultProcessorConfig{}
	processor := NewPlannerProcessor(cfg)
	ctx := context.Background()

	t.Run("success - plan created with steps", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"steps": []map[string]string{
				{"name": "step1", "description": "First step"},
				{"name": "step2", "description": "Second step"},
			},
		})
		result := &agent.ExecutionResult{
			Success:       true,
			Output:        "Plan created successfully",
			ArtifactsJSON: artifacts,
			Summary:       "2 steps planned",
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionNextStep, processResult.Decision)
		assert.Equal(t, string(models.AgentRoleDeveloper), processResult.NextRole)
		assert.Equal(t, string(models.TaskStatusInProgress), processResult.NewStatus)
		assert.Equal(t, "2 steps planned", processResult.ContextAdditions["plan_summary"])
	})

	t.Run("failure - execution failed", func(t *testing.T) {
		result := &agent.ExecutionResult{
			Success: false,
			Output:  "LLM error",
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionFail, processResult.Decision)
		assert.Equal(t, string(models.TaskStatusFailed), processResult.NewStatus)
		assert.Contains(t, processResult.ErrorMessage, "planner failed")
	})
}

// ============ DeveloperProcessor Tests ============

func TestDeveloperProcessor_Process(t *testing.T) {
	cfg := ResultProcessorConfig{}
	processor := NewDeveloperProcessor(cfg)
	ctx := context.Background()

	t.Run("success - code developed", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"files":         []string{"service.go", "handler.go"},
			"changed_files": []string{"service.go", "backend/internal/service/service.go"},
		})
		result := &agent.ExecutionResult{
			Success:           true,
			Output:            "Code developed successfully",
			ArtifactsJSON:     artifacts,
			SandboxInstanceID: "sandbox-123",
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionNextStep, processResult.Decision)
		assert.Equal(t, string(models.AgentRoleReviewer), processResult.NextRole)
		assert.Equal(t, string(models.TaskStatusReview), processResult.NewStatus)

		// Проверка дедупликации (service.go был дважды)
		var files []string
		json.Unmarshal([]byte(processResult.ContextAdditions["changed_files"]), &files)
		assert.Len(t, files, 3)
	})

	t.Run("failure - path traversal in artifacts", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"files": []string{"../../../etc/passwd"},
		})
		result := &agent.ExecutionResult{
			Success:       true,
			ArtifactsJSON: artifacts,
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrPathTraversal))
		assert.Equal(t, DecisionFail, processResult.Decision)
		assert.Equal(t, string(models.TaskStatusFailed), processResult.NewStatus)
	})
}

// ============ ReviewerProcessor Tests ============

func TestReviewerProcessor_Process(t *testing.T) {
	cfg := ResultProcessorConfig{
		MaxReviewIterations: ptrInt(3),
	}
	processor := NewReviewerProcessor(cfg)
	ctx := context.Background()

	t.Run("success - approve", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"decision": "approve",
		})
		result := &agent.ExecutionResult{
			Success:       true,
			Output:        "Code looks good",
			ArtifactsJSON: artifacts,
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionNextStep, processResult.Decision)
		assert.Equal(t, string(models.AgentRoleTester), processResult.NextRole)
		assert.Equal(t, string(models.TaskStatusTesting), processResult.NewStatus)
	})

	t.Run("changes_requested - with structured comments", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"decision": "changes_requested",
			"comments": []map[string]string{
				{"message": "Fix this"},
			},
		})
		result := &agent.ExecutionResult{
			Success:       true,
			Output:        "Need changes output",
			ArtifactsJSON: artifacts,
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionRetry, processResult.Decision)
		assert.NotEmpty(t, processResult.ContextAdditions["review_comments"])
		// Не должно быть reviewer_feedback если есть структурированные комменты
		assert.Empty(t, processResult.ContextAdditions["reviewer_feedback"])
	})

	t.Run("changes_requested - without structured comments", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"decision": "changes_requested",
		})
		result := &agent.ExecutionResult{
			Success:       true,
			Output:        "Need changes plain text",
			ArtifactsJSON: artifacts,
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Empty(t, processResult.ContextAdditions["review_comments"])
		assert.Equal(t, "Need changes plain text", processResult.ContextAdditions["reviewer_feedback"])
	})
}

// ============ TesterProcessor Tests ============

func TestTesterProcessor_Process(t *testing.T) {
	cfg := ResultProcessorConfig{
		MaxTestIterations: ptrInt(3),
	}
	processor := NewTesterProcessor(cfg)
	ctx := context.Background()

	t.Run("success - test pass", func(t *testing.T) {
		artifacts, _ := json.Marshal(map[string]interface{}{
			"test_result": "pass",
		})
		result := &agent.ExecutionResult{
			Success:       true,
			ArtifactsJSON: artifacts,
		}

		processResult, err := processor.Process(ctx, result, IterationCounters{})

		require.NoError(t, err)
		assert.Equal(t, DecisionComplete, processResult.Decision)
		assert.Equal(t, string(models.TaskStatusCompleted), processResult.NewStatus)
	})
}

// ============ Helper Functions ============

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// mockRoleProcessor - мок для тестирования
type mockRoleProcessor struct {
	role string
}

func (m *mockRoleProcessor) Process(ctx context.Context, result *agent.ExecutionResult, iterations IterationCounters) (ProcessResult, error) {
	return ProcessResult{
		Decision:   DecisionNextStep,
		NextRole:   m.role,
		NewStatus:  "in_progress",
		Iterations: iterations,
	}, nil
}
