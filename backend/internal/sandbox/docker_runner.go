package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
)

// Политика образов (5.5): при отсутствии локально выполняем ImagePull и обязательно дочитываем тело ответа.
// Детальная настройка и вариант «только предзагрузка» — 5.10 / README.

const dockerOpDetachTimeout = 45 * time.Second

// DockerSandboxRunner — реализация SandboxRunner через Docker Engine API (задача 5.5).
type DockerSandboxRunner struct {
	cli     *client.Client
	stopper *dockerStopper
	allowed []string
	// allowedServiceImages — allowlist образов эфемерных сервис-сайдкаров (Sprint 22).
	// Пусто → DefaultAllowedSandboxServiceImages(). Сверяется в RunTask по каждому сервису.
	allowedServiceImages []string

	// limitPolicy — полы/потолки cgroup для ContainerCreate (5.9); по умолчанию DefaultResourceLimitPolicy().
	limitPolicy ResourceLimitPolicy

	// defaultTaskTimeout — при opts.Timeout <= 0 (5.10 / cfg.Sandbox); 0 — вести себя как DefaultSandboxTimeout.
	defaultTaskTimeout time.Duration

	// streamLogsEntryBuffer, если > 0, задаёт ёмкость буферизованного канала StreamLogs вместо StreamLogsDefaultBuffer (тесты / конфиг).
	streamLogsEntryBuffer int

	// publisher — публикатор логов (7.6).
	publisher LogPublisher

	mu sync.Mutex
	// instances — полный ID контейнера (64 hex).
	instances map[string]*instanceState
	// creating — TaskID → state между валидацией и успешным переносом в instances.
	creating map[string]*instanceState
}

// DefaultAllowedSandboxImages — allowlist по умолчанию (см. ValidateAllowedImage).
func DefaultAllowedSandboxImages() []string {
	return []string{
		"devteam/sandbox-claude:latest",
		"devteam/sandbox-claude:local",
		"devteam/sandbox-aider:latest",
		"devteam/sandbox-aider:local",
		// Sprint 16: Hermes Agent (Nous Research) sandbox image.
		"devteam/sandbox-hermes:latest",
		"devteam/sandbox-hermes:local",
	}
}

// DefaultAllowedSandboxServiceImages — allowlist образов сервис-сайдкаров по умолчанию.
// Расширяется через WithAllowedServiceImages из конфига/DI.
func DefaultAllowedSandboxServiceImages() []string {
	return []string{
		"postgres:16-alpine",
		"postgres:16",
		"postgres:15-alpine",
		"postgres:15",
	}
}

// RunnerOption — опциональная настройка DockerSandboxRunner (расширяем без ломки существующих вызовов конструктора).
type RunnerOption func(*DockerSandboxRunner)

// WithAllowedServiceImages задаёт allowlist образов сервис-сайдкаров (Sprint 22). Пусто игнорируется.
func WithAllowedServiceImages(images []string) RunnerOption {
	return func(r *DockerSandboxRunner) {
		if len(images) > 0 {
			r.allowedServiceImages = append([]string(nil), images...)
		}
	}
}

// WithStreamLogsEntryBuffer задаёт ёмкость канала StreamLogs (число слотов LogEntry). Значения <= 0 игнорируются.
func WithStreamLogsEntryBuffer(n int) RunnerOption {
	return func(r *DockerSandboxRunner) {
		if n > 0 {
			r.streamLogsEntryBuffer = n
		}
	}
}

// WithResourceLimitPolicy задаёт политику лимитов для RunTask (5.9). Нулевые поля нормализуются к дефолтам.
func WithResourceLimitPolicy(p ResourceLimitPolicy) RunnerOption {
	return func(r *DockerSandboxRunner) {
		r.limitPolicy = normalizeResourceLimitPolicy(p)
	}
}

// WithDefaultTaskTimeout задаёт таймаут задачи при SandboxOptions.Timeout <= 0 (значение из config.Sandbox, 5.10).
// Значения <= 0 игнорируются (остаётся DefaultSandboxTimeout).
func WithDefaultTaskTimeout(d time.Duration) RunnerOption {
	return func(r *DockerSandboxRunner) {
		if d > 0 {
			r.defaultTaskTimeout = d
		}
	}
}

// WithEventBus задаёт шину событий для трансляции логов (7.4/7.6).
// Deprecated: используйте WithLogPublisher с адаптером (7.6).
func WithEventBus(any) RunnerOption {
	return func(r *DockerSandboxRunner) {
		// Оставляем для совместимости, если нужно, но в 7.6 переходим на LogPublisher
	}
}

func (r *DockerSandboxRunner) effectiveTaskTimeout(opts SandboxOptions) time.Duration {
	if opts.Timeout > 0 {
		return opts.Timeout
	}
	if r.defaultTaskTimeout > 0 {
		return r.defaultTaskTimeout
	}
	return DefaultSandboxTimeout
}

func (r *DockerSandboxRunner) fallbackTaskTimeoutFromLabels() time.Duration {
	if r.defaultTaskTimeout > 0 {
		return r.defaultTaskTimeout
	}
	return DefaultSandboxTimeout
}

// NewDockerSandboxRunner создаёт раннер. cli не должен быть nil; allowedImages пустой — дефолты.
func NewDockerSandboxRunner(cli *client.Client, allowedImages []string, opts ...RunnerOption) *DockerSandboxRunner {
	allowed := append([]string(nil), allowedImages...)
	if len(allowed) == 0 {
		allowed = DefaultAllowedSandboxImages()
	}
	r := &DockerSandboxRunner{
		cli:                  cli,
		stopper:              newDockerStopper(cli),
		allowed:              allowed,
		allowedServiceImages: DefaultAllowedSandboxServiceImages(),
		limitPolicy:          normalizeResourceLimitPolicy(ResourceLimitPolicy{}),
		instances:            make(map[string]*instanceState),
		creating:             make(map[string]*instanceState),
	}
	for _, o := range opts {
		if o != nil {
			o(r)
		}
	}
	return r
}

func taskContainerName(taskID string) string {
	return TaskContainerNamePrefix + taskID
}

func sandboxBridgeNetworkName(taskID, executionID string) string {
	sum := sha256.Sum256([]byte(taskID + executionID))
	return fmt.Sprintf("devteam-sbx-%x", sum[:8])
}

func detachTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	base := context.Background()
	if parent != nil {
		base = context.WithoutCancel(parent)
	}
	return context.WithTimeout(base, d)
}

func ptrInt(v int) *int { return &v }

