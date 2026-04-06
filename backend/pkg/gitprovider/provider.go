package gitprovider

import (
	"context"
	"errors"
	"io"
)

// Sentinel-ошибки для маппинга на HTTP и единообразной обработки в сервисах.
var (
	// ErrAuthFailed — невалидные credentials или нет доступа.
	ErrAuthFailed = errors.New("gitprovider: authentication failed")

	// ErrRepoNotFound — репозиторий не найден (404).
	ErrRepoNotFound = errors.New("gitprovider: repository not found")

	// ErrBranchNotFound — ветка не найдена.
	ErrBranchNotFound = errors.New("gitprovider: branch not found")

	// ErrBranchAlreadyExists — ветка уже существует.
	ErrBranchAlreadyExists = errors.New("gitprovider: branch already exists")

	// ErrPRNotFound — Pull Request не найден.
	ErrPRNotFound = errors.New("gitprovider: pull request not found")

	// ErrPRAlreadyExists — Pull Request для этой ветки уже существует (идемпотентность оркестратора).
	ErrPRAlreadyExists = errors.New("gitprovider: pull request already exists")

	// ErrFileNotFound — файл не найден в репозитории.
	ErrFileNotFound = errors.New("gitprovider: file not found")

	// ErrConflict — конфликт при пуше или мерже.
	ErrConflict = errors.New("gitprovider: conflict detected")

	// ErrRateLimited — превышен лимит API.
	ErrRateLimited = errors.New("gitprovider: rate limit exceeded")

	// ErrPermissionDenied — аутентификация есть, но операция запрещена (например protected branch).
	ErrPermissionDenied = errors.New("gitprovider: permission denied")

	// ErrCloneFailed — ошибка клонирования (сеть, диск, таймаут).
	ErrCloneFailed = errors.New("gitprovider: clone failed")

	// ErrNotImplemented — операция не поддерживается данным провайдером (например PR у LocalGitProvider).
	ErrNotImplemented = errors.New("gitprovider: operation not implemented for this provider")
)

// Factory создаёт GitProvider с уже расшифрованными credentials.
// providerType — строка в духе "github", "gitlab", "local" (как string(models.GitProvider)).
// Реализация с конструкторами — задача 4.5 (factory.go).
type Factory interface {
	Create(providerType string, creds Credentials) (GitProvider, error)
}

