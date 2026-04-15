//go:build integration

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/require"
)

const integrationDefaultSandboxImage = "devteam/sandbox-claude:local"

// integrationSandboxImageRef — SANDBOX_CLAUDE_IMAGE или devteam/sandbox-claude:local (согласовано с entrypoint-тестами и make sandbox-build).
func integrationSandboxImageRef(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("SANDBOX_CLAUDE_IMAGE"); v != "" {
		return v
	}
	return integrationDefaultSandboxImage
}

func integrationDockerCLIAvailable(t *testing.T) bool {
	t.Helper()
	_, err := exec.LookPath("docker")
	return err == nil
}

func integrationDockerImageExistsCLI(t *testing.T, image string) bool {
	t.Helper()
	cmd := exec.Command("docker", "image", "inspect", "--", image)
	return cmd.Run() == nil
}

// newIntegrationDockerClient подключается к Docker Engine через SDK (как прод).
// При недоступном демоне — t.Skip, не fail.
func newIntegrationDockerClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.Ping(pingCtx); err != nil {
		t.Skipf("Docker Engine API недоступен (ping): %v", err)
	}
	return cli
}

// integrationSkipIfSandboxImageMissing пропускает тест, если локально нет образа (ImageInspect).
func integrationSkipIfSandboxImageMissing(t *testing.T, ctx context.Context, cli *client.Client, ref string) {
	t.Helper()
	_, err := cli.ImageInspect(ctx, ref)
	if err == nil {
		return
	}
	if errdefs.IsNotFound(err) {
		t.Skipf("образ %q отсутствует; соберите: make sandbox-build или docker build -t %s -f deployment/sandbox/claude/Dockerfile deployment/sandbox/claude", ref, ref)
	}
	require.NoError(t, err)
}