func mergeSandboxEnv(opts SandboxOptions) []string {
	// Обязательные пары в конце (перекрывают дубликаты ключей из EnvVars).
	var out []string
	// Sprint 16.C: HermesEnv (DEVTEAM_HERMES_*, HERMES_MCP_*) приоритет НИЖЕ
	// EnvVars — пользовательские переопределения через EnvVars побеждают.
	// Ключи прошли whitelist в ValidateEnvKeys (включая префикс HERMES_MCP_*).
	if opts.AgentSettings != nil && len(opts.AgentSettings.HermesEnv) > 0 {
		for k, v := range opts.AgentSettings.HermesEnv {
			if _, dup := opts.EnvVars[k]; dup {
				continue
			}
			if k == EnvRepoURL || k == EnvBranchName || k == EnvBackend {
				continue
			}
			out = append(out, k+"="+v)
		}
	}
	// MCPEnv (MCP_*) — резолвленные секреты MCP-серверов (Claude Code раскрывает
	// ${VAR} из env при чтении .mcp.json). Тот же приоритет, что у HermesEnv.
	if opts.AgentSettings != nil && len(opts.AgentSettings.MCPEnv) > 0 {
		for k, v := range opts.AgentSettings.MCPEnv {
			if _, dup := opts.EnvVars[k]; dup {
				continue
			}
			if k == EnvRepoURL || k == EnvBranchName || k == EnvBackend {
				continue
			}
			out = append(out, k+"="+v)
		}
	}
	for k, v := range opts.EnvVars {
		if k == EnvRepoURL || k == EnvBranchName || k == EnvBackend {
			continue
		}
		out = append(out, k+"="+v)
	}
	out = append(out,
		EnvRepoURL+"="+opts.RepoURL,
		EnvBranchName+"="+opts.Branch,
		EnvBackend+"="+string(opts.Backend),
	)
	// Мульти-репо: соседние репозитории — JSON-массив в SIBLING_REPOS. Entrypoint клонирует
	// их read-only тем же GIT_TOKEN (host-scoped credential helper).
	if len(opts.SiblingRepos) > 0 {
		if blob, err := json.Marshal(opts.SiblingRepos); err == nil {
			out = append(out, EnvSiblingRepos+"="+string(blob))
		}
	}
	// Sprint 22: connection-vars эфемерных сервис-сайдкаров (POSTGRES_*/DATABASE_URL/
	// SERVICE_READY_TIMEOUT первого сервиса). Дописываются последними → высший приоритет
	// (минуя ValidateEnvKeys — генерит runner, как REPO_URL/BRANCH_NAME).
	out = appendServiceConnEnv(out, opts.Services)
	// Sprint 15.22 / 15.M5: permission-mode для claude code CLI.
	// Жёстко валидируем значение по белому списку, чтобы инъекция вида "default\n--evil-flag"
	// или произвольный текст из БД не попадал в env контейнера.
	if opts.AgentSettings != nil && opts.AgentSettings.PermissionMode != "" {
		if isValidClaudeCodePermissionMode(opts.AgentSettings.PermissionMode) {
			out = append(out, EnvClaudeCodePermissionMode+"="+opts.AgentSettings.PermissionMode)
		}
		// Невалидный mode игнорируем молча — entrypoint выберет дефолт (--dangerously-skip-permissions).
		// Логирование происходит в сервис-уровне (AgentSettingsService.BuildArtifacts), здесь — defense-in-depth.
	}
	return out
}

// isValidClaudeCodePermissionMode — белый список значений для CLAUDE_CODE_PERMISSION_MODE
// (см. claude-code CLI --permission-mode и Sprint 15.21 IsValidPermissionMode).
func isValidClaudeCodePermissionMode(mode string) bool {
	switch mode {
	case "default", "acceptEdits", "plan", "bypassPermissions":
		return true
	default:
		return false
	}
}

// drainDockerWait освобождает каналы ContainerWait без вечной блокировки: после select в containerWaitLoop
// заполнено не более одного из каналов; второй может остаться пустым навсегда (буфер errC не гарантирует закрытие).
func drainDockerWait(respC <-chan containertypes.WaitResponse, errC <-chan error) {
	go func() {
		select {
		case <-respC:
		default:
		}
		select {
		case <-errC:
		default:
		}
	}()
}

func (r *DockerSandboxRunner) pullImage(ctx context.Context, ref string) error {
	rc, err := r.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull: %w", errors.Join(ErrSandboxDocker, err))
	}
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("image pull drain: %w", errors.Join(ErrSandboxDocker, err))
	}
	return nil
}

func (r *DockerSandboxRunner) ensureLocalImage(ctx context.Context, ref string) error {
	_, err := r.cli.ImageInspect(ctx, ref)
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("image inspect: %w", errors.Join(ErrSandboxDocker, err))
	}
	return r.pullImage(ctx, ref)
}

func (r *DockerSandboxRunner) removeContainerForceLogged(ctx context.Context, taskID, id, phase string) {
	if id == "" || r.cli == nil {
		return
	}
	rmCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()
	err := r.cli.ContainerRemove(rmCtx, id, containertypes.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !errdefs.IsNotFound(err) {
		slog.Warn("sandbox: rollback container remove", "task_id", taskID, "sandbox_id", id, "phase", phase, "err", err)
	}
}

func isNetworkRemoveRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "has active endpoints") ||
		strings.Contains(s, "being used") ||
		strings.Contains(s, "in use") ||
		strings.Contains(s, "resource is still in use")
}

// removeNetworkBestEffort удаляет сеть; при гонке с отключением контейнера от сети — короткие повторы.
func (r *DockerSandboxRunner) removeNetworkBestEffort(ctx context.Context, netID string) {
	if netID == "" {
		return
	}
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(150 * time.Millisecond):
			}
		}
		rmCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
		err := r.cli.NetworkRemove(rmCtx, netID)
		cancel()
		if err == nil || errdefs.IsNotFound(err) {
			return
		}
		if !isNetworkRemoveRetryable(err) {
			slog.Warn("sandbox: network remove", "network_id", netID, "err", err)
			return
		}
	}
	slog.Warn("sandbox: network remove retries exhausted", "network_id", netID)
}

// buildPromptContextTar упаковывает prompt.txt + context.txt (+ опционально settings.json и .mcp.json
// из AgentSettingsBundle, Sprint 15.22) в tar для CopyToContainer.
// Все пути относительно /workspace; settings.json кладётся в .claude/settings.json, .mcp.json — в repo/.mcp.json
// (entrypoint после clone положит его в корень репозитория).
//
// Sprint 16.C: tar содержит ТОЛЬКО /workspace-артефакты. Hermes-артефакты
// (~/.hermes/...) копируются вторым CopyToContainer-вызовом в корень контейнера —
// см. buildHermesHomeTar / RunTask.
func buildPromptContextTar(instruction, contextText string, settings *AgentSettingsBundle) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		var err error
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.CloseWithError(err)
		}()
		now := time.Now()

		type entry struct {
			name    string
			content []byte
			isDir   bool
		}
		entries := []entry{
			{name: "prompt.txt", content: []byte(instruction)},
			{name: "context.txt", content: []byte(contextText)},
		}
		if settings != nil {
			if len(settings.SettingsJSON) > 0 {
				entries = append(entries,
					entry{name: ".claude", isDir: true},
					entry{name: ".claude/settings.json", content: settings.SettingsJSON},
				)
			}
			if len(settings.MCPJSON) > 0 {
				// Сохраняем .mcp.json в /workspace; entrypoint после clone переносит его в repo/.
				entries = append(entries, entry{name: ".mcp.json", content: settings.MCPJSON})
			}
		}

		for _, f := range entries {
			// Контейнер запускается под non-root user sandbox (uid 1001, см. Dockerfile).
			// CopyToContainer сохраняет uid/gid/mode из tar-заголовка; без явных Uid/Gid
			// файл создаётся как root:root и недоступен на чтение sandbox-пользователю.
			hdr := &tar.Header{
				Name:    f.name,
				Mode:    0o644,
				Uid:     1001,
				Gid:     1001,
				ModTime: now,
			}
			if f.isDir {
				hdr.Typeflag = tar.TypeDir
				hdr.Mode = 0o755
			} else {
				hdr.Typeflag = tar.TypeReg
				hdr.Size = int64(len(f.content))
			}
			if err = tw.WriteHeader(hdr); err != nil {
				return
			}
			if !f.isDir {
				if _, err = io.Copy(tw, strings.NewReader(string(f.content))); err != nil {
					return
				}
			}
		}
	}()
	return pr, nil
}

