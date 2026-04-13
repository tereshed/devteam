package sandbox

import (
	"archive/tar"
	"bytes"
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

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
)

// Политика образов (5.5): при отсутствии локально выполняем ImagePull и обязательно дочитываем тело ответа.
// Детальная настройка и вариант «только предзагрузка» — 5.10 / README.

const (
	sandboxMemoryFloorBytes int64 = 1 << 30 // 1 GiB
	sandboxPidsFloor        int64 = 100
	sandboxMemoryCeilBytes  int64 = 16 << 30
	sandboxPidsCeil         int64 = 8192
	sandboxNanoCPUsCeil     int64 = 16_000_000_000
	dockerOpDetachTimeout         = 45 * time.Second
	dockerStopGraceSeconds        = 10
)

// DockerSandboxRunner — реализация SandboxRunner через Docker Engine API (задача 5.5).
type DockerSandboxRunner struct {
	cli     *client.Client
	allowed []string

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

// NewDockerSandboxRunner создаёт раннер. cli не должен быть nil; allowedImages пустой — дефолты.
func NewDockerSandboxRunner(cli *client.Client, allowedImages []string) *DockerSandboxRunner {
	allowed := append([]string(nil), allowedImages...)
	if len(allowed) == 0 {
		allowed = DefaultAllowedSandboxImages()
	}
	return &DockerSandboxRunner{
		cli:       cli,
		allowed:   allowed,
		instances: make(map[string]*instanceState),
		creating:  make(map[string]*instanceState),
	}
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
	return out
}

func effectiveMemoryBytes(rl ResourceLimit) int64 {
	mb := int64(rl.MemoryMB)
	if mb <= 0 || mb*1024*1024 < sandboxMemoryFloorBytes {
		return sandboxMemoryFloorBytes
	}
	b := mb * 1024 * 1024
	if b > sandboxMemoryCeilBytes {
		return sandboxMemoryCeilBytes
	}
	return b
}

func effectivePidsLimit(rl ResourceLimit) int64 {
	p := int64(rl.PIDsLimit)
	if p < sandboxPidsFloor {
		p = sandboxPidsFloor
	}
	if p > sandboxPidsCeil {
		p = sandboxPidsCeil
	}
	return p
}

func effectiveNanoCPUs(rl ResourceLimit) int64 {
	if rl.NanoCPUs <= 0 {
		return 0
	}
	if rl.NanoCPUs > sandboxNanoCPUsCeil {
		return sandboxNanoCPUsCeil
	}
	return rl.NanoCPUs
}

func drainDockerWait(respC <-chan containertypes.WaitResponse, errC <-chan error) {
	go func() { <-respC }()
	go func() { <-errC }()
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

func (r *DockerSandboxRunner) removeContainerForce(ctx context.Context, id string) {
	rmCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()
	_ = r.cli.ContainerRemove(rmCtx, id, containertypes.RemoveOptions{Force: true, RemoveVolumes: true})
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

// buildPromptContextTar упаковывает prompt.txt и context.txt в tar для CopyToContainer (без лишнего I/O на диске хоста).
func buildPromptContextTar(instruction, contextText string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		var err error
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.CloseWithError(err)
		}()
		now := time.Now()
		for _, f := range []struct{ name, content string }{
			{"prompt.txt", instruction},
			{"context.txt", contextText},
		} {
			hdr := &tar.Header{
				Typeflag: tar.TypeReg,
				Name:     f.name,
				Mode:     0o600,
				Size:     int64(len(f.content)),
				ModTime:  now,
			}
			if err = tw.WriteHeader(hdr); err != nil {
				return
			}
			if _, err = io.Copy(tw, strings.NewReader(f.content)); err != nil {
				return
			}
		}
	}()
	return pr, nil
}

// RunTask реализует SandboxRunner.RunTask.
func (r *DockerSandboxRunner) RunTask(ctx context.Context, opts SandboxOptions) (*SandboxInstance, error) {
	opts = opts.Clone()
	if err := opts.Validate(ctx); err != nil {
		return nil, err
	}
	// Явная защита от инъекций через env/ветку (Validate уже вызывает ValidateRepoURL с DNS — не дублируем).
	if err := ValidateEnvKeys(opts.EnvVars); err != nil {
		return nil, errors.Join(ErrInvalidOptions, err)
	}
	if err := ValidateBranchName(opts.Branch); err != nil {
		return nil, errors.Join(ErrInvalidOptions, err)
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
			r.removeContainerForce(ctx, containerID)
		}
		if !registeredRun && networkID != "" {
			r.removeNetworkBestEffort(ctx, networkID)
		}
		if !registeredRun && hostTmp != "" {
			_ = os.RemoveAll(hostTmp)
		}
	}()

	// prompt/context идут в контейнер только через tar в памяти (без хостового каталога).
	hostTmp = ""

	if err := r.ensureLocalImage(ctx, opts.Image); err != nil {
		return nil, err
	}

	var (
		netName  string
		netCfg   *network.NetworkingConfig
		hostNet  containertypes.NetworkMode
		initTrue = true
		memBytes = effectiveMemoryBytes(opts.ResourceLimit)
		pidsLim  = effectivePidsLimit(opts.ResourceLimit)
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
				PidsLimit:  &pidsLim,
			},
			ReadonlyRootfs: false,
		}
	)
	if nc := effectiveNanoCPUs(opts.ResourceLimit); nc > 0 {
		hc.Resources.NanoCPUs = nc
	}

	if opts.DisableNetwork {
		hc.NetworkMode = network.NetworkNone
		netCfg = &network.NetworkingConfig{}
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

	timeoutSecs := int(opts.EffectiveTimeout() / time.Second)
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
	for _, w := range createResp.Warnings {
		if w != "" {
			slog.Warn("sandbox: docker create warning", "warning", w)
		}
	}
	containerID = createResp.ID
	if err := ValidateSandboxID(containerID); err != nil {
		r.removeContainerForce(ctx, containerID)
		return nil, fmt.Errorf("unexpected container id from engine: %w", errors.Join(ErrSandboxDocker, err))
	}

	st.mu.Lock()
	st.containerID = containerID
	st.mu.Unlock()

	tarRC, err := buildPromptContextTar(opts.Instruction, opts.Context)
	if err != nil {
		r.removeContainerForce(ctx, containerID)
		return nil, err
	}
	defer tarRC.Close()
	if cpErr := r.cli.CopyToContainer(ctx, containerID, WorkspacePath, tarRC, containertypes.CopyToContainerOptions{}); cpErr != nil {
		r.removeContainerForce(ctx, containerID)
		return nil, fmt.Errorf("copy to container: %w", errors.Join(ErrSandboxDocker, cpErr))
	}

	if err := r.cli.ContainerStart(ctx, containerID, containertypes.StartOptions{}); err != nil {
		r.removeContainerForce(ctx, containerID)
		return nil, fmt.Errorf("container start: %w", errors.Join(ErrSandboxDocker, err))
	}

	if err := r.postStartSanity(ctx, containerID); err != nil {
		r.removeContainerForce(ctx, containerID)
		return nil, err
	}

	eff := opts.EffectiveTimeout()
	st.mu.Lock()
	st.effectiveTimeout = eff
	st.mu.Unlock()

	st.businessTimer = time.AfterFunc(eff, func() {
		if st.cleaned.Load() {
			return
		}
		st.timedOut.Store(1)
		st.stoppedByRunner.Store(1)
		killCtx, cancel := detachTimeout(context.Background(), dockerOpDetachTimeout)
		defer cancel()
		_ = r.cli.ContainerStop(killCtx, containerID, containertypes.StopOptions{Timeout: ptrInt(0)})
		_ = r.cli.ContainerKill(killCtx, containerID, "SIGKILL")
	})

	r.mu.Lock()
	delete(r.creating, opts.TaskID)
	r.instances[containerID] = st
	registeredRun = true
	r.mu.Unlock()

	r.startWaitLoopIfNeeded(st)

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
	respC, errC := r.cli.ContainerWait(waitCtx, cid, containertypes.WaitConditionNotRunning)
	defer func() {
		cancelWait()
		drainDockerWait(respC, errC)
	}()

	var wr containertypes.WaitResponse
	select {
	case err := <-errC:
		if err != nil {
			st.mu.Lock()
			st.finalWaitErr = fmt.Errorf("wait: %w", errors.Join(ErrSandboxDocker, err))
			st.mu.Unlock()
			st.closeDone()
			return
		}
		select {
		case wr = <-respC:
		case <-time.After(5 * time.Minute):
			st.mu.Lock()
			st.finalWaitErr = fmt.Errorf("wait: missing body: %w", ErrSandboxDocker)
			st.mu.Unlock()
			st.closeDone()
			return
		}
	case wr = <-respC:
	}

	if wr.Error != nil {
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

	st.mu.Lock()
	st.finalStatus = r.composeFinalStatus(st, &insp, int(wr.StatusCode))
	st.mu.Unlock()
	st.closeDone()
}

func (r *DockerSandboxRunner) composeFinalStatus(st *instanceState, insp *types.ContainerJSON, exitCode int) *SandboxStatus {
	out := &SandboxStatus{
		ID:       insp.ID,
		ExitCode: exitCode,
		Logs:     nil,
	}
	if insp.State != nil && insp.State.OOMKilled {
		out.Status = SandboxStatusFailed
		return out
	}
	if st.timedOut.Load() == 1 {
		out.Status = SandboxStatusTimedOut
		return out
	}
	if st.stoppedByRunner.Load() == 1 && st.timedOut.Load() == 0 {
		out.Status = SandboxStatusStopped
		return out
	}
	if exitCode == 0 {
		out.Status = SandboxStatusCompleted
	} else {
		out.Status = SandboxStatusFailed
	}
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
	if secs, perr := strconv.ParseInt(insp.Config.Labels["devteam.timeout_secs"], 10, 64); perr == nil && secs > 0 {
		st.effectiveTimeout = time.Duration(secs) * time.Second
	} else {
		st.effectiveTimeout = DefaultSandboxTimeout
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
	if st.timedOut.Load() == 1 {
		out.Status = SandboxStatusTimedOut
	}
	return out, nil
}

// streamLineWriter пишет строки в канал LogEntry (минимальный стрим до задачи 5.6).
type streamLineWriter struct {
	ch        chan<- LogEntry
	sandboxID string
	stderr    bool
	buf       []byte
}

func (w *streamLineWriter) emitFragment(line []byte) {
	if len(line) == 0 {
		return
	}
	for len(line) > 0 {
		n := len(line)
		if n > LogEntryMaxLineBytes {
			n = LogEntryMaxLineBytes
		}
		chunk := string(line[:n])
		line = line[n:]
		select {
		case w.ch <- LogEntry{SandboxID: w.sandboxID, Timestamp: time.Now(), Line: chunk, Stderr: w.stderr}:
		default:
			return
		}
	}
}

func (w *streamLineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emitFragment(w.buf[:i])
		n := copy(w.buf, w.buf[i+1:])
		w.buf = w.buf[:n]
	}
	if len(w.buf) > LogEntryMaxLineBytes*8 {
		w.emitFragment(w.buf)
		w.buf = w.buf[:0]
	}
	return len(p), nil
}

func (w *streamLineWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	w.emitFragment(w.buf)
	w.buf = nil
}

// StreamLogs — минимальный follow-стрим (stdout/stderr); детали буфера — 5.6.
func (r *DockerSandboxRunner) StreamLogs(ctx context.Context, sandboxID string) (<-chan LogEntry, error) {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return nil, err
	}
	st, err := r.getOrAttachState(ctx, sandboxID)
	if err != nil {
		return nil, err
	}

	ch := make(chan LogEntry, StreamLogsDefaultBuffer)
	st.streamMu.Lock()
	if st.streamActive {
		st.streamMu.Unlock()
		return nil, ErrStreamAlreadyActive
	}
	streamCtx, cancel := context.WithCancel(ctx)
	st.streamCancel = cancel
	st.streamActive = true
	st.streamMu.Unlock()

	go func() {
		defer close(ch)
		defer func() {
			st.streamMu.Lock()
			st.streamActive = false
			st.streamCancel = nil
			st.streamMu.Unlock()
			cancel()
		}()

		rc, err := r.cli.ContainerLogs(streamCtx, sandboxID, containertypes.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		})
		if err != nil {
			select {
			case ch <- LogEntry{SandboxID: sandboxID, Timestamp: time.Now(), Error: fmt.Errorf("container logs: %w", errors.Join(ErrSandboxDocker, err))}:
			default:
			}
			return
		}
		defer rc.Close()

		outW := &streamLineWriter{ch: ch, sandboxID: sandboxID, stderr: false}
		errW := &streamLineWriter{ch: ch, sandboxID: sandboxID, stderr: true}
		_, cpyErr := stdcopy.StdCopy(outW, errW, rc)
		outW.flush()
		errW.flush()
		if cpyErr != nil && !errors.Is(cpyErr, context.Canceled) {
			select {
			case ch <- LogEntry{SandboxID: sandboxID, Timestamp: time.Now(), Error: fmt.Errorf("log stream: %w", errors.Join(ErrSandboxDocker, cpyErr))}:
			default:
			}
		}
	}()
	return ch, nil
}

