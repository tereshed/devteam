package sandbox

import (
	"archive/tar"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 15.27 — wiring-тест для bypassPermissions.
//
// Полный E2E-прогон с реальным Docker-контейнером покрыт sandbox_real_test.go (под build-тегом
// `sandbox_integration`). Здесь — детерминированная проверка, что:
//   1) AgentSettingsBundle с PermissionMode=bypassPermissions попадает в env как
//      CLAUDE_CODE_PERMISSION_MODE=bypassPermissions (entrypoint выберет --dangerously-skip-permissions);
//   2) settings.json и .mcp.json упаковываются в tar для CopyToContainer.
//
// Эти инварианты — то, что entrypoint.sh ожидает увидеть; полный «без зависания на confirm»
// гарантируется (а) bypassPermissions → --dangerously-skip-permissions, (б) headless-режимом CLI.
func TestSandboxBundle_BypassPermissions_Wiring(t *testing.T) {
	bundle := &AgentSettingsBundle{
		SettingsJSON:   []byte(`{"permissions":{"defaultMode":"bypassPermissions"}}`),
		MCPJSON:        []byte(`{"mcpServers":{}}`),
		PermissionMode: "bypassPermissions",
	}
	opts := SandboxOptions{
		TaskID:        "00000000-0000-0000-0000-000000000001",
		ProjectID:     "00000000-0000-0000-0000-000000000002",
		RepoURL:       "https://example.com/repo.git",
		Branch:        "feat/test",
		Backend:       "claude-code",
		AgentSettings: bundle,
		EnvVars:       map[string]string{},
	}

	// Env должен содержать CLAUDE_CODE_PERMISSION_MODE.
	env := mergeSandboxEnv(opts)
	assert.Contains(t, env, EnvClaudeCodePermissionMode+"=bypassPermissions",
		"permission mode must be propagated to container env for entrypoint")

	// Tar должен содержать prompt/context + .claude/settings.json + .mcp.json.
	rc, err := buildPromptContextTar("instr", "ctx", bundle)
	require.NoError(t, err)
	defer rc.Close()
	names := readTarNames(t, rc)
	assert.Contains(t, names, "prompt.txt")
	assert.Contains(t, names, "context.txt")
	assert.Contains(t, names, ".claude/settings.json")
	assert.Contains(t, names, ".mcp.json")
}

func TestSandboxBundle_NilBundle_BackwardCompatible(t *testing.T) {
	// Без AgentSettings ничего лишнего ни в tar, ни в env (legacy).
	opts := SandboxOptions{
		TaskID:  "00000000-0000-0000-0000-000000000003",
		RepoURL: "https://example.com/r.git",
		Branch:  "main",
		Backend: "claude-code",
	}
	env := mergeSandboxEnv(opts)
	for _, e := range env {
		assert.NotContains(t, e, EnvClaudeCodePermissionMode+"=",
			"no permission mode env expected when AgentSettings is nil")
	}
	rc, err := buildPromptContextTar("a", "b", nil)
	require.NoError(t, err)
	defer rc.Close()
	names := readTarNames(t, rc)
	assert.NotContains(t, names, ".claude/settings.json")
	assert.NotContains(t, names, ".mcp.json")
}

func TestSandboxBundle_PermissionModeAcceptEdits(t *testing.T) {
	opts := SandboxOptions{
		TaskID:        "00000000-0000-0000-0000-000000000004",
		RepoURL:       "https://example.com/r.git",
		Branch:        "main",
		Backend:       "claude-code",
		AgentSettings: &AgentSettingsBundle{PermissionMode: "acceptEdits"},
	}
	env := mergeSandboxEnv(opts)
	assert.Contains(t, env, EnvClaudeCodePermissionMode+"=acceptEdits")
}

// Sprint 15.M5 regression: невалидный mode не должен попасть в env
// (попытка инъекции "default\n--evil-flag" игнорируется).
func TestSandboxBundle_PermissionMode_RejectsInjection(t *testing.T) {
	cases := []string{
		"default\n--evil-flag",
		"acceptEdits;rm -rf /",
		"bogus",
		"--dangerously-skip-permissions",
	}
	for _, mode := range cases {
		opts := SandboxOptions{
			TaskID:        "00000000-0000-0000-0000-000000000005",
			RepoURL:       "https://example.com/r.git",
			Branch:        "main",
			Backend:       "claude-code",
			AgentSettings: &AgentSettingsBundle{PermissionMode: mode},
		}
		env := mergeSandboxEnv(opts)
		for _, e := range env {
			assert.False(t,
				len(e) > len(EnvClaudeCodePermissionMode) &&
					e[:len(EnvClaudeCodePermissionMode)] == EnvClaudeCodePermissionMode,
				"permission_mode=%q must not be exported, got %q", mode, e)
		}
	}
}

func readTarNames(t *testing.T, rc io.ReadCloser) []string {
	t.Helper()
	tr := tar.NewReader(rc)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, h.Name)
	}
	return names
}