// hermesHomeBase — корневая директория для tar Hermes-артефактов (relative to /).
// Файлы кладутся в `home/sandbox/.hermes/...`; CopyToContainer вызывается с dst="/".
const hermesHomeBase = "home/sandbox/.hermes"

// buildHermesHomeTar — Sprint 16.C: упаковывает ~/.hermes/{config.yaml, mcp.json,
// skills/<name>/<file>} в tar для CopyToContainer dst="/".
//
// Path-traversal: ключи Skills уже валидируются в HermesArtifactBuilder
// (assertSafeRelativePath), но runner повторяет проверку defense-in-depth —
// если кто-то соберёт AgentSettingsBundle минуя билдер, мы всё равно не запишем
// файл вне ~/.hermes/skills/.
//
// Permissions: config.yaml и mcp.json — 0600 (содержат секрет-ссылки и токены),
// директории — 0700, skills-файлы — 0644.
//
// Возвращает (nil, nil), если в bundle нет ни одного hermes-поля.
func buildHermesHomeTar(b *AgentSettingsBundle) (io.ReadCloser, error) {
	if b == nil {
		return nil, nil
	}
	if len(b.HermesConfigYAML) == 0 && len(b.HermesMCPJSON) == 0 && len(b.HermesSkills) == 0 {
		return nil, nil
	}
	type entry struct {
		name    string
		content []byte
		mode    int64
		isDir   bool
	}
	entries := []entry{
		{name: hermesHomeBase, mode: 0o700, isDir: true},
	}
	if len(b.HermesConfigYAML) > 0 {
		entries = append(entries, entry{
			name: hermesHomeBase + "/config.yaml", content: b.HermesConfigYAML, mode: 0o600,
		})
	}
	if len(b.HermesMCPJSON) > 0 {
		entries = append(entries, entry{
			name: hermesHomeBase + "/mcp.json", content: b.HermesMCPJSON, mode: 0o600,
		})
	}
	if len(b.HermesSkills) > 0 {
		entries = append(entries, entry{
			name: hermesHomeBase + "/skills", mode: 0o700, isDir: true,
		})
		// Стабильный порядок (детерминированный tar для логов / снапшотов).
		keys := make([]string, 0, len(b.HermesSkills))
		for k := range b.HermesSkills {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, rel := range keys {
			if err := assertHermesSkillRelPath(rel); err != nil {
				return nil, fmt.Errorf("hermes skill %q: %w", rel, err)
			}
			content := b.HermesSkills[rel]
			// Создаём промежуточные директории, если skill кладёт файл глубже одного уровня.
			parent := hermesHomeBase + "/skills/" + parentDirSlash(rel)
			if parent != hermesHomeBase+"/skills/" {
				entries = append(entries, entry{name: strings.TrimRight(parent, "/"), mode: 0o700, isDir: true})
			}
			entries = append(entries, entry{
				name: hermesHomeBase + "/skills/" + rel, content: content, mode: 0o644,
			})
		}
	}

	pr, pw := io.Pipe()
	go func() {
		var err error
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.CloseWithError(err)
		}()
		now := time.Now()
		for _, f := range entries {
			hdr := &tar.Header{
				Name:    f.name,
				Mode:    f.mode,
				Uid:     1001,
				Gid:     1001,
				ModTime: now,
			}
			if f.isDir {
				hdr.Typeflag = tar.TypeDir
			} else {
				hdr.Typeflag = tar.TypeReg
				hdr.Size = int64(len(f.content))
			}
			if err = tw.WriteHeader(hdr); err != nil {
				return
			}
			if !f.isDir {
				if _, err = io.Copy(tw, strings.NewReader(string(f.content))); err != nil {
					return
				}
			}
		}
	}()
	return pr, nil
}

// claudeSkillsHomeBase — каталог skills внутри контейнера (relative to /) для
// claude-семейства backend'ов. Возвращает "" для backend'ов без поддержки skills —
// caller обязан трактовать это как «не копировать».
//
//	claude-code → ~/.claude/skills (personal skills, CLI находит сам);
//	antigravity → ~/.gemini/antigravity/skills (глобальный каталог Antigravity;
//	              entrypoint дополнительно зеркалит в $REPO_DIR/.agents/skills).
func claudeSkillsHomeBase(backend CodeBackendType) string {
	switch backend {
	case CodeBackendClaudeCode:
		return "home/sandbox/.claude/skills"
	case CodeBackendAntigravity:
		return "home/sandbox/.gemini/antigravity/skills"
	default:
		return ""
	}
}

// buildClaudeSkillsTar — упаковывает AgentSettingsBundle.SkillsFiles в tar для
// CopyToContainer dst="/" (home-каталог лежит вне /workspace — тот же приём,
// что buildHermesHomeTar).
//
// Возвращает (nil, nil), если skills нет или backend их не поддерживает
// (последнее — программная ошибка конфигурации бандла; молча не копируем,
// предупреждение логирует caller).
//
// Permissions: директории — 0755, файлы — 0644 (exec-бит скриптам не нужен:
// CLI запускает их через bash/python3). Владелец — sandbox (uid/gid 1001).
func buildClaudeSkillsTar(b *AgentSettingsBundle, backend CodeBackendType) (io.ReadCloser, error) {
	if b == nil || len(b.SkillsFiles) == 0 {
		return nil, nil
	}
	base := claudeSkillsHomeBase(backend)
	if base == "" {
		return nil, fmt.Errorf("backend %q does not support skills files", backend)
	}
	type entry struct {
		name    string
		content []byte
		mode    int64
		isDir   bool
	}
	// Родительские каталоги до base: tar требует, чтобы они существовали с
	// правильным владельцем (иначе docker создаст их root:root).
	entries := make([]entry, 0, len(b.SkillsFiles)*2+4)
	parts := strings.Split(base, "/")
	for i := 2; i <= len(parts); i++ { // home/sandbox уже существует в образе
		entries = append(entries, entry{name: strings.Join(parts[:i], "/"), mode: 0o755, isDir: true})
	}
	keys := make([]string, 0, len(b.SkillsFiles))
	for k := range b.SkillsFiles {
		keys = append(keys, k)
	}
	sortStrings(keys)
	seenDirs := map[string]bool{}
	for _, rel := range keys {
		if err := assertHermesSkillRelPath(rel); err != nil {
			return nil, fmt.Errorf("skill file %q: %w", rel, err)
		}
		// Промежуточные директории skill'а (skill-name/, skill-name/scripts/, ...).
		segs := strings.Split(rel, "/")
		for i := 1; i < len(segs); i++ {
			dir := base + "/" + strings.Join(segs[:i], "/")
			if !seenDirs[dir] {
				seenDirs[dir] = true
				entries = append(entries, entry{name: dir, mode: 0o755, isDir: true})
			}
		}
		entries = append(entries, entry{name: base + "/" + rel, content: b.SkillsFiles[rel], mode: 0o644})
	}

	pr, pw := io.Pipe()
	go func() {
		var err error
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.CloseWithError(err)
		}()
		now := time.Now()
		for _, f := range entries {
			hdr := &tar.Header{
				Name:    f.name,
				Mode:    f.mode,
				Uid:     1001,
				Gid:     1001,
				ModTime: now,
			}
			if f.isDir {
				hdr.Typeflag = tar.TypeDir
			} else {
				hdr.Typeflag = tar.TypeReg
				hdr.Size = int64(len(f.content))
			}
			if err = tw.WriteHeader(hdr); err != nil {
				return
			}
			if !f.isDir {
				if _, err = io.Copy(tw, strings.NewReader(string(f.content))); err != nil {
					return
				}
			}
		}
	}()
	return pr, nil
}

