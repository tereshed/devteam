package sandbox

import "errors"

// Sentinel-ошибки пакета sandbox (маппинг на HTTP — на границе handler/apierror).
var (
	// ErrInvalidSandboxID — sandboxID не прошёл валидацию до вызова Docker API.
	ErrInvalidSandboxID = errors.New("sandbox: invalid sandbox id")
	// ErrSandboxNotFound — валидный ID, инстанс неизвестен раннеру.
	ErrSandboxNotFound = errors.New("sandbox: sandbox not found")
	// ErrSandboxAlreadyStopped — повторная остановка в недопустимом состоянии.
	ErrSandboxAlreadyStopped = errors.New("sandbox: sandbox already stopped")
	// ErrInvalidOptions — невалидные SandboxOptions в начале RunTask.
	ErrInvalidOptions = errors.New("sandbox: invalid options")
	// ErrInvalidTaskID — TaskID не прошёл формат (UUID или безопасное подмножество символов).
	ErrInvalidTaskID = errors.New("sandbox: invalid task id")
	// ErrInvalidProjectID — ProjectID не прошёл формат (когда поле задано и участвует в путях/именах — см. комментарии к SandboxOptions).
	ErrInvalidProjectID = errors.New("sandbox: invalid project id")
	// ErrStreamAlreadyActive — повторный StreamLogs при уже активном стриме (вариант А, MVP).
	ErrStreamAlreadyActive = errors.New("sandbox: log stream already active")
	// ErrInvalidBranchName — имя ветки не прошло ValidateBranchName (инъекции, пробелы, правила ref).
	ErrInvalidBranchName = errors.New("sandbox: invalid branch name")
	// ErrInvalidEnvKeys — ключи EnvVars не прошли ValidateEnvKeys (инъекция через PATH/LD_* и т.д.).
	ErrInvalidEnvKeys = errors.New("sandbox: invalid env keys")
	// ErrInvalidRepoURL — RepoURL не прошёл ValidateRepoURL (SSRF, file://, недопустимая схема).
	ErrInvalidRepoURL = errors.New("sandbox: invalid repo url")
	// ErrSandboxRunConflict — повторный RunTask при уже существующем контейнере для того же TaskID (политика без adopt).
	ErrSandboxRunConflict = errors.New("sandbox: run conflict for task id")
)
