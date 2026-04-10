// Package sandbox — контракт Docker sandbox (Sprint 5). Согласован с deployment/sandbox/claude/entrypoint.sh.
package sandbox

// Имена переменных окружения для SandboxRunner / entrypoint.
const (
	EnvRepoURL          = "REPO_URL"
	EnvBranchName       = "BRANCH_NAME"
	EnvTaskInstruction  = "TASK_INSTRUCTION"
	EnvTaskContext      = "TASK_CONTEXT"
	EnvBaseRef          = "BASE_REF"
	EnvGitDefaultBranch = "GIT_DEFAULT_BRANCH"
	EnvBackend          = "BACKEND"
	EnvAnthropicAPIKey  = "ANTHROPIC_API_KEY"
	EnvMaxTurns         = "MAX_TURNS"
)

// Фиксированные пути артефактов внутри контейнера (не из env — защита от path injection).
const (
	WorkspacePath   = "/workspace"
	RepoPath        = "/workspace/repo"
	PromptFilePath  = "/workspace/prompt.txt"
	ContextFilePath = "/workspace/context.txt"
	AgentLogPath    = "/workspace/agent.log"
	FullDiffPath    = "/workspace/full.diff"
	ChangesPath     = "/workspace/changes.txt"
	StatusJSONPath  = "/workspace/status.json"
)