// assertHermesSkillRelPath — defense-in-depth path-traversal проверка для tar-ключей.
// Дублирует логику service.assertSafeRelativePath (в sandbox-пакете не зависим
// от service/, поэтому повторяем), чтобы быть последней линией защиты до WriteHeader.
func assertHermesSkillRelPath(rel string) error {
	if rel == "" {
		return errors.New("empty path")
	}
	for i := 0; i < len(rel); i++ {
		if rel[i] == 0 {
			return errors.New("null byte in path")
		}
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "~") {
		return fmt.Errorf("absolute or home path %q not allowed", rel)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return fmt.Errorf("parent traversal segment in %q", rel)
		}
	}
	return nil
}

func parentDirSlash(rel string) string {
	idx := strings.LastIndex(rel, "/")
	if idx < 0 {
		return ""
	}
	return rel[:idx+1]
}

func sortStrings(s []string) {
	// Малый сорт без импорта sort — достаточно стабильности и предсказуемости в тестах.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// startServiceContainers поднимает эфемерные сервис-сайдкары на сети netName с alias-DNS.
// Возвращает ID всех СОЗДАННЫХ контейнеров (даже при ошибке на середине) — чтобы вызывающий
// rollback-defer снёс их. seed (если задан) копируется в /docker-entrypoint-initdb.d ДО старта.
func (r *DockerSandboxRunner) startServiceContainers(ctx context.Context, st *instanceState, opts SandboxOptions, netName, networkID, agentContainerName string) ([]string, error) {
	var ids []string
	pol := r.limitPolicy
	for i := range opts.Services {
		s := opts.Services[i]
		if err := r.ensureLocalImage(ctx, s.Image); err != nil {
			return ids, err
		}
		if err := st.errIfInitCancelled(); err != nil {
			return ids, err
		}
		svcName := fmt.Sprintf("%s-svc-%s", agentContainerName, s.Alias)
		if inspect, ierr := r.cli.ContainerInspect(ctx, svcName); ierr == nil {
			slog.Warn("sandbox: service run conflict, removing existing container", "task_id", opts.TaskID, "container_id", inspect.ID, "alias", s.Alias)
			r.removeContainerForceLogged(ctx, opts.TaskID, inspect.ID, "service_run_conflict")
		} else if !errdefs.IsNotFound(ierr) {
			return ids, fmt.Errorf("inspect service container name: %w", errors.Join(ErrSandboxDocker, ierr))
		}

		initTrue := true
		memBytes := effectiveMemoryBytes(s.ResourceLimit.MemoryMB, pol.MemoryFloorBytes, pol.MemoryCeilBytes)
		pidsLim := effectivePidsLimit(s.ResourceLimit.PIDsLimit, pol.PidsFloor, pol.PidsCeil)
		nanoCPUs := effectiveNanoCPUs(s.ResourceLimit.NanoCPUs, pol.DefaultNanoCPUs, pol.NanoCPUsCeil)
		hc := &containertypes.HostConfig{
			NetworkMode: containertypes.NetworkMode(netName),
			Init:        &initTrue,
			LogConfig: containertypes.LogConfig{
				Type:   "json-file",
				Config: map[string]string{"max-size": "10m", "max-file": "3"},
			},
			Resources: containertypes.Resources{
				Memory:     memBytes,
				MemorySwap: memBytes,
				NanoCPUs:   nanoCPUs,
				PidsLimit:  &pidsLim,
			},
			ReadonlyRootfs: false,
		}
		cfg := &containertypes.Config{
			Image: s.Image,
			Env:   serviceEnvSlice(s.Env),
			Labels: map[string]string{
				"devteam.sandbox":    "1",
				"devteam.task_id":    opts.TaskID,
				"devteam.service":    "1",
				"devteam.service_of": agentContainerName,
				"devteam.network_id": networkID,
			},
		}
		netCfg := &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				netName: {Aliases: []string{s.Alias}},
			},
		}
		createResp, cerr := r.cli.ContainerCreate(ctx, cfg, hc, netCfg, nil, svcName)
		if cerr != nil {
			return ids, fmt.Errorf("service container create (%s): %w", s.Alias, errors.Join(ErrSandboxDocker, cerr))
		}
		ids = append(ids, createResp.ID)
		st.mu.Lock()
		st.serviceContainerIDs = append(st.serviceContainerIDs, createResp.ID)
		st.mu.Unlock()

		// Сид: postgres-образ прогоняет *.sql из /docker-entrypoint-initdb.d на первой
		// инициализации (контейнер всегда свежий → всегда «первая»). Копируем ДО старта.
		if strings.TrimSpace(s.SeedSQL) != "" {
			seedRC, serr := buildServiceSeedTar(s.SeedSQL)
			if serr != nil {
				return ids, fmt.Errorf("build seed tar (%s): %w", s.Alias, serr)
			}
			cpErr := r.cli.CopyToContainer(ctx, createResp.ID, "/docker-entrypoint-initdb.d", seedRC, containertypes.CopyToContainerOptions{})
			seedRC.Close()
			if cpErr != nil {
				return ids, fmt.Errorf("copy seed to service (%s): %w", s.Alias, errors.Join(ErrSandboxDocker, cpErr))
			}
		}

		if err := st.errIfInitCancelled(); err != nil {
			return ids, err
		}
		if serr := r.cli.ContainerStart(ctx, createResp.ID, containertypes.StartOptions{}); serr != nil {
			return ids, fmt.Errorf("service container start (%s): %w", s.Alias, errors.Join(ErrSandboxDocker, serr))
		}
	}
	return ids, nil
}

// buildServiceSeedTar упаковывает seed SQL в tar с единственным файлом seed.sql
// (CopyToContainer распакует в /docker-entrypoint-initdb.d). Размер ограничен maxServiceSeedBytes.
func buildServiceSeedTar(seedSQL string) (io.ReadCloser, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: "seed.sql", Mode: 0o644, Size: int64(len(seedSQL))}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(seedSQL)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// removeServiceContainersByNetwork сносит сервис-сайдкары прогона по labels (fallback в
// Cleanup, когда in-memory state потерян — например после рестарта процесса).
func (r *DockerSandboxRunner) removeServiceContainersByNetwork(ctx context.Context, taskID, netID string) {
	if netID == "" {
		return
	}
	f := filters.NewArgs()
	f.Add("label", "devteam.service=1")
	f.Add("label", "devteam.network_id="+netID)
	list, err := r.cli.ContainerList(ctx, containertypes.ListOptions{All: true, Filters: f})
	if err != nil {
		slog.Warn("sandbox: list service containers for cleanup", "network_id", netID, "err", err)
		return
	}
	for _, c := range list {
		r.removeContainerForceLogged(ctx, taskID, c.ID, "cleanup_service_by_label")
	}
}

