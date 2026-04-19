package ws

import "errors"

// ErrEmptyProjectID — projectID не может быть пустым.
var ErrEmptyProjectID = errors.New("ws: project_id is empty")

// ErrClientNotFound — клиент с указанным ID не найден.
var ErrClientNotFound = errors.New("ws: client not found")
