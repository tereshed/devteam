import re

with open('internal/service/orchestrator_service_test.go', 'r') as f:
    content = f.read()

# Replace the struct definition
old_struct = """	tests := []struct {
		name           string
		role           models.AgentRole
		executorErr    error
		expectedErrIs  error
		expectedErrMsg string
		isSandbox      bool
	}{"""
new_struct = """	tests := []struct {
		name           string
		role           models.AgentRole
		executorErr    error
		expectedErrIs  error
		expectedErrMsg string
		expectedStatus models.TaskStatus
		isSandbox      bool
	}{"""
content = content.replace(old_struct, new_struct)

# Add expectedStatus to test cases
replacements = [
    ('name:          "ExecutorTimeout",\n\t\t\trole:          models.AgentRolePlanner,\n\t\t\texecutorErr:   context.DeadlineExceeded,\n\t\t\texpectedErrIs: context.DeadlineExceeded,',
     'name:          "ExecutorTimeout",\n\t\t\trole:          models.AgentRolePlanner,\n\t\t\texecutorErr:   context.DeadlineExceeded,\n\t\t\texpectedErrIs: context.DeadlineExceeded,\n\t\t\texpectedStatus: models.TaskStatusFailed,'), # Wait, if it transitions to cancelled, it should be cancelled!
]

# Let's just use regex to add expectedStatus: models.TaskStatusFailed to all, then change ExecutorTimeout
content = re.sub(r'(expectedErrMsg: ".*",\n)', r'\1\t\t\texpectedStatus: models.TaskStatusFailed,\n', content)
content = re.sub(r'(expectedErrIs: context.DeadlineExceeded,\n)', r'\1\t\t\texpectedStatus: models.TaskStatusFailed,\n', content)

# Update the mock expectation to use tt.expectedStatus
old_mock = """h.taskSvc.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(opts TransitionOpts) bool {"""
new_mock = """h.taskSvc.On("Transition", mock.Anything, taskID, tt.expectedStatus, mock.MatchedBy(func(opts TransitionOpts) bool {"""
content = content.replace(old_mock, new_mock)

# Also update the return value
old_return = """}).Return(&models.Task{
				ID:     taskID,
				Status: models.TaskStatusFailed,
			}, nil).Once()"""
new_return = """}).Return(&models.Task{
				ID:     taskID,
				Status: tt.expectedStatus,
			}, nil).Once()"""
content = content.replace(old_return, new_return)

with open('internal/service/orchestrator_service_test.go', 'w') as f:
    f.write(content)