// RunTask реализует SandboxRunner.RunTask.
func (r *DockerSandboxRunner) RunTask(ctx context.Context, opts SandboxOptions) (*SandboxInstance, error) {
	opts = opts.Clone()
	if err := opts.validateWithoutResourceLimits(ctx); err != nil {
		return nil, err
	}
	if err := opts.ValidateResourceLimits(r.limitPolicy); err != nil {
		return nil, err
	}
	if err := ValidateAllowedImage(opts.Image, r.allowed); err != nil {
		return nil, err
	}
	// Sprint 22: образ каждого сервис-сайдкара — против отдельного allowlist (как агент-образ).
	for i := range opts.Services {
		if err := ValidateAllowedImage(opts.Services[i].Image, r.allowedServiceImages); err != nil {
			return nil, fmt.Errorf("service %q: %w", opts.Services[i].Alias, err)
		}
	}
	if r.cli == nil {
		return nil, fmt.Errorf("docker client is nil: %w", ErrSandboxDocker)
	}

	cName := taskContainerName(opts.TaskID)
	if opts.ExecutionID != "" {
		cName = fmt.Sprintf("%s-%s", cName, opts.ExecutionID)
	}
	if inspect, err := r.cli.ContainerInspect(ctx, cName); err == nil {
		slog.Warn("sandbox: run conflict detected, removing existing container", "task_id", opts.TaskID, "container_id", inspect.ID)
		r.removeContainerForceLogged(ctx, opts.TaskID, inspect.ID, "run_conflict_resolution")
	} else if !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("inspect container name: %w", errors.Join(ErrSandboxDocker, err))
	}

	st := newInstanceState(opts.TaskID)
	st.containerName = cName
	st.stopGracePeriod = opts.EffectiveStopGrace()

	r.mu.Lock()
	if _, dup := r.creating[cName]; dup {
		r.mu.Unlock()
		return nil, ErrSandboxRunConflict
	}
	r.creating[cName] = st
	r.mu.Unlock()

	var (
		containerID   string
		networkID     string
		hostTmp       string
		serviceIDs    []string
		registeredRun = false
	)
	defer func() {
		r.mu.Lock()
		delete(r.creating, cName)
		r.mu.Unlock()
		if !registeredRun && containerID != "" {
			r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "run_task_defer")
		}
		// Сервис-сайдкары сносим ПОСЛЕ агент-контейнера и ДО сети (removeNetworkBestEffort
		// ретраит на «active endpoints», но так меньше повторов).
		if !registeredRun {
			for _, sid := range serviceIDs {
				r.removeContainerForceLogged(ctx, opts.TaskID, sid, "run_task_defer_service")
			}
		}
		if !registeredRun && networkID != "" {
			r.removeNetworkBestEffort(ctx, networkID)
		}
		if !registeredRun && hostTmp != "" {
			if rmErr := os.RemoveAll(hostTmp); rmErr != nil {
				slog.Warn("sandbox: rollback host temp", "task_id", opts.TaskID, "path", hostTmp, "err", rmErr)
			}
		}
	}()

	// prompt/context идут в контейнер только через tar в памяти (без хостового каталога).
	hostTmp = ""

	if err := r.ensureLocalImage(ctx, opts.Image); err != nil {
		return nil, err
	}
	if err := st.errIfInitCancelled(); err != nil {
		return nil, err
	}

	pol := r.limitPolicy
	var (
		netName  string
		netCfg   *network.NetworkingConfig
		hostNet  containertypes.NetworkMode
		initTrue = true
		memBytes = effectiveMemoryBytes(opts.ResourceLimit.MemoryMB, pol.MemoryFloorBytes, pol.MemoryCeilBytes)
		pidsLim  = effectivePidsLimit(opts.ResourceLimit.PIDsLimit, pol.PidsFloor, pol.PidsCeil)
		nanoCPUs = effectiveNanoCPUs(opts.ResourceLimit.NanoCPUs, pol.DefaultNanoCPUs, pol.NanoCPUsCeil)
		hc       = &containertypes.HostConfig{
			NetworkMode: hostNet,
			Init:        &initTrue,
			LogConfig: containertypes.LogConfig{
				Type: "json-file",
				Config: map[string]string{
					"max-size": "10m",
					"max-file": "3",
				},
			},
			Resources: containertypes.Resources{
				Memory:     memBytes,
				MemorySwap: memBytes,
				NanoCPUs:   nanoCPUs,
				PidsLimit:  &pidsLim,
			},
			ReadonlyRootfs: false,
		}
	)

	if opts.DisableNetwork {
		hc.NetworkMode = network.NetworkNone
		netCfg = &network.NetworkingConfig{}
		if err := st.errIfInitCancelled(); err != nil {
			return nil, err
		}
	} else {
		netName = sandboxBridgeNetworkName(opts.TaskID, opts.ExecutionID)
		// ICC (inter-container comms) выключен по умолчанию; для прогонов с сервис-
		// сайдкарами включаем, иначе агент не достучится до сервиса. Сеть приватна на один
		// прогон (только агент + его сервисы) → изоляция от других прогонов не слабеет.
		icc := "false"
		if len(opts.Services) > 0 {
			icc = "true"
		}
		netResp, nerr := r.cli.NetworkCreate(ctx, netName, network.CreateOptions{
			Driver: "bridge",
			Options: map[string]string{
				"com.docker.network.bridge.enable_icc": icc,
			},
			Labels: map[string]string{
				"devteam.sandbox": "1",
				"devteam.task_id": opts.TaskID,
			},
		})
		if nerr != nil {
			if errdefs.IsConflict(nerr) {
				slog.Warn("sandbox: network name conflict, reusing existing network", "name", netName)
				netInsp, ierr := r.cli.NetworkInspect(ctx, netName, network.InspectOptions{})
				if ierr != nil {
					return nil, fmt.Errorf("network inspect after conflict: %w", errors.Join(ErrSandboxDocker, ierr))
				}
				networkID = netInsp.ID
			} else {
				return nil, fmt.Errorf("network create: %w", errors.Join(ErrSandboxDocker, nerr))
			}
		} else {
			networkID = netResp.ID
		}
		if err := st.errIfInitCancelled(); err != nil {
			return nil, err
		}
		st.mu.Lock()
		st.networkID = networkID
		st.mu.Unlock()
		hc.NetworkMode = containertypes.NetworkMode(netName)
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				netName: {},
			},
		}
	}

	// Sprint 22: поднимаем эфемерные сервис-сайдкары на той же bridge-сети с alias-DNS
	// ДО агент-контейнера (агент ждёт их готовности в entrypoint). svcIDs возвращаются
	// даже при ошибке — чтобы rollback-defer снёс уже созданные.
	if len(opts.Services) > 0 {
		svcIDs, serr := r.startServiceContainers(ctx, st, opts, netName, networkID, cName)
		serviceIDs = append(serviceIDs, svcIDs...)
		if serr != nil {
			return nil, serr
		}
	}

	effTO := r.effectiveTaskTimeout(opts)
	timeoutSecs := int(effTO / time.Second)
	if timeoutSecs <= 0 {
		timeoutSecs = int(DefaultSandboxTimeout / time.Second)
	}
	labels := map[string]string{
		"devteam.sandbox":      "1",
		"devteam.task_id":      opts.TaskID,
		"devteam.timeout_secs": strconv.Itoa(timeoutSecs),
		"devteam.host_tmp":     hostTmp,
		"devteam.network_id":   networkID,
	}

	if err := st.errIfInitCancelled(); err != nil {
		return nil, err
	}

	cfg := &containertypes.Config{
		Image:      opts.Image,
		Env:        mergeSandboxEnv(opts),
		Labels:     labels,
		WorkingDir: WorkspacePath,
	}

	createResp, err := r.cli.ContainerCreate(ctx, cfg, hc, netCfg, nil, cName)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", errors.Join(ErrSandboxDocker, err))
	}
	containerID = createResp.ID
	for _, w := range createResp.Warnings {
		if w == "" {
			continue
		}
		if isDockerSwapLimitKernelUnsupportedWarning(w) {
			slog.Warn("sandbox: docker create warning (swap cgroup limits not enforced by kernel; continuing)", "warning", w)
			continue
		}
		if isDockerCreateWarningFatal(w) {
			r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "docker_create_fatal_warning")
			return nil, fmt.Errorf("docker create warning is fatal: %q: %w", w, ErrSandboxDocker)
		}
		slog.Warn("sandbox: docker create warning", "warning", w)
	}
	if err := ValidateSandboxID(containerID); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "bad_container_id")
		return nil, fmt.Errorf("unexpected container id from engine: %w", errors.Join(ErrSandboxDocker, err))
	}

	st.mu.Lock()
	st.containerID = containerID
	st.mu.Unlock()

	tarRC, err := buildPromptContextTar(opts.Instruction, opts.Context, opts.AgentSettings)
	if err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "before_copy_tar")
		return nil, err
	}
	defer tarRC.Close()
	if err := st.errIfInitCancelled(); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "before_copy")
		return nil, err
	}
	if cpErr := r.cli.CopyToContainer(ctx, containerID, WorkspacePath, tarRC, containertypes.CopyToContainerOptions{}); cpErr != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "copy_failed")
		return nil, fmt.Errorf("copy to container: %w", errors.Join(ErrSandboxDocker, cpErr))
	}

	// Sprint 16.C — Hermes-артефакты копируем отдельным CopyToContainer'ом в "/",
	// потому что home-каталог /home/sandbox/ лежит вне /workspace.
	if hermesRC, herr := buildHermesHomeTar(opts.AgentSettings); herr != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "hermes_tar_build")
		return nil, fmt.Errorf("build hermes tar: %w", herr)
	} else if hermesRC != nil {
		defer hermesRC.Close()
		if cpErr := r.cli.CopyToContainer(ctx, containerID, "/", hermesRC, containertypes.CopyToContainerOptions{}); cpErr != nil {
			r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "copy_hermes_failed")
			return nil, fmt.Errorf("copy hermes to container: %w", errors.Join(ErrSandboxDocker, cpErr))
		}
	}

	// Skills claude-семейства (claude-code/antigravity) — тоже в "/", в home-каталог
	// (~/.claude/skills или ~/.gemini/antigravity/skills по opts.Backend).
	if skillsRC, serr := buildClaudeSkillsTar(opts.AgentSettings, opts.Backend); serr != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "skills_tar_build")
		return nil, fmt.Errorf("build skills tar: %w", serr)
	} else if skillsRC != nil {
		defer skillsRC.Close()
		if cpErr := r.cli.CopyToContainer(ctx, containerID, "/", skillsRC, containertypes.CopyToContainerOptions{}); cpErr != nil {
			r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "copy_skills_failed")
			return nil, fmt.Errorf("copy skills to container: %w", errors.Join(ErrSandboxDocker, cpErr))
		}
	}

	if err := st.errIfInitCancelled(); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "before_start")
		return nil, err
	}
	if err := r.cli.ContainerStart(ctx, containerID, containertypes.StartOptions{}); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "start_failed")
		return nil, fmt.Errorf("container start: %w", errors.Join(ErrSandboxDocker, err))
	}

	if err := st.errIfInitCancelled(); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "after_start")
		return nil, err
	}
	if err := r.postStartSanity(ctx, containerID); err != nil {
		r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "sanity_failed")
		return nil, err
	}

	eff := r.effectiveTaskTimeout(opts)
	st.mu.Lock()
	st.effectiveTimeout = eff
	st.mu.Unlock()

	scheduleSandboxBusinessDeadline(st, r.stopper, containerID, eff, opts.TaskID)

	r.mu.Lock()
	delete(r.creating, cName)
	r.instances[containerID] = st
	registeredRun = true
	r.mu.Unlock()

	r.startWaitLoopIfNeeded(st)

	if r.publisher != nil && opts.ProjectID != "" && opts.TaskID != "" {
		pID, errP := uuid.Parse(opts.ProjectID)
		tID, errT := uuid.Parse(opts.TaskID)
		if errP == nil && errT == nil && pID != uuid.Nil && tID != uuid.Nil {
			// Запускаем pump только если есть куда и что публиковать
			r.setupLogPump(ctx, st, pID, tID)
		}
	}

	return &SandboxInstance{
		ID:        containerID,
		TaskID:    opts.TaskID,
		Status:    SandboxStatusRunning,
		CreatedAt: time.Now(),
	}, nil
}