// Stop — SIGTERM через ContainerStop с таймаутом, затем SIGKILL; при отмене ctx — best-effort kill (5.8 уточнит).
func (r *DockerSandboxRunner) Stop(ctx context.Context, sandboxID string) error {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return err
	}
	st, _ := r.getOrAttachState(ctx, sandboxID)
	if st != nil {
		st.stoppedByRunner.Store(1)
	}
	err := r.cli.ContainerStop(ctx, sandboxID, containertypes.StopOptions{Timeout: ptrInt(dockerStopGraceSeconds)})
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		killCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
		defer cancel()
		_ = r.cli.ContainerKill(killCtx, sandboxID, "SIGKILL")
		return err
	}
	killCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()
	if kerr := r.cli.ContainerKill(killCtx, sandboxID, "SIGKILL"); kerr != nil && !errdefs.IsNotFound(kerr) {
		return fmt.Errorf("stop/kill: %w", errors.Join(ErrSandboxDocker, errors.Join(err, kerr)))
	}
	return nil
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
		st.cleaned.Store(true)
		st.stopBusinessTimer()
		st.streamMu.Lock()
		if st.streamCancel != nil {
			st.streamCancel()
			st.streamCancel = nil
		}
		st.streamActive = false
		st.streamMu.Unlock()
	} else {
		if insp, ierr := r.cli.ContainerInspect(rmCtx, sandboxID); ierr == nil {
			netID = insp.Config.Labels["devteam.network_id"]
			hostTmp = insp.Config.Labels["devteam.host_tmp"]
		}
	}

	if err := r.cli.ContainerRemove(rmCtx, sandboxID, containertypes.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("container remove: %w", errors.Join(ErrSandboxDocker, err))
	}
	r.removeNetworkBestEffort(rmCtx, netID)
	if hostTmp != "" {
		_ = os.RemoveAll(hostTmp)
	}
	return nil
}

var _ SandboxRunner = (*DockerSandboxRunner)(nil)
