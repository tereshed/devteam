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
	ErrAntigravitySubscriptionNotFound = errors.New("antigravity subscription not found")

	// Ошибки GitIntegrationCredential (UI Refactoring Stage 3a).
	ErrGitIntegrationNotFound = errors.New("git integration credential not found")

	// Sprint 15.24 — реестр MCP-серверов и agent_skills.
	ErrMCPServerRegistryNotFound = errors.New("mcp server registry entry not found")
	ErrAgentSkillNotFound        = errors.New("agent skill not found")

	// Sprint 21 — Assistant Sidebar (docs/tasks/21-assistant-sidebar.md).
	// ErrAssistantSessionNotFound — сессии нет или принадлежит другому пользователю (один код для обоих,
	// чтобы handler возвращал 404 без раскрытия факта существования чужой сессии).
	ErrAssistantSessionNotFound = errors.New("assistant session not found")
	// ErrAssistantSessionBusy — CAS-захват busy провалился: уже идёт агент-петля. Handler → 409 Conflict.
	ErrAssistantSessionBusy = errors.New("assistant session is busy")
	// ErrAssistantSessionNotBusy — попытка освободить/подтвердить незахваченную сессию.
	ErrAssistantSessionNotBusy = errors.New("assistant session is not busy")
	// ErrAssistantMessageDuplicate — конфликт уникального индекса по (session_id, client_message_id):
	// идемпотентный повтор user-сообщения. Service конвертирует в no-op 202.
	ErrAssistantMessageDuplicate = errors.New("assistant message with this client_message_id already exists")
	// ErrAssistantNoPendingConfirmation — confirm пришёл без активного pending_tool_call_id или с mismatch.
	ErrAssistantNoPendingConfirmation = errors.New("assistant session has no pending tool call confirmation")
	// ErrAssistantAlreadyConfirmed — параллельный confirm уже закрыл tool-row (tool_result IS NOT NULL).
	ErrAssistantAlreadyConfirmed = errors.New("assistant tool call already confirmed")
)