func (r *DockerSandboxRunner) postStartSanity(ctx context.Context, id string) error {
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("post-start inspect: %w", err)
		}
		insp, err := r.cli.ContainerInspect(ctx, id)
		if err != nil {
			return fmt.Errorf("post-start inspect: %w", errors.Join(ErrSandboxStartup, err))
		}
		if insp.State != nil && insp.State.Running {
			return nil
		}
		if insp.State != nil && (insp.State.Status == "exited" || insp.State.Status == "dead") {
			code := insp.State.ExitCode
			return fmt.Errorf("container exited immediately (status=%s exit=%d oom=%v): %w",
				insp.State.Status, code, insp.State.OOMKilled, ErrSandboxStartup)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("post-start inspect: container not running in time: %w", ErrSandboxStartup)
}

func (r *DockerSandboxRunner) startWaitLoopIfNeeded(st *instanceState) {
	st.waitLoopOnce.Do(func() {
		go r.containerWaitLoop(st)
	})
}

func (r *DockerSandboxRunner) containerWaitLoop(st *instanceState) {
	defer st.stopBusinessTimer()
	cid := st.containerID
	waitCtx, cancelWait := context.WithCancel(context.Background())
	st.mu.Lock()
	st.cancelWait = cancelWait
	st.mu.Unlock()
	respC, errC := r.cli.ContainerWait(waitCtx, cid, containertypes.WaitConditionNotRunning)
	defer func() {
		st.mu.Lock()
		st.cancelContainerWaitLocked()
		st.mu.Unlock()
		drainDockerWait(respC, errC)
	}()

	var wr containertypes.WaitResponse
	var waitOK bool
	var waitCtxCanceled bool

	select {
	case err := <-errC:
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// Ручная отмена wait-контекста после сбоя ForceStop или Cleanup — не инфраструктурный сбой Docker:
				// идём к Inspect и сбору артефактов, exit code берём из движка.
				waitCtxCanceled = true
			} else {
				st.mu.Lock()
				st.finalWaitErr = fmt.Errorf("wait: %w", errors.Join(ErrSandboxDocker, err))
				st.mu.Unlock()
				st.closeDone()
				return
			}
		} else {
			select {
			case wr = <-respC:
				waitOK = true
			case <-time.After(5 * time.Minute):
				st.mu.Lock()
				st.finalWaitErr = fmt.Errorf("wait: missing body: %w", ErrSandboxDocker)
				st.mu.Unlock()
				st.closeDone()
				return
			}
		}
	case wr = <-respC:
		waitOK = true
	}

	st.markWaitCompleted()
	st.stopBusinessTimer()

	if waitOK && wr.Error != nil {
		st.mu.Lock()
		st.finalWaitErr = fmt.Errorf("wait engine error: %s: %w", wr.Error.Message, ErrSandboxDocker)
		st.mu.Unlock()
		st.closeDone()
		return
	}

	inspCtx, cancel := context.WithTimeout(context.Background(), dockerOpDetachTimeout)
	defer cancel()
	insp, err := r.cli.ContainerInspect(inspCtx, cid)
	if err != nil {
		st.mu.Lock()
		st.finalWaitErr = fmt.Errorf("post-wait inspect: %w", errors.Join(ErrSandboxDocker, err))
		st.mu.Unlock()
		st.closeDone()
		return
	}

	effectiveExit := 0
	if waitOK {
		effectiveExit = int(wr.StatusCode)
	}
	if insp.State != nil && insp.State.Status == "exited" {
		if waitCtxCanceled || !waitOK {
			effectiveExit = insp.State.ExitCode
		} else if effectiveExit == 0 {
			st.mu.Lock()
			if st.businessTimeoutIntent || st.userStopIntent {
				effectiveExit = insp.State.ExitCode
			}
			st.mu.Unlock()
		}
	}

	// Сбор артефактов: отдельный дедлайн от ctx вызывающего Wait. При таймауте/отмене collectCtx
	// collErr попадает в finalWaitErr — Wait вернёт ошибку (инфраструктура), а не SandboxStatus с пустым Result;
	// оркестратору так отличить «движок/сеть» от «контейнер завершился, но нет контракта status.json».
	collectCtx, cancelCollect := context.WithTimeout(context.Background(), dockerOpDetachTimeout)
	artOut, collErr := collectArtifactsForRunner(collectCtx, r.cli, cid)
	cancelCollect()

	st.mu.Lock()
	fs := r.composeFinalStatusLocked(st, &insp, effectiveExit)
	infraStrict := st.lifecycleInfraStrictLocked() || (insp.State != nil && insp.State.OOMKilled)
	if collErr != nil {
		st.finalWaitErr = collErr
	} else {
		mergeArtifactResultsIntoFinalStatus(fs, st, &insp, artOut, infraStrict)
	}
	st.finalStatus = fs
	st.mu.Unlock()
	st.closeDone()
}

