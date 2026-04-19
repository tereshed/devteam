import re

with open('internal/service/orchestrator_service_test.go', 'r') as f:
    content = f.read()

# Replace .Once() with nothing for GetByID
pattern = r'(h\.taskRepo\.On\("GetByID", mock\.Anything, taskID\)\.Return\(task, nil\))\.Once\(\)'
content = re.sub(pattern, r'\1', content)

with open('internal/service/orchestrator_service_test.go', 'w') as f:
    f.write(content)

