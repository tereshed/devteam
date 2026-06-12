//go:build integration

package sandbox

import (
	"archive/tar"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 22 — живой Docker-тест доставки skills claude-семейства:
// buildClaudeSkillsTar → CopyToContainer("/") → файлы лежат в home-каталоге
// контейнера с владельцем sandbox (uid 1001) и корректным содержимым.
//
// Контейнер НЕ стартует (entrypoint не выполняется): проверяем ровно тот шаг,
// который делает RunTask до ContainerStart. Чтение назад — CopyFromContainer
// (работает на created-контейнере), uid/mode берём из tar-заголовка stat'а.
func TestIntegration_ClaudeSkillsDeliveredToContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	cli := newIntegrationDockerClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	image := integrationSandboxImageRef(t)
	integrationSkipIfSandboxImageMissing(t, ctx, cli, image)

	created, err := cli.ContainerCreate(ctx, &containertypes.Config{
		Image:      image,
		Entrypoint: []string{"sleep"},
		Cmd:        []string{"60"},
	}, nil, nil, nil, "")
	require.NoError(t, err)
	containerID := created.ID
	t.Cleanup(func() {
		_ = cli.ContainerRemove(context.Background(), containerID,
			containertypes.RemoveOptions{Force: true})
	})

	bundle := &AgentSettingsBundle{
		SkillsFiles: map[string][]byte{
			"skill-proof/SKILL.md":       []byte("---\nname: skill-proof\ndescription: proof\n---\n"),
			"skill-proof/scripts/pong.py": []byte("print('PONG-d34dbeef')"),
		},
	}
	rc, err := buildClaudeSkillsTar(bundle, CodeBackendClaudeCode)
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer rc.Close()
	require.NoError(t, cli.CopyToContainer(ctx, containerID, "/", rc,
		containertypes.CopyToContainerOptions{}))

	// Читаем скрипт назад и проверяем содержимое + владельца.
	src := "/home/sandbox/.claude/skills/skill-proof/scripts/pong.py"
	rd, stat, err := cli.CopyFromContainer(ctx, containerID, src)
	require.NoError(t, err, "script must exist at %s", src)
	defer rd.Close()
	assert.Equal(t, int64(len("print('PONG-d34dbeef')")), stat.Size)

	tr := tar.NewReader(rd)
	hdr, err := tr.Next()
	require.NoError(t, err)
	var body strings.Builder
	_, err = io.Copy(&body, tr)
	require.NoError(t, err)
	assert.Equal(t, "print('PONG-d34dbeef')", body.String())
	assert.Equal(t, 1001, hdr.Uid, "files must be owned by sandbox user (uid 1001)")
	assert.Equal(t, int64(0o644), hdr.Mode&0o777)

	// SKILL.md тоже на месте.
	rd2, _, err := cli.CopyFromContainer(ctx, containerID,
		"/home/sandbox/.claude/skills/skill-proof/SKILL.md")
	require.NoError(t, err)
	_ = rd2.Close()
}

// То же для antigravity-пути (~/.gemini/antigravity/skills) на antigravity-образе,
// если он собран локально; иначе skip.
func TestIntegration_AntigravitySkillsPath(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	cli := newIntegrationDockerClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const image = "devteam/sandbox-antigravity:local"
	integrationSkipIfSandboxImageMissing(t, ctx, cli, image)

	created, err := cli.ContainerCreate(ctx, &containertypes.Config{
		Image:      image,
		Entrypoint: []string{"sleep"},
		Cmd:        []string{"60"},
	}, nil, nil, nil, "")
	require.NoError(t, err)
	containerID := created.ID
	t.Cleanup(func() {
		_ = cli.ContainerRemove(context.Background(), containerID,
			containertypes.RemoveOptions{Force: true})
	})

	bundle := &AgentSettingsBundle{
		SkillsFiles: map[string][]byte{"sk/SKILL.md": []byte("---\nname: sk\n---\n")},
	}
	rc, err := buildClaudeSkillsTar(bundle, CodeBackendAntigravity)
	require.NoError(t, err)
	defer rc.Close()
	require.NoError(t, cli.CopyToContainer(ctx, containerID, "/", rc,
		containertypes.CopyToContainerOptions{}))

	rd, _, err := cli.CopyFromContainer(ctx, containerID,
		"/home/sandbox/.gemini/antigravity/skills/sk/SKILL.md")
	require.NoError(t, err, "skill must land in ~/.gemini/antigravity/skills")
	_ = rd.Close()
}