// composeFinalStatusLocked — держатель st.mu. exitCode — из ContainerWait или из Inspect при отменённом wait (5.8).
func (r *DockerSandboxRunner) composeFinalStatusLocked(st *instanceState, insp *types.ContainerJSON, exitCode int) *SandboxStatus {
	out := &SandboxStatus{
		ID:       insp.ID,
		ExitCode: exitCode,
		Logs:     nil,
	}
	if insp.State != nil && insp.State.OOMKilled {
		out.Status = SandboxStatusFailed
		return out
	}
	if exitCode == 0 {
		out.Status = SandboxStatusCompleted
		return out
	}
	if st.businessTimeoutIntent {
		out.Status = SandboxStatusTimedOut
		return out
	}
	if st.userStopIntent {
		out.Status = SandboxStatusStopped
		return out
	}
	out.Status = SandboxStatusFailed
	return out
}

func (r *DockerSandboxRunner) getOrAttachState(ctx context.Context, sandboxID string) (*instanceState, error) {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return nil, err
	}
	r.mu.Lock()
	if st, ok := r.instances[sandboxID]; ok {
		r.mu.Unlock()
		return st, nil
	}
	r.mu.Unlock()

	insp, err := r.cli.ContainerInspect(ctx, sandboxID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, fmt.Errorf("inspect: %w", ErrSandboxNotFound)
		}
		return nil, fmt.Errorf("inspect: %w", errors.Join(ErrSandboxDocker, err))
	}
	if insp.Config.Labels["devteam.sandbox"] != "1" {
		return nil, fmt.Errorf("inspect: %w", ErrSandboxNotFound)
	}
	taskID := insp.Config.Labels["devteam.task_id"]
	if taskID == "" {
		return nil, fmt.Errorf("inspect: %w", ErrSandboxNotFound)
	}

	st := newInstanceState(taskID)
	st.containerID = insp.ID
	st.containerName = strings.TrimPrefix(insp.Name, "/")
	st.hostTempDir = insp.Config.Labels["devteam.host_tmp"]
	st.networkID = insp.Config.Labels["devteam.network_id"]
	st.stopGracePeriod = DefaultSandboxStopGrace
	if secs, perr := strconv.ParseInt(insp.Config.Labels["devteam.timeout_secs"], 10, 64); perr == nil && secs > 0 {
		st.effectiveTimeout = time.Duration(secs) * time.Second
	} else {
		st.effectiveTimeout = r.fallbackTaskTimeoutFromLabels()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.instances[sandboxID]; ok {
		return existing, nil
	}
	r.instances[sandboxID] = st
	return st, nil
}

func (r *DockerSandboxRunner) snapshotStatusFromInspect(insp *types.ContainerJSON) *SandboxStatus {
	out := &SandboxStatus{ID: insp.ID}
	if insp.State == nil {
		out.Status = SandboxStatusFailed
		return out
	}
	switch insp.State.Status {
	case "running", "restarting", "paused", "dead":
		if insp.State.Status == "running" {
			out.Status = SandboxStatusRunning
		} else {
			out.Status = SandboxStatusRunning
		}
	case "created":
		out.Status = SandboxStatusCreating
	case "removing":
		out.Status = SandboxStatusStopped
	case "exited":
		out.ExitCode = insp.State.ExitCode
		if insp.State.OOMKilled {
			out.Status = SandboxStatusFailed
		} else if insp.State.ExitCode == 0 {
			out.Status = SandboxStatusCompleted
		} else {
			out.Status = SandboxStatusFailed
		}
	default:
		out.Status = SandboxStatusFailed
	}
	if insp.State.StartedAt != "" && insp.State.StartedAt != "0001-01-01T00:00:00Z" {
		if started, err := time.Parse(time.RFC3339Nano, insp.State.StartedAt); err == nil && insp.State.Running {
			out.RunningFor = time.Since(started)
		}
	}
	return out
}

// Wait реализует SandboxRunner.Wait.
func (r *DockerSandboxRunner) Wait(ctx context.Context, sandboxID string) (*SandboxStatus, error) {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return nil, err
	}
	st, err := r.getOrAttachState(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	r.startWaitLoopIfNeeded(st)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-st.doneCh:
		st.mu.Lock()
		fs := st.finalStatus
		fe := st.finalWaitErr
		st.mu.Unlock()
		if fe != nil {
			return nil, fe
		}
		if fs == nil {
			return nil, fmt.Errorf("wait: empty final status: %w", ErrSandboxDocker)
		}
		return fs, nil
	}
}

