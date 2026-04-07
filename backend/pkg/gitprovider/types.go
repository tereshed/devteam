package gitprovider

import (
	"fmt"
	"strings"
	"time"
)

// --- Константы: ревью PR ---

// PRReviewEvent определяет допустимые значения для PRReviewOptions.Event.
const (
	ReviewEventApprove        = "APPROVE"
	ReviewEventRequestChanges = "REQUEST_CHANGES"
	ReviewEventComment        = "COMMENT"
)

// --- Константы: метод слияния PR ---

// MergeMethod определяет допустимые методы слияния PR.
const (
	MergeMethodMerge  = "merge"
	MergeMethodSquash = "squash"
	MergeMethodRebase = "rebase"
)

// --- Константы: состояние PR ---

// PRState определяет допустимые значения для PullRequest.State и PROptions.State.
const (
	PRStateOpen   = "open"
	PRStateClosed = "closed"
	PRStateMerged = "merged"
	PRStateAll    = "all" // только для фильтрации в PROptions.State
)

// --- Константы: тип комментария ---

const (
	CommentTypeIssue  = "issue_comment"
	CommentTypeReview = "review_comment"
)

// --- Константы: статус файла в PR ---

// FileStatus определяет допустимые значения для PRFile.Status.
const (
	FileStatusAdded    = "added"
	FileStatusRemoved  = "removed"
	FileStatusModified = "modified"
	FileStatusRenamed  = "renamed"
)

// PRDiffSide — сторона unified diff для review-комментариев (GitHub REST: LEFT / RIGHT).
const (
	PRDiffSideLeft  = "LEFT"
	PRDiffSideRight = "RIGHT"
)

// Credentials содержит расшифрованные данные для доступа к Git-провайдеру.
// Вызывающий код отвечает за расшифровку (AES-256-GCM, Sprint 4.7)
// и маппинг из слоя приложения (сущность учётных данных БД) в эту структуру.
type Credentials struct {
	// Token — Personal Access Token / OAuth token.
	// GitHub: ghp_xxx, GitLab: glpat-xxx.
	Token string

	// SSHKey — приватный SSH-ключ (PEM-формат).
	// Используется вместо Token для SSH-клонирования.
	SSHKey string

	// SSHKeyPassphrase — пароль для SSH-ключа (если есть).
	SSHKeyPassphrase string

	// Username — имя пользователя (для Basic Auth).
	// В качестве пароля в этом случае используется поле Token.
	// Большинство провайдеров используют Token как единственный способ.
	Username string
}

// Author задаёт автора git-коммита.
// Разделение Name/Email вместо строки "Name <email>" гарантирует корректный
// парсинг при передаче в `git commit --author` или go-git Signature.
type Author struct {
	Name  string // "DevTeam Bot"
	Email string // "bot@devteam.io"
}

// String форматирует Author для git CLI: "Name <email>".
func (a Author) String() string {
	return fmt.Sprintf("%s <%s>", a.Name, a.Email)
}

// Validate проверяет поля автора перед передачей в git commit --author.
// Нулевое значение (оба поля пустые после trim) — валидно: git возьмёт автора из config.
func (a Author) Validate() error {
	name := strings.TrimSpace(a.Name)
	email := strings.TrimSpace(a.Email)
	if name == "" && email == "" {
		return nil
	}
	if name == "" || email == "" {
		return fmt.Errorf("gitprovider: author requires non-empty name and email")
	}
	if strings.ContainsAny(name, "\n<>") || strings.ContainsAny(email, "\n<>") {
		return fmt.Errorf("gitprovider: author name and email must not contain newlines or angle brackets")
	}
	return nil
}

// CloneOptions задаёт параметры для GitProvider.Clone.
// repoURL передаётся отдельным аргументом метода (не в структуре).
type CloneOptions struct {
	// DestPath — локальная директория, куда будет клонирован репозиторий.
	// После Clone эта директория становится workDir для остальных операций.
	// Директория НЕ ДОЛЖНА существовать (Clone создаёт её).
	DestPath string

	// Branch — ветка для клонирования. Пустая строка = default branch провайдера.
	Branch string

	// Depth — глубина клонирования (shallow clone).
	// 0 = полный клон (все коммиты). >0 = последние N коммитов.
	// Для импорта проекта (индексация): 0 (полный).
	// Для sandbox: 50 (достаточно для diff с main).
	Depth int
}

