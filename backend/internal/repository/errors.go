package repository

import "errors"

var (
	// Общие ошибки репозитория
	ErrInvalidInput = errors.New("invalid input")

	// Ошибки сущностей
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("user already exists")
	ErrProjectNotFound = errors.New("project not found")
	ErrProjectNameExists = errors.New("project with this name already exists")
	ErrAgentNotFound   = errors.New("agent not found")
	ErrTeamAgentNotFound = errors.New("agent not found for project team")

	// Ошибки Conversation
	ErrConversationNotFound      = errors.New("conversation not found")
	ErrInvalidConversationStatus = errors.New("invalid conversation status")

	// Ошибки ConversationMessage
	ErrMessageNotFound = errors.New("message not found")
	ErrInvalidMessageRole = errors.New("invalid message role")

	// Ошибки Task
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskConcurrentUpdate = errors.New("task was modified concurrently, please retry")
	// ErrTaskLocked — строка tasks уже залочена другой транзакцией (SELECT FOR UPDATE NOWAIT, SQLSTATE 55P03).
	ErrTaskLocked = errors.New("task row is locked by another transaction")

	// Ошибки LLMProvider (Sprint 15.10)
	ErrLLMProviderNotFound   = errors.New("llm provider not found")
	ErrLLMProviderNameExists = errors.New("llm provider with this name already exists")

	// Ошибки ClaudeCodeSubscription (Sprint 15.12)
	ErrClaudeCodeSubscriptionNotFound = errors.New("claude code subscription not found")

	// Sprint 15.24 — реестр MCP-серверов и agent_skills.
	ErrMCPServerRegistryNotFound = errors.New("mcp server registry entry not found")
	ErrAgentSkillNotFound        = errors.New("agent skill not found")
)