// GetStatus реализует SandboxRunner.GetStatus.
func (r *DockerSandboxRunner) GetStatus(ctx context.Context, sandboxID string) (*SandboxStatus, error) {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return nil, err
	}
	st, err := r.getOrAttachState(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	insp, err := r.cli.ContainerInspect(ctx, sandboxID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, fmt.Errorf("inspect: %w", ErrSandboxNotFound)
		}
		return nil, fmt.Errorf("inspect: %w", errors.Join(ErrSandboxDocker, err))
	}
	out := r.snapshotStatusFromInspect(&insp)
	if insp.State != nil && insp.State.Status == "exited" {
		st.mu.Lock()
		fs := st.finalStatus
		st.mu.Unlock()
		if fs != nil {
			if fs.HasResult() {
				out.Result = fs.Result
			}
			// Согласование с Wait после 5.7 (контракт status.json, таймаут, стоп).
			switch fs.Status {
			case SandboxStatusCompleted, SandboxStatusFailed, SandboxStatusTimedOut, SandboxStatusStopped:
				out.Status = fs.Status
			}
		}
	}
	if insp.State != nil && insp.State.Status == "exited" && insp.State.ExitCode == 0 && !insp.State.OOMKilled {
		// Успешный exit 0 первичен над гонкой намерений таймера/стопа (5.8).
		return out, nil
	}
	st.mu.Lock()
	timed := st.businessTimeoutIntent
	user := st.userStopIntent
	st.mu.Unlock()
	if timed {
		out.Status = SandboxStatusTimedOut
	} else if user {
		out.Status = SandboxStatusStopped
	}
	return out, nil
}

// Stop — graceful stop через ContainerStopper + отмена бизнес-таймера и ContainerWait (5.8).
func (r *DockerSandboxRunner) Stop(ctx context.Context, sandboxID string) error {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return err
	}
	st, err := r.getOrAttachState(ctx, sandboxID)
	if err != nil {
		if errors.Is(err, ErrSandboxNotFound) {
			return nil
		}
		return err
	}
	cid, grace, already := st.applyUserStopIntent()
	if already {
		return nil
	}
	if cid == "" {
		return nil
	}
	sErr := r.stopper.ForceStop(ctx, cid, grace, "user_stop", st.taskID)
	if sErr != nil {
		st.mu.Lock()
		st.cancelContainerWaitLocked()
		st.mu.Unlock()
	}
	return sErr
}

// StopTask отменяет RunTask до появления containerID (фаза creating) или делегирует Stop по ID для уже запущенной задачи (5.8).
func (r *DockerSandboxRunner) StopTask(ctx context.Context, taskID string) error {
	if err := ValidateTaskID(taskID); err != nil {
		return err
	}
	if r.cli == nil {
		return fmt.Errorf("docker client is nil: %w", ErrSandboxDocker)
	}
	r.mu.Lock()
	foundCreating := false
	for _, st := range r.creating {
		if st.taskID == taskID {
			st.mu.Lock()
			st.initCancelRequested = true
			st.mu.Unlock()
			foundCreating = true
		}
	}
	if foundCreating {
		r.mu.Unlock()
		return nil
	}
	var found *instanceState
	for _, s := range r.instances {
		if s.taskID == taskID {
			found = s
			break
		}
	}
	r.mu.Unlock()
	if found != nil {
		return r.Stop(ctx, found.containerID)
	}
	cname := taskContainerName(taskID)
	insp, ierr := r.cli.ContainerInspect(ctx, cname)
	if ierr != nil {
		if errdefs.IsNotFound(ierr) {
			return nil
		}
		return fmt.Errorf("inspect: %w", errors.Join(ErrSandboxDocker, ierr))
	}
	if insp.Config.Labels["devteam.sandbox"] != "1" {
		return nil
	}
	if err := ValidateSandboxID(insp.ID); err != nil {
		return nil
	}
	return r.Stop(ctx, insp.ID)
}

// Cleanup — идемпотентная уборка контейнера, сети и хостового temp (см. 5.3 про ctx).
func (r *DockerSandboxRunner) Cleanup(ctx context.Context, sandboxID string) error {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return err
	}
	rmCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()

	var netID, hostTmp string
	var serviceIDs []string
	r.mu.Lock()
	st := r.instances[sandboxID]
	if st != nil {
		netID = st.networkID
		hostTmp = st.hostTempDir
		serviceIDs = append([]string(nil), st.serviceContainerIDs...)
		delete(r.instances, sandboxID)
	}
	r.mu.Unlock()

	if st != nil {
		st.setCleaned()
		st.stopBusinessTimer()
		st.streamMu.Lock()
		if st.streamCancel != nil {
			st.streamCancel()
			st.streamCancel = nil
		}
		// streamActive сбрасывает только горутина стрима после закрытия канала (5.6, без гонки со вторым StreamLogs).
		st.streamMu.Unlock()
	} else {
		if insp, ierr := r.cli.ContainerInspect(rmCtx, sandboxID); ierr == nil {
			netID = insp.Config.Labels["devteam.network_id"]
			hostTmp = insp.Config.Labels["devteam.host_tmp"]
		}
	}

	if err := r.cli.ContainerRemove(rmCtx, sandboxID, containertypes.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
		if errdefs.IsNotFound(err) {
			slog.Debug("sandbox: container remove not found (already gone)",
				"sandbox_id", sandboxID, "op", "container_remove", "err", err)
		} else {
			return fmt.Errorf("container remove: %w", errors.Join(ErrSandboxDocker, err))
		}
	}
	// Сервис-сайдкары (Sprint 22) сносим после агент-контейнера, до сети. При живом state —
	// по запомненным ID; иначе (рестарт процесса) — по labels через network_id.
	if st != nil {
		for _, sid := range serviceIDs {
			r.removeContainerForceLogged(rmCtx, st.taskID, sid, "cleanup_service")
		}
	} else {
		r.removeServiceContainersByNetwork(rmCtx, "", netID)
	}
	r.removeNetworkBestEffort(rmCtx, netID)
	if hostTmp != "" {
		_ = os.RemoveAll(hostTmp)
	}
	return nil
}

func (r *DockerSandboxRunner) setupLogPump(rootCtx context.Context, st *instanceState, projectID, taskID uuid.UUID) {
	st.streamMu.Lock()
	if st.streamActive {
		st.streamMu.Unlock()
		return
	}

	pumpCtx, cancel := context.WithCancel(rootCtx)
	st.streamCancel = cancel
	st.streamActive = true

	bufCap := StreamLogsDefaultBuffer
	if r.streamLogsEntryBuffer > 0 {
		bufCap = r.streamLogsEntryBuffer
	}

	// Создаем мастер-канал
	masterCh := make(chan LogEntry, bufCap)
	st.streamCh = masterCh
	st.streamMu.Unlock()

	// Создаем tee на 2 канала: один для пампа, один (потенциально) для внешнего StreamLogs
	tees := tee(masterCh, 2)
	pumpLogCh := tees[0]
	externalLogCh := tees[1]

	// stopCh для сигнализации Cleanup
	stopCh := make(chan struct{})
	st.mu.Lock()
	oldCleanup := st.onCleanup
	st.onCleanup = func() {
		close(stopCh)
		if oldCleanup != nil {
			oldCleanup()
		}
	}
	st.mu.Unlock()

	// Запускаем сам Docker-стрим
	go r.runLogStream(pumpCtx, cancel, st, st.containerID, masterCh)

	// Запускаем памп в шину
	go r.streamLogsToBus(pumpCtx, stopCh, projectID, taskID, st.containerID, pumpLogCh)

	// Сохраняем второе плечо для возможного вызова StreamLogs
	st.streamMu.Lock()
	st.externalCh = externalLogCh
	st.streamMu.Unlock()
}

var _ SandboxRunner = (*DockerSandboxRunner)(nil)
