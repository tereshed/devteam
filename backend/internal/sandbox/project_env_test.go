package sandbox

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReservedSandboxEnvKey(t *testing.T) {
	reserved := []string{
		"GIT_TOKEN", "GITHUB_TOKEN", "REPO_URL", "BRANCH_NAME",
		"ANTHROPIC_API_KEY", "DEVTEAM_AGENT_MODEL", "DEVTEAM_FOO",
		"HERMES_API_KEY", "HERMES_MCP_X_Y", "MCP_GITHUB_TOKEN", "APP_X",
		"git_token", // нормализация регистра
	}
	for _, k := range reserved {
		assert.True(t, IsReservedSandboxEnvKey(k), "%q must be reserved", k)
	}

	free := []string{"DATABASE_URL", "OPENAI_API_KEY", "MY_VAR", "STRIPE_KEY"}
	for _, k := range free {
		assert.False(t, IsReservedSandboxEnvKey(k), "%q must NOT be reserved", k)
	}
}

func TestValidateProjectEnvKeys(t *testing.T) {
	// Валидные пользовательские ключи.
	require.NoError(t, ValidateProjectEnvKeys(map[string]string{
		"DATABASE_URL": "postgres://x",
		"OPENAI_KEY":   "sk-xxx",
	}))

	// Зарезервированный ключ — ошибка.
	err := ValidateProjectEnvKeys(map[string]string{"GIT_TOKEN": "x"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidEnvKeys))

	// Невалидный синтаксис (нижний регистр допустим синтаксически, но проверим мусор).
	require.Error(t, ValidateProjectEnvKeys(map[string]string{"BAD-KEY": "x"}))
	require.Error(t, ValidateProjectEnvKeys(map[string]string{"": "x"}))

	// Зарезервированный префикс.
	require.Error(t, ValidateProjectEnvKeys(map[string]string{"DEVTEAM_X": "x"}))
}

// ProjectEnv попадает в env контейнера, но с НИЗШИМ приоритетом: системный EnvVars
// с тем же ключом должен победить (последняя пара K=V в docker выигрывает).
func TestMergeSandboxEnv_ProjectEnvLowestPriority(t *testing.T) {
	opts := SandboxOptions{
		TaskID:  "00000000-0000-0000-0000-000000000001",
		RepoURL: "https://example.com/core.git",
		Branch:  "feat/test",
		Backend: "claude-code",
		EnvVars: map[string]string{"GIT_TOKEN": "system-token"},
		ProjectEnv: map[string]string{
			"DATABASE_URL": "postgres://proj",
		},
	}
	env := mergeSandboxEnv(opts)

	// DATABASE_URL присутствует.
	assert.Contains(t, env, "DATABASE_URL=postgres://proj")

	// ProjectEnv добавлен раньше EnvVars (ниже приоритет): индекс project < индекс git.
	idxProject, idxGit := -1, -1
	for i, kv := range env {
		if strings.HasPrefix(kv, "DATABASE_URL=") {
			idxProject = i
		}
		if strings.HasPrefix(kv, "GIT_TOKEN=") {
			idxGit = i
		}
	}
	require.GreaterOrEqual(t, idxProject, 0)
	require.GreaterOrEqual(t, idxGit, 0)
	assert.Less(t, idxProject, idxGit, "ProjectEnv должен идти раньше системных EnvVars (низший приоритет)")
}

// Значения ProjectEnv не должны утекать в логи/JSON (произвольные имена не ловятся эвристикой).
func TestProjectEnv_MaskedInLogsAndJSON(t *testing.T) {
	opts := SandboxOptions{
		TaskID:     "00000000-0000-0000-0000-000000000001",
		RepoURL:    "https://example.com/core.git",
		Branch:     "main",
		Backend:    "claude-code",
		ProjectEnv: map[string]string{"DATABASE_URL": "postgres://secret-host/db"},
	}

	logSafe := opts.LogSafe()
	assert.NotContains(t, logSafe, "secret-host", "значение ProjectEnv не должно попадать в лог")
	assert.Contains(t, logSafe, "DATABASE_URL=***")

	blob, err := json.Marshal(opts)
	require.NoError(t, err)
	assert.NotContains(t, string(blob), "secret-host", "значение ProjectEnv не должно попадать в JSON")
	assert.Contains(t, string(blob), `"DATABASE_URL":"***"`)
}
