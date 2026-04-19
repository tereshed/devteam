import re

with open('internal/service/orchestrator_service_test.go', 'r') as f:
    content = f.read()

# Define the new table-driven test
new_test = """
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		role           models.AgentRole
		executorErr    error
		expectedErrIs  error
		expectedErrMsg string
		expectedStatus models.TaskStatus
		isSandbox      bool
	}{
		{
			name:          "ExecutorTimeout",
			role:          models.AgentRolePlanner,
			executorErr:   context.DeadlineExceeded,
			expectedErrIs: context.DeadlineExceeded,
			expectedStatus: models.TaskStatusCancelled,
		},
		{
			name:          "ExecutorError",
			role:          models.AgentRolePlanner,
			executorErr:   errors.New("model error"),
			expectedErrMsg: "model error",
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name:          "ErrorMessagePropagated",
			role:          models.AgentRolePlanner,
			executorErr:   errors.New("specific error message"),
			expectedErrMsg: "specific error",
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name:          "ErrorUnwrapping",
			role:          models.AgentRolePlanner,
			executorErr:   fmt.Errorf("planner failed: %w", errors.New("original error")),
			expectedErrMsg: "original error",
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name:          "OrchestratorFails",
			role:          models.AgentRoleOrchestrator,
			executorErr:   errors.New("orchestrator failed"),
			expectedErrMsg: "orchestrator failed",
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name:          "PlannerFails",
			role:          models.AgentRolePlanner,
			executorErr:   errors.New("planner failed"),
			expectedErrMsg: "planner failed",
			expectedStatus: models.TaskStatusFailed,
		},
		{
			name:          "DeveloperFails",
			role:          models.AgentRoleDeveloper,
			executorErr:   errors.New("developer failed"),
			expectedErrMsg: "developer failed",
			expectedStatus: models.TaskStatusFailed,
			isSandbox:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t) })

			h := newTestOrchestratorHarness(t, 5)
			ctx := context.Background()
			taskID := uuid.New()
			projectID := uuid.New()
			agentID := uuid.New()

			task := &models.Task{
				ID:              taskID,
				ProjectID:       projectID,
				Status:          models.TaskStatusPending,
				AssignedAgentID: &agentID,
			}
			project := &models.Project{ID: projectID}
			testAgent := &models.Agent{
				ID:   agentID,
				Role: tt.role,
			}
			if tt.isSandbox {
				testAgent.CodeBackend = &[]models.CodeBackend{models.CodeBackendClaudeCode}[0]
			}

			h.taskRepo.On("GetByID", ctx, taskID).Return(task, nil).Once()
			h.projectSvc.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(project, nil)
			h.taskRepo.On("GetByID", mock.Anything, taskID).Return(task, nil)
			h.workflowRepo.On("GetAgentByID", mock.Anything, agentID).Return(testAgent, nil)

			if tt.isSandbox {
				h.sandboxExecutor.On("Execute", mock.Anything, mock.Anything).Return(nil, tt.executorErr).Once()
				h.sandboxStop.On("StopTask", mock.Anything, taskID.String()).Return(nil).Once()
			} else {
				h.llmExecutor.On("Execute", mock.Anything, mock.Anything).Return(nil, tt.executorErr).Once()
			}

			h.taskSvc.On("Transition", mock.Anything, taskID, tt.expectedStatus, mock.MatchedBy(func(opts TransitionOpts) bool {
				if tt.expectedErrMsg != "" {
					return opts.ErrorMessage != nil && strings.Contains(*opts.ErrorMessage, tt.expectedErrMsg)
				}
				return true
			})).Return(&models.Task{
				ID:     taskID,
				Status: tt.expectedStatus,
			}, nil).Once()

			err := h.service.ProcessTask(ctx, taskID)
			require.Error(t, err)
			if tt.expectedErrIs != nil {
				require.ErrorIs(t, err, tt.expectedErrIs)
			}
			if tt.expectedErrMsg != "" {
				require.ErrorContains(t, err, tt.expectedErrMsg)
			}

			if tt.isSandbox {
				h.sandboxStop.AssertCalled(t, "StopTask", mock.Anything, taskID.String())
			}
		})
	}
}
"""

# Remove the old unused var errorHandlingTests
content = re.sub(r'var errorHandlingTests = \[\]struct \{[\s\S]*?\}\{\n(?:.*\n)*?\}\n', '', content)

# Remove the 7 old tests
tests_to_remove = [
	"TestError_ExecutorTimeout",
	"TestError_ExecutorError",
	"TestError_ErrorMessagePropagated",
	"TestError_ErrorUnwrapping",
	"TestError_OrchestratorFails",
	"TestError_PlannerFails",
	"TestError_DeveloperFails"
]

for test in tests_to_remove:
	pattern = r'func ' + test + r'\(t \*testing\.T\) \{[\s\S]*?\n\}\n'
	content = re.sub(pattern, '', content)

# Insert the new test at the end of the file or after the Error Handling Tests header
header_pattern = r'(// 7\. Error Handling Tests \(Table-Driven\)\n// =============================================================================\n)'
content = re.sub(header_pattern, r'\1\n' + new_test + '\n', content)

with open('internal/service/orchestrator_service_test.go', 'w') as f:
    f.write(content)

