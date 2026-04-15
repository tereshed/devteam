//go:build integration

package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestDockerSandboxRunner_DisableNetworkCloneFails — реальный Engine + DockerSandboxRunner:
// без сети git clone не выполняется; entrypoint пишет status.json (trap EXIT); Wait → failed, затем Cleanup.
//
// sequential: один Docker daemon — без t.Parallel.
func TestDockerSandboxRunner_DisableNetworkCloneFails(t *testing.T) {
	cli := newIntegrationDockerClient(t)
	img := integrationSandboxImageRef(t)
	imgCtx, imgCancel := context.WithTimeout(context.Background(), 20*time.Second)
	integrationSkipIfSandboxImageMissing(t, imgCtx, cli, img)
	imgCancel()

	runner := NewDockerSandboxRunner(cli, nil, WithDefaultTaskTimeout(5*time.Minute))

	taskID := uuid.New().String()
	opts := SandboxOptions{
		TaskID:      taskID,
		Backend:     CodeBackendClaudeCode,
		Image:       img,
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "master",
		Instruction: "integration: expect clone failure without network",
		Context:     "",
		EnvVars: map[string]string{
			// Плейсхолдер проходит валидацию entrypoint до clone (как в entrypoint_integration_test.go).
			EnvAnthropicAPIKey: "sk-ant-api03-ci-placeholder-not-used",
		},
		DisableNetwork: true,
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer runCancel()

	inst, err := runner.RunTask(runCtx, opts)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.Equal(t, SandboxStatusRunning, inst.Status)
	sandboxID := inst.ID

	cleanupTimeout := 90 * time.Second
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer ccancel()
		_ = runner.Cleanup(cctx, sandboxID)
	})

	getCtx, getCancel := context.WithTimeout(context.Background(), 45*time.Second)
	snap, err := runner.GetStatus(getCtx, sandboxID)
	getCancel()
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Contains(t, []SandboxStatusType{SandboxStatusRunning, SandboxStatusFailed}, snap.Status)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer waitCancel()
	final, werr := runner.Wait(waitCtx, sandboxID)
	require.NoError(t, werr)
	require.NotNil(t, final)
	require.Equal(t, SandboxStatusFailed, final.Status)
	require.NotZero(t, final.ExitCode)
	require.True(t, final.HasResult())
	require.False(t, final.Result.Success)

	rmCtx, rmCancel := context.WithTimeout(context.Background(), cleanupTimeout)
	require.NoError(t, runner.Cleanup(rmCtx, sandboxID))
	rmCancel()

	rmCtx2, rmCancel2 := context.WithTimeout(context.Background(), cleanupTimeout)
	require.NoError(t, runner.Cleanup(rmCtx2, sandboxID))
	rmCancel2()

	inspCtx, inspCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer inspCancel()
	_, ierr := cli.ContainerInspect(inspCtx, sandboxID)
	require.True(t, errdefs.IsNotFound(ierr), "контейнер должен быть удалён: %v", ierr)
}

// TestDockerSandboxRunner_InvalidBranchNoContainer связывает ValidateBranchName с реальным раннером:
// ошибка до ContainerCreate; таблица инъекций для Branch/RepoURL — в options_validate_test.go / docker_runner_test.go (5.13).
func TestDockerSandboxRunner_InvalidBranchNoContainer(t *testing.T) {
	cli := newIntegrationDockerClient(t)
	img := integrationSandboxImageRef(t)
	imgCtx, imgCancel := context.WithTimeout(context.Background(), 20*time.Second)
	integrationSkipIfSandboxImageMissing(t, imgCtx, cli, img)
	imgCancel()

	runner := NewDockerSandboxRunner(cli, nil, WithDefaultTaskTimeout(2*time.Minute))
	taskID := uuid.New().String()
	opts := SandboxOptions{
		TaskID:      taskID,
		Backend:     CodeBackendClaudeCode,
		Image:       img,
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "-bad",
		Instruction: "should not run",
		EnvVars: map[string]string{
			EnvAnthropicAPIKey: "sk-ant-api03-ci-placeholder-not-used",
		},
		DisableNetwork: true,
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer runCancel()
	_, err := runner.RunTask(runCtx, opts)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidOptions), "got: %v", err)
	require.True(t, errors.Is(err, ErrInvalidBranchName), "got: %v", err)

	inspCtx, inspCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer inspCancel()
	_, ierr := cli.ContainerInspect(inspCtx, taskContainerName(taskID))
	require.True(t, errdefs.IsNotFound(ierr), "контейнер не должен создаваться: %v", ierr)
}