// GitProvider — универсальный контракт для работы с git-репозиториями.
// Реализации: GitHubProvider (4.3), LocalGitProvider (4.4); далее GitLab, Bitbucket.
//
// Гибридная модель (sandbox + API): в основном пайплайне разработки clone/branch/commit/push
// выполняет git CLI внутри Docker sandbox (entrypoint.sh); методы Clone, CreateBranch, Commit, Push
// здесь нужны для импорта проекта (ProjectService), LocalGitProvider без sandbox и утилитарных сценариев.
// Операции PR, diff по remote, метаданные репо — через API провайдера на стороне Go-бэкенда.
//
// Типы опций и DTO — в types.go того же пакета; зависимости от internal/models нет.
type GitProvider interface {
	// ValidateAccess проверяет credentials и доступ к репозиторию.
	ValidateAccess(ctx context.Context, repoURL string) error

	// Clone клонирует репозиторий; repoURL — источник, остальное в opts (DestPath, Branch, Depth).
	// Credentials подставляет реализация (URL с токеном, git config и т.д.).
	Clone(ctx context.Context, repoURL string, opts CloneOptions) error

	// CreateBranch создаёт ветку от базовой в локальном клоне; workDir — путь к клону на диске.
	CreateBranch(ctx context.Context, workDir string, opts BranchOptions) error

	// ListBranches возвращает ветки через REMOTE API; prefix фильтрует по префиксу имени.
	// MVP: без пагинации; post-MVP — см. задачу 4.1 (ListBranchesOptions / курсор).
	// LocalGitProvider: ErrNotImplemented — использовать ListLocalBranches.
	ListBranches(ctx context.Context, repoURL string, prefix string) ([]string, error)

	// ListLocalBranches — ветки в локальном клоне; поддерживают все провайдеры.
	ListLocalBranches(ctx context.Context, workDir string, prefix string) ([]string, error)

	// DeleteBranch удаляет ветку на remote; если ветки нет — ErrBranchNotFound.
	// LocalGitProvider: ErrNotImplemented — использовать DeleteLocalBranch.
	DeleteBranch(ctx context.Context, repoURL string, branch string) error

	// DeleteLocalBranch удаляет ветку в локальном клоне; поддерживают все провайдеры.
	DeleteLocalBranch(ctx context.Context, workDir string, branch string) error

	// Commit создаёт локальный коммит без push; workDir — директория с клоном.
	Commit(ctx context.Context, workDir string, opts CommitOptions) (commitSHA string, hasChanges bool, err error)

	// Push отправляет коммиты в remote (отдельно от Commit для retry и нескольких коммитов).
	Push(ctx context.Context, workDir string, opts PushOptions) error

	// CommitAndPush — Commit затем Push; workDir общий для обоих шагов.
	CommitAndPush(ctx context.Context, workDir string, commitOpts CommitOptions, pushOpts PushOptions) (commitSHA string, hasChanges bool, err error)

	// GetDiff unified diff base..head через REMOTE API (например GitHub compare).
	// LocalGitProvider: ErrNotImplemented — использовать GetLocalDiff.
	// Вызывающий обязан закрыть ReadCloser: defer rc.Close().
	GetDiff(ctx context.Context, repoURL string, base, head string) (io.ReadCloser, error)

	// GetLocalDiff — git diff в workDir; все провайдеры.
	// LocalGitProvider: Close() должен дождаться cmd.Wait() (без зомби-процессов).
	// Вызывающий обязан закрыть ReadCloser: defer rc.Close().
	GetLocalDiff(ctx context.Context, workDir string, base, head string) (io.ReadCloser, error)

	// CreatePullRequest создаёт PR/MR через API. LocalGitProvider: ErrNotImplemented.
	CreatePullRequest(ctx context.Context, repoURL string, opts PRCreateOptions) (*PullRequest, error)

	// UpdatePullRequest обновляет заголовок/описание PR. LocalGitProvider: ErrNotImplemented.
	UpdatePullRequest(ctx context.Context, repoURL string, number int, opts PRUpdateOptions) (*PullRequest, error)

	// GetPullRequest получает PR по номеру. LocalGitProvider: ErrNotImplemented.
	GetPullRequest(ctx context.Context, repoURL string, number int) (*PullRequest, error)

	// ListPullRequests список PR с фильтрами (PROptions); идемпотентный поиск по ветке.
	// LocalGitProvider: ErrNotImplemented.
	ListPullRequests(ctx context.Context, repoURL string, opts PROptions) ([]PullRequest, error)

	// ListPRFiles файлы PR; MVP без пагинации. LocalGitProvider: ErrNotImplemented.
	ListPRFiles(ctx context.Context, repoURL string, number int) ([]PRFile, error)

	// ListPRComments все комментарии к PR. LocalGitProvider: ErrNotImplemented.
	ListPRComments(ctx context.Context, repoURL string, number int) ([]PRComment, error)

	// AddPRComment общий комментарий к PR. LocalGitProvider: ErrNotImplemented.
	AddPRComment(ctx context.Context, repoURL string, number int, body string) error

	// AddPRReviewComment комментарий к строке. LocalGitProvider: ErrNotImplemented.
	AddPRReviewComment(ctx context.Context, repoURL string, number int, opts PRReviewCommentOptions) error

	// SubmitPRReview итог ревью (approve / request changes / comment). LocalGitProvider: ErrNotImplemented.
	SubmitPRReview(ctx context.Context, repoURL string, number int, opts PRReviewOptions) error

	// MergePullRequest merge через API; SHA в PRMergeOptions обязателен против гонок.
	// При конфликтах или проваленных проверках — ErrConflict. LocalGitProvider: ErrNotImplemented.
	MergePullRequest(ctx context.Context, repoURL string, number int, opts PRMergeOptions) error

	// GetRepoInfo метаданные репозитория с API. LocalGitProvider: ErrNotImplemented.
	GetRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error)

	// GetFileContent содержимое файла из ветки через REMOTE API.
	// Вызывающий обязан закрыть ReadCloser. LocalGitProvider: ErrNotImplemented — GetLocalFileContent.
	GetFileContent(ctx context.Context, repoURL string, branch string, path string) (io.ReadCloser, error)

	// GetLocalFileContent git show ref:path в workDir; все провайдеры.
	// LocalGitProvider: Close() должен вызвать cmd.Wait().
	// Вызывающий обязан закрыть ReadCloser.
	GetLocalFileContent(ctx context.Context, workDir string, ref string, path string) (io.ReadCloser, error)

	// ProviderType идентификатор провайдера: "github", "gitlab", "local", ...
	ProviderType() string

	// SupportsPullRequests false для LocalGitProvider — оркестратор адаптирует пайплайн.
	SupportsPullRequests() bool
}