// BranchOptions задаёт параметры для GitProvider.CreateBranch.
// workDir передаётся отдельным аргументом метода (не в структуре).
type BranchOptions struct {
	// BranchName — имя создаваемой ветки.
	// Конвенция: "task/<uuid>" для задач агентов.
	BranchName string

	// BaseBranch — ветка, от которой создаётся новая.
	// Пустая строка = текущая ветка (HEAD).
	BaseBranch string
}

// CommitOptions задаёт параметры для GitProvider.Commit и CommitAndPush.
// workDir передаётся отдельным аргументом метода (не в структуре).
type CommitOptions struct {
	// Message — текст коммит-сообщения.
	Message string

	// Author — автор коммита. Если не задан (zero value),
	// реализация использует Author по умолчанию из git config.
	Author Author

	// Files — список файлов для добавления в коммит.
	// Пустой слайс / nil = `git add -A` (все изменения).
	// Непустой = `git add <file1> <file2> ...` (только указанные).
	Files []string
}

// PushOptions задаёт параметры для GitProvider.Push и CommitAndPush.
// workDir передаётся отдельным аргументом метода (не в структуре).
type PushOptions struct {
	// Branch — имя ветки для пуша. Обязательно.
	Branch string

	// Force — принудительный пуш (git push --force).
	// ⚠️ Использовать с осторожностью! Перезаписывает историю remote.
	Force bool

	// Remote — имя remote (по умолчанию "origin").
	Remote string
}

// PRCreateOptions задаёт параметры для GitProvider.CreatePullRequest.
// repoURL передаётся отдельным аргументом метода (не в структуре).
type PRCreateOptions struct {
	// Title — заголовок PR.
	Title string

	// Body — описание PR (Markdown).
	Body string

	// HeadBranch — ветка с изменениями (source).
	HeadBranch string

	// BaseBranch — целевая ветка (куда мержить). Обычно "main".
	BaseBranch string

	// Draft — создать как черновик (draft PR).
	Draft bool

	// Labels — метки для PR (например, ["devteam", "auto-generated"]).
	Labels []string
}

// PRUpdateOptions задаёт параметры для GitProvider.UpdatePullRequest.
// Все поля — указатели: nil = не менять, non-nil = обновить.
type PRUpdateOptions struct {
	// Title — новый заголовок PR.
	Title *string

	// Body — новое описание PR.
	Body *string

	// State — новое состояние ("open", "closed").
	State *string
}

// PROptions задаёт фильтры для GitProvider.ListPullRequests.
// repoURL передаётся отдельным аргументом метода (не в структуре).
type PROptions struct {
	// State — фильтр по состоянию (см. константы PRState*).
	// Пустая строка = "open" (по умолчанию).
	State string

	// HeadBranch — фильтр по ветке-источнику.
	// Используется оркестратором для поиска существующего PR по имени ветки задачи.
	HeadBranch string

	// BaseBranch — фильтр по целевой ветке.
	BaseBranch string

	// Limit — максимальное количество PR в ответе.
	// 0 = значение по умолчанию провайдера (обычно 30).
	Limit int
}

// PRReviewCommentOptions задаёт параметры для GitProvider.AddPRReviewComment.
type PRReviewCommentOptions struct {
	// Body — текст комментария (Markdown).
	Body string

	// Path — путь к файлу относительно корня репозитория.
	Path string

	// Line — реальный номер строки в итоговом файле (НЕ индекс в diff-патче).
	// В GitHub API это поле `line` (не `position`), используется совместно с Side.
	Line int

	// CommitSHA — SHA коммита, к которому привязан комментарий.
	// Обязательно для GitHub API.
	CommitSHA string

	// Side — сторона diff: PRDiffSideLeft (старая) или PRDiffSideRight (новая).
	// Пустая строка при запросе: реализация обычно подставляет PRDiffSideRight.
	Side string
}

// PRReviewOptions задаёт параметры для GitProvider.SubmitPRReview.
type PRReviewOptions struct {
	// Event — тип решения:
	//   "APPROVE" — одобрить PR
	//   "REQUEST_CHANGES" — запросить изменения
	//   "COMMENT" — оставить комментарий без решения
	Event string

	// Body — текст общего комментария к ревью (опционально).
	Body string
}

// PRMergeOptions задаёт параметры для GitProvider.MergePullRequest.
type PRMergeOptions struct {
	// CommitTitle — заголовок merge-коммита (для squash/merge).
	// Пустая строка = автоматический заголовок провайдера.
	CommitTitle string

	// CommitMessage — тело merge-коммита.
	CommitMessage string

	// MergeMethod — метод слияния: "merge", "squash", "rebase".
	// Пустая строка = "merge" (по умолчанию).
	MergeMethod string

	// SHA — SHA последнего коммита head-ветки.
	// ⚠️ ОБЯЗАТЕЛЬНО: Защита от race conditions — если SHA не совпадает
	// с HEAD ветки, мерж будет отклонён. Предотвращает мерж чужого коммита,
	// запушенного после нашего аппрува.
	SHA string
}

