//go:build test_export

package service

import (
	"archive/tar"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// Sprint 15.36 — wiring-сценарий: агент с sandbox_permissions.defaultMode=bypassPermissions
// → AgentSettingsService.BuildArtifacts даёт PermissionMode=bypassPermissions →
// SandboxOptions.AgentSettings корректно конвертируется в bundle + env.
//
// Это «end-to-end на уровне процесса», без реального Docker. Полный E2E с claude code CLI
// покрывается build-tagged sandbox_integration тестом — оркестратор и так получает то же самое.
func TestSprint15_36_BypassPermissions_FullWiring(t *testing.T) {
	perms := SandboxPermissions{
		Allow:       []string{"Read", "Edit", "Write", "Bash(git commit:*)"},
		Deny:        []string{"Bash(rm -rf:*)"},
		DefaultMode: "bypassPermissions",
	}
	permsJSON, _ := json.Marshal(perms)

	// Минимальные code_backend_settings — без MCP / Skills.
	codeBackend := models.CodeBackendClaudeCode
	agent := &models.Agent{
		ID:                 uuid.New(),
		Role:               models.AgentRoleDeveloper,
		CodeBackend:        &codeBackend,
		SandboxPermissions: datatypes.JSON(permsJSON),
	}

	svc := NewAgentSettingsService()
	art, err := svc.BuildArtifacts(agent, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "bypassPermissions", art.PermissionMode)
	assert.NotEmpty(t, art.SettingsJSON)

	// Конвертация artifacts → sandbox bundle (как в orchestrator перед RunTask).
	bundle := &sandbox.AgentSettingsBundle{
		SettingsJSON:   art.SettingsJSON,
		MCPJSON:        art.MCPJSON,
		PermissionMode: art.PermissionMode,
	}

	// Симулируем то, что делает DockerSandboxRunner (заголовки/имена соблюдаются).
	opts := sandbox.SandboxOptions{
		TaskID:        "00000000-0000-0000-0000-000000000099",
		ProjectID:     "00000000-0000-0000-0000-000000000088",
		RepoURL:       "https://example.com/r.git",
		Branch:        "feat/test",
		Backend:       sandbox.CodeBackendType("claude-code"),
		AgentSettings: bundle,
		EnvVars: map[string]string{
			// Sandbox должен получить дефолтный ANTHROPIC_API_KEY (legacy path).
			sandbox.EnvAnthropicAPIKey: "sk-ant-something",
		},
	}

	env := sandbox.ExportMergeSandboxEnvForTesting(opts)
	assert.Contains(t, env, sandbox.EnvClaudeCodePermissionMode+"=bypassPermissions",
		"orchestrator должен пробросить permission mode в env для entrypoint")

	// Проверяем tar: settings.json лежит в .claude/, .mcp.json отсутствует (т.к. nil).
	rc, err := sandbox.ExportBuildPromptContextTarForTesting("instr", "ctx", bundle)
	require.NoError(t, err)
	defer rc.Close()
	names := readTarEntryNames(t, rc)
	assert.Contains(t, names, ".claude/settings.json")
	assert.NotContains(t, names, ".mcp.json", "MCPJSON был nil — .mcp.json не должен попасть")
}

func readTarEntryNames(t *testing.T, rc io.ReadCloser) []string {
	t.Helper()
	tr := tar.NewReader(rc)
	var out []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		out = append(out, h.Name)
		// прочитать тело хедера до конца, чтобы курсор tar Reader сдвинулся.
		_, _ = io.Copy(io.Discard, tr)
	}
	return out
}

// Sprint 15.36 — sandbox_permissions с defaultMode "" (пустой) → bundle без permission-mode env,
// entrypoint в этом случае откатится на --dangerously-skip-permissions (обратная совместимость).
func TestSprint15_36_NoDefaultMode_LegacyFallback(t *testing.T) {
	agent := &models.Agent{ID: uuid.New(), Role: models.AgentRoleDeveloper}
	art, err := NewAgentSettingsService().BuildArtifacts(agent, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, art.PermissionMode)
	assert.False(t, hasMCPJSON(art.SettingsJSON))
}

func hasMCPJSON(b []byte) bool {
	return strings.Contains(string(b), `"mcpServers"`)
}
