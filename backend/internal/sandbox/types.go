// Package sandbox — контракт Docker sandbox (Sprint 5). Согласован с deployment/sandbox/claude/entrypoint.sh.
package sandbox

import "time"

// DefaultSandboxTimeout — жёсткий дефолт бизнес-таймаута задачи, если SandboxOptions.Timeout <= 0.
// Раннер (5.5) и таймеры (5.8) обязаны использовать EffectiveTimeout(), не планировать бесконечное выполнение по умолчанию.
const DefaultSandboxTimeout = 30 * time.Minute

// DefaultSandboxStopGrace — SIGTERM/ContainerStop до SIGKILL при ручном Stop, если StopGracePeriod <= 0 (5.8).
const DefaultSandboxStopGrace = 10 * time.Second

// Имена переменных окружения для SandboxRunner / entrypoint.
// Instruction и Context из SandboxOptions не имеют имён ENV в Go: большие тексты
// передаются только файлами (PromptFilePath, ContextFilePath), см. CopyToContainer в 5.5.
const (
	EnvRepoURL          = "REPO_URL"
	EnvBranchName       = "BRANCH_NAME"
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

// LogEntryMaxLineBytes — верхняя граница длины LogEntry.Line в байтах; длиннее нельзя одной записью
// (bufio.Scanner, OOM). Реализация StreamLogs обязана чанкировать (несколько LogEntry подряд).
const LogEntryMaxLineBytes = 16 * 1024

// CodeResultMaxArtifactBytes — верхний предел размера Diff и Output в памяти бэкенда (сбор артефактов 5.7).
const CodeResultMaxArtifactBytes = 1 << 20 // 1 MiB

// StreamLogsDefaultBuffer — рекомендуемая ёмкость буфера канала StreamLogs (элементы LogEntry); 5.6 может варьировать в пределах разумного.
const StreamLogsDefaultBuffer = 2048

// TaskContainerNamePrefix — префикс детерминированного имени контейнера для идемпотентности RunTask по TaskID (5.5).
const TaskContainerNamePrefix = "devteam-task-"

// CodeBackendType — CLI/рантайм внутри sandbox-образа (entrypoint BACKEND).
type CodeBackendType string

const (
	CodeBackendClaudeCode CodeBackendType = "claude-code"
	CodeBackendAider      CodeBackendType = "aider"
	CodeBackendCustom     CodeBackendType = "custom"
)

// SandboxStatusType — фаза жизненного цикла инстанса с точки зрения раннера.
type SandboxStatusType string

const (
	SandboxStatusCreating  SandboxStatusType = "creating"
	SandboxStatusRunning   SandboxStatusType = "running"
	SandboxStatusCompleted SandboxStatusType = "completed"
	SandboxStatusFailed    SandboxStatusType = "failed"
	SandboxStatusStopped   SandboxStatusType = "stopped"
	// SandboxStatusTimedOut — контейнер принудительно остановлен по SandboxOptions.Timeout (5.8), не путать с failed/stopped.
	SandboxStatusTimedOut SandboxStatusType = "timed_out"
)

// SandboxOptions — параметры запуска изоляции без доменных типов БД (ID задачи/проекта — строки).
//
// Безопасность логов/JSON: для вывода используйте LogSafe(), String() или MarshalJSON — не fmt.Printf("%+v", opts):
// такой вывод обходит fmt.Stringer и утечёт Instruction/Context и сырые EnvVars (секреты).
type SandboxOptions struct {
	TaskID string
	// ProjectID — опционально; в текущем контракте 5.5 не участвует в имени контейнера и хостовых путях.
	// Если позже ID попадёт в эти строки — сохраняйте ValidateProjectID (тот же формат, что TaskID) в Validate().
	ProjectID string

	Backend CodeBackendType
	Image   string
	// RepoURL — URL клона; до контейнера обязан пройти ValidateRepoURL(ctx, …) (схемы http/https/git/ssh, без file:// и SSRF-хостов, DNS).
	RepoURL string
	// Branch — имя git-ветки; до Docker/entrypoint обязана пройти ValidateBranchName (защита от flag/command injection).
	Branch string
	// Instruction и Context — большие полезные нагрузки; в DockerSandboxRunner передаются
	// в контейнер только через CopyToContainer/bind-mount в PromptFilePath / ContextFilePath, не через ENV.
	Instruction string
	Context     string

	// EnvVars: при передаче SandboxOptions по значению мапа копируется только по ссылке (shallow copy).
	// Реализация RunTask обязана первой строкой вызвать opts = opts.Clone() (или эквивалентную глубокую
	// копию EnvVars) до любой итерации или асинхронного использования — иначе concurrent map read/write.
	// Ключи проходят ValidateEnvKeys (белый список + APP_*, без PATH/LD_* и т.д.).
	EnvVars map[string]string

	// Timeout — бизнес-таймаут жизни задачи в изоляции (после успешного start контейнера, политика 5.5/5.8).
	// Ноль и отрицательные значения запрещены как «бесконечность»: используйте EffectiveTimeout() перед таймерами.
	Timeout       time.Duration
	// StopGracePeriod — время SIGTERM до SIGKILL при Stop (5.8). Ноль — DefaultSandboxStopGrace; <0 запрещено в Validate.
	StopGracePeriod time.Duration
	ResourceLimit   ResourceLimit

	// DisableNetwork: true — режим сети «none» (без исходящего интернета и без bridge к хосту).
	// false — контейнер в изолированной bridge-сети без доступа к внутренним сервисам хоста (БД, Redis и т.д.);
	// детали политики маршрутизации и egress — в реализации 5.5/compose.
	DisableNetwork bool
}

// Clone возвращает копию опций с глубокой копией EnvVars. Используйте в начале RunTask до чтения/передачи opts в горутины.
func (o SandboxOptions) Clone() SandboxOptions {
	res := o
	if o.EnvVars != nil {
		res.EnvVars = make(map[string]string, len(o.EnvVars))
		for k, v := range o.EnvVars {
			res.EnvVars[k] = v
		}
	} else {
		res.EnvVars = nil
	}
	return res
}

// SandboxInstance — созданный инстанс сразу после RunTask (без ожидания завершения агента).
type SandboxInstance struct {
	ID        string // доверенный sandboxID (см. ValidateSandboxID)
	TaskID    string
	Status    SandboxStatusType
	CreatedAt time.Time
}

// SandboxStatus — снимок состояния (GetStatus / финальный Wait).
// Status может быть SandboxStatusTimedOut при принудительной остановке по opts.Timeout.
type SandboxStatus struct {
	ID       string
	Status   SandboxStatusType
	ExitCode int
	// Logs — последние N строк (хвост); срез для O(1) ротации (кольцевой буфер в реализации), без += по одной строке.
	Logs []string
	// Result — только после сбора артефактов (ожидаемо при Status == completed); иначе nil.
	// Перед доступом к полям CodeResult используйте HasResult(); иначе panic при nil Result.
	Result     *CodeResult
	RunningFor time.Duration
}

// HasResult безопасен при s == nil; true только если Result не nil.
func (s *SandboxStatus) HasResult() bool {
	return s != nil && s.Result != nil
}

// CodeResult — артефакты после завершения (сбор 5.7: CopyFromContainer + status.json из StatusJSONPath).
//
// Логи: тип реализует slog.LogValuer — безопасное представление для slog (Diff/Output не целиком).
// fmt.Printf("%+v", result) и %#v обходят LogValue и могут утечь секреты — не использовать для отладки.
type CodeResult struct {
	Success bool
	// FilesChanged: при заполнении из git предпочтительно `git diff --name-only -z` + Split по 0x00,
	// а не парсинг вывода `--stat` (ломается на пробелах в путях и переименованиях).
	// Каждый элемент — относительный путь внутри корня репозитория (без ведущего /, без ..).
	// Реализация 5.7 и потребители обязаны санитизировать: filepath.Clean + проверка, что путь не выходит
	// за пределы корня клона (path traversal в UI/диффе).
	// MVP 5.7 (вариант A): nil до доработки entrypoint (5.2) с отдельным файлом name-only.
	FilesChanged []string
	// Diff — unified diff; реализация 5.7 обязана усекать до CodeResultMaxArtifactBytes байт перед отдачей в API/БД (OOM/DoS).
	Diff       string
	CommitHash string
	// Output — сырой вывод агента; усечение до CodeResultMaxArtifactBytes обязательно на стороне раннера (5.7).
	Output     string
	TokensUsed int
	Duration   time.Duration
	BranchName string
}

// ResourceLimit — лимиты контейнера (Docker cgroup, задача 5.9).
//
// NanoCPUs — как в docker.Resources.NanoCPUs: 1 CPU = 1_000_000_000. Ноль — «не задано»: в HostConfig подставляется
// DefaultSandboxNanoCPUs (не меньше 1 CPU). Значения < 0 отклоняются Validate.
//
// MemoryMB — мегабайты RAM; 0 — политический минимум (пол). Валидация отсекает переполнение int64 и значения выше потолка.
//
// DiskMB — зарезервировано: только 0 до реализации квоты диска (overlay/volume driver). Иначе Validate — ошибка.
//
// PIDsLimit — cgroup pids.max; 0 — применяется пол из ResourceLimitPolicy. Отрицательные значения запрещены.
type ResourceLimit struct {
	NanoCPUs  int64
	MemoryMB  int
	DiskMB    int
	PIDsLimit int
}

// LogEntry — одна порция логов из stdout/stderr контейнера.
//
// Рекомендация (5.4): при росте пакета имеет смысл вынести LogEntry и константы стрима в logs.go/stream.go
// в том же пакете без переименования типов.
//
// Если Error != nil, запись терминальная: стрим оборвался (сеть, рестарт Docker и т.д.);
// Line/Stderr могут быть пустыми. Оркестратор обязан проверять entry.Error после чтения из канала,
// иначе «тихое» завершение range по закрытому каналу скрывает сбой.
//
// Чанкирование: len(Line) не должен превышать LogEntryMaxLineBytes. Логическая строка без '\n'
// или длиннее лимита — несколько LogEntry подряд с одинаковым Stderr; Timestamp ненулевой только
// у первого чанка логической строки (см. stream_line_writer.go, 5.6).
// чтобы не упираться в bufio.MaxScanTokenSize и не раздувать память одной записью.
//
// SandboxID — ID контейнера (как в Wait/GetStatus); заполняется раннером для fan-in и мультиплексирования логов.
type LogEntry struct {
	SandboxID string
	Timestamp time.Time
	// Line — фрагмент не длиннее LogEntryMaxLineBytes байт (см. константу).
	Line   string
	Stderr bool
	Error  error
}