// PullRequest содержит информацию о Pull Request / Merge Request.
type PullRequest struct {
	// Number — номер PR (уникальный в рамках репозитория).
	Number int

	// Title — заголовок PR.
	Title string

	// Body — описание PR (Markdown).
	Body string

	// State — текущее состояние (см. константы PRState*).
	State string

	// HeadBranch — ветка-источник (откуда изменения).
	HeadBranch string

	// BaseBranch — целевая ветка (куда мержить).
	BaseBranch string

	// HeadSHA — SHA последнего коммита в head-ветке.
	HeadSHA string

	// HTMLURL — ссылка на PR в браузере.
	// Сохраняется в task.artifacts для отображения в UI.
	HTMLURL string

	// Draft — true если PR в статусе черновика.
	Draft bool

	// Mergeable — true если PR можно смержить (нет конфликтов, проверки пройдены).
	// Может быть nil, если провайдер ещё не вычислил (GitHub: "unknown").
	Mergeable *bool

	// AuthorLogin — логин автора PR.
	AuthorLogin string

	// Labels — метки PR.
	Labels []string

	// CreatedAt — дата создания.
	CreatedAt time.Time

	// UpdatedAt — дата последнего обновления.
	UpdatedAt time.Time

	// MergedAt — дата мержа. Zero time если не смержен.
	MergedAt time.Time

	// ClosedAt — дата закрытия. Zero time если открыт.
	ClosedAt time.Time

	// ChangedFiles — количество изменённых файлов.
	ChangedFiles int

	// Additions — количество добавленных строк.
	Additions int

	// Deletions — количество удалённых строк.
	Deletions int
}

// PRFile описывает один файл, изменённый в Pull Request.
type PRFile struct {
	// Filename — путь к файлу относительно корня репозитория.
	Filename string

	// Status — тип изменения (см. константы FileStatus*).
	Status string

	// Additions — количество добавленных строк в этом файле.
	Additions int

	// Deletions — количество удалённых строк в этом файле.
	Deletions int

	// Changes — по контракту: Additions + Deletions.
	// При маппинге из GitHub/GitLab не копировать поле `changes` от API вслепую:
	// если семантика провайдера иная — явно привести к Additions+Deletions.
	Changes int

	// PreviousFilename — предыдущее имя файла (если Status == "renamed").
	PreviousFilename string

	// Patch — unified diff для этого файла (может быть пустым для бинарных файлов).
	Patch string
}

// PRComment описывает один комментарий к Pull Request.
// Объединяет Issue Comments (общие) и Review Comments (к строкам кода).
type PRComment struct {
	// ID — уникальный идентификатор комментария.
	ID int64

	// Body — текст комментария (Markdown).
	Body string

	// AuthorLogin — логин автора комментария.
	AuthorLogin string

	// CreatedAt — дата создания.
	CreatedAt time.Time

	// UpdatedAt — дата последнего обновления.
	UpdatedAt time.Time

	// Type — тип комментария: "issue_comment" (общий) или "review_comment" (к строке кода).
	Type string

	// --- Поля только для review_comment (Type == "review_comment") ---

	// Path — путь к файлу (только для review_comment).
	Path string

	// Line — номер строки (только для review_comment).
	Line int

	// CommitSHA — SHA коммита (только для review_comment).
	CommitSHA string

	// Side — сторона diff: PRDiffSideLeft или PRDiffSideRight.
	// Заполняется только для review_comment.
	Side string
}

// RepoInfo содержит базовую метаинформацию о репозитории.
type RepoInfo struct {
	// Name — имя репозитория (без owner).
	Name string

	// FullName — полное имя (owner/repo).
	FullName string

	// Description — описание репозитория.
	Description string

	// DefaultBranch — ветка по умолчанию (main, master).
	DefaultBranch string

	// HTMLURL — ссылка на репозиторий в браузере.
	HTMLURL string

	// CloneURL — URL для HTTPS-клонирования.
	CloneURL string

	// SSHURL — URL для SSH-клонирования.
	SSHURL string

	// Private — true если репозиторий приватный.
	Private bool

	// Archived — true если репозиторий архивирован (read-only).
	Archived bool

	// Language — основной язык программирования.
	Language string

	// Topics — теги/топики репозитория.
	Topics []string
}
