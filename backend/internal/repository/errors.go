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

	// Ошибки Conversation
	ErrConversationNotFound      = errors.New("conversation not found")
	ErrInvalidConversationStatus = errors.New("invalid conversation status")

	// Ошибки Task
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskConcurrentUpdate = errors.New("task was modified concurrently, please retry")
)
