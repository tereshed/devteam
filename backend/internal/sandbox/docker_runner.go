package sandbox

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// Политика образов (5.5): при отсутствии локально выполняем ImagePull и обязательно дочитываем тело ответа.
// Детальная настройка и вариант «только предзагрузка» — 5.10 / README.

const dockerOpDetachTimeout = 45 * time.Second

// DockerSandboxRunner — реализация SandboxRunner через Docker Engine API (задача 5.5).
type DockerSandboxRunner struct {
	cli     *client.Client
	stopper *dockerStopper
	allowed []string

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
	}
}

// RunnerOption — опциональная настройка DockerSandboxRunner (расширяем без ломки существующих вызовов конструктора).
type RunnerOption func(*DockerSandboxRunner)

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
		cli:         cli,
		stopper:     newDockerStopper(cli),
		allowed:     allowed,
		limitPolicy: normalizeResourceLimitPolicy(ResourceLimitPolicy{}),
		instances:   make(map[string]*instanceState),
		creating:    make(map[string]*instanceState),
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

func sandboxBridgeNetworkName(taskID string) string {
	sum := sha256.Sum256([]byte(taskID))
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
	// Sprint 15.22: permission-mode для claude code CLI; пробрасываем только если задан в AgentSettings.
	if opts.AgentSettings != nil && opts.AgentSettings.PermissionMode != "" {
		out = append(out, EnvClaudeCodePermissionMode+"="+opts.AgentSettings.PermissionMode)
	}
	return out
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
	if r.cli == nil {
		return nil, fmt.Errorf("docker client is nil: %w", ErrSandboxDocker)
	}

	cName := taskContainerName(opts.TaskID)
	if _, err := r.cli.ContainerInspect(ctx, cName); err == nil {
		return nil, ErrSandboxRunConflict
	} else if !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("inspect container name: %w", errors.Join(ErrSandboxDocker, err))
	}

	st := newInstanceState(opts.TaskID)
	st.containerName = cName
	st.stopGracePeriod = opts.EffectiveStopGrace()

	r.mu.Lock()
	if _, dup := r.creating[opts.TaskID]; dup {
		r.mu.Unlock()
		return nil, ErrSandboxRunConflict
	}
	r.creating[opts.TaskID] = st
	r.mu.Unlock()

	var (
		containerID   string
		networkID     string
		hostTmp       string
		registeredRun = false
	)
	defer func() {
		r.mu.Lock()
		delete(r.creating, opts.TaskID)
		r.mu.Unlock()
		if !registeredRun && containerID != "" {
			r.removeContainerForceLogged(ctx, opts.TaskID, containerID, "run_task_defer")
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
		netName = sandboxBridgeNetworkName(opts.TaskID)
		netResp, nerr := r.cli.NetworkCreate(ctx, netName, network.CreateOptions{
			Driver: "bridge",
			Options: map[string]string{
				"com.docker.network.bridge.enable_icc": "false",
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
	delete(r.creating, opts.TaskID)
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
	if st, ok := r.creating[taskID]; ok {
		st.mu.Lock()
		st.initCancelRequested = true
		st.mu.Unlock()
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
	r.mu.Lock()
	st := r.instances[sandboxID]
	if st != nil {
		netID = st.networkID
		hostTmp = st.hostTempDir
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
