package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// ContainerStopper — узкий контракт остановки контейнера без *client.Client в lifecycle (5.8, тестируемость).
type ContainerStopper interface {
	// ForceStop: SIGTERM-цепочка через ContainerStop (graceSeconds>0), при graceSeconds<=0 — немедленный SIGKILL-путь.
	// reason — enum-строка для логов: "timeout", "user_stop", "init_cancel" (без opts/секретов).
	ForceStop(ctx context.Context, containerID string, graceSeconds int, reason string, taskID string) error
}

// dockerStopper — единая DRY-реализация Stop→Kill для таймера, Stop и откатов (5.8).
type dockerStopper struct {
	cli *client.Client
}

func newDockerStopper(cli *client.Client) *dockerStopper {
	return &dockerStopper{cli: cli}
}

// ForceStop реализует ContainerStopper.
func (d *dockerStopper) ForceStop(ctx context.Context, containerID string, graceSeconds int, reason string, taskID string) error {
	if d == nil || d.cli == nil {
		return fmt.Errorf("docker stopper: nil client: %w", ErrSandboxDocker)
	}
	if containerID == "" {
		return fmt.Errorf("docker stopper: empty container id: %w", ErrSandboxDocker)
	}

	if graceSeconds <= 0 {
		return d.forceKill(ctx, containerID, reason, taskID)
	}

	stopCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()
	err := d.cli.ContainerStop(stopCtx, containerID, containertypes.StopOptions{Timeout: ptrInt(graceSeconds)})
	if err == nil {
		return nil
	}
	if errdefs.IsNotFound(err) {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if errors.Is(err, context.DeadlineExceeded) && stopCtx.Err() != nil {
			slog.Error("sandbox: docker stop detach deadline (engine hung?)",
				"task_id", taskID, "sandbox_id", containerID, "reason", reason, "err", err)
		}
		killErr := d.forceKill(context.Background(), containerID, reason, taskID)
		return errors.Join(err, killErr)
	}

	killCtx, killCancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer killCancel()
	if kerr := d.cli.ContainerKill(killCtx, containerID, "SIGKILL"); kerr != nil && !errdefs.IsNotFound(kerr) {
		if errors.Is(kerr, context.DeadlineExceeded) {
			slog.Error("sandbox: docker kill detach deadline after stop error",
				"task_id", taskID, "sandbox_id", containerID, "reason", reason, "stop_err", err, "kill_err", kerr)
		}
		return fmt.Errorf("stop then kill: %w", errors.Join(ErrSandboxDocker, errors.Join(err, kerr)))
	}
	return nil
}

func (d *dockerStopper) forceKill(ctx context.Context, containerID, reason, taskID string) error {
	killCtx, cancel := detachTimeout(ctx, dockerOpDetachTimeout)
	defer cancel()
	err := d.cli.ContainerKill(killCtx, containerID, "SIGKILL")
	if err == nil || errdefs.IsNotFound(err) {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Error("sandbox: docker kill detach deadline",
			"task_id", taskID, "sandbox_id", containerID, "reason", reason, "err", err)
	}
	return fmt.Errorf("kill: %w", errors.Join(ErrSandboxDocker, err))
}

// stopGraceSecondsFromDuration переводит EffectiveStopGrace в целые секунды для Docker Stop (минимум 1 при grace>0).
func stopGraceSecondsFromDuration(grace time.Duration) int {
	if grace <= 0 {
		return 0
	}
	s := int(math.Ceil(grace.Seconds()))
	if s < 1 {
		return 1
	}
	return s
}
