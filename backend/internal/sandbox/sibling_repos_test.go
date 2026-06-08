package sandbox

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Мульти-репо: соседние репозитории сериализуются в env SIBLING_REPOS как JSON-массив
// {slug,url,branch}, который entrypoint парсит и клонирует read-only.
func TestMergeSandboxEnv_SiblingRepos(t *testing.T) {
	opts := SandboxOptions{
		TaskID:  "00000000-0000-0000-0000-000000000001",
		RepoURL: "https://example.com/core.git",
		Branch:  "feat/test",
		Backend: "claude-code",
		EnvVars: map[string]string{},
		SiblingRepos: []SiblingRepoSpec{
			{Slug: "ui", RepoURL: "https://example.com/ui.git", Branch: "main"},
			{Slug: "infra", RepoURL: "https://example.com/infra.git", Branch: ""},
		},
	}

	env := mergeSandboxEnv(opts)

	var blob string
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvSiblingRepos+"=") {
			blob = strings.TrimPrefix(kv, EnvSiblingRepos+"=")
		}
	}
	require.NotEmpty(t, blob, "SIBLING_REPOS must be present in env")

	var got []SiblingRepoSpec
	require.NoError(t, json.Unmarshal([]byte(blob), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "ui", got[0].Slug)
	assert.Equal(t, "https://example.com/ui.git", got[0].RepoURL)
	assert.Equal(t, "main", got[0].Branch)
	assert.Equal(t, "infra", got[1].Slug)
	assert.Equal(t, "", got[1].Branch)
}

// Без соседних репо env не содержит SIBLING_REPOS (обратная совместимость одно-репо).
func TestMergeSandboxEnv_NoSiblings(t *testing.T) {
	opts := SandboxOptions{
		TaskID:  "00000000-0000-0000-0000-000000000001",
		RepoURL: "https://example.com/core.git",
		Branch:  "feat/test",
		Backend: "claude-code",
		EnvVars: map[string]string{},
	}
	for _, kv := range mergeSandboxEnv(opts) {
		assert.False(t, strings.HasPrefix(kv, EnvSiblingRepos+"="), "SIBLING_REPOS must be absent without siblings")
	}
}
