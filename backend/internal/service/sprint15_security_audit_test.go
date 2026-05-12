package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 15.37 — Security audit:
//   1) OAuth-токены и API-ключи провайдеров не сериализуются в JSON моделей.
//   2) sandbox.sensitiveEnvKey покрывает все три формы аутентификации (15.14).
//   3) settings.json / sandbox_permissions / code_backend_settings не уезжают в индексер Weaviate
//      (статический grep по пакету indexer + vectorloader — там нет ссылок на эти поля).
//   4) Логирование секретов в sandbox/options_log не печатает plaintext (используется маска).

func TestSprint15_37_LLMProvider_JSON_HidesCredentials(t *testing.T) {
	p := &models.LLMProvider{
		ID:                   uuid.New(),
		Name:                 "openrouter",
		Kind:                 models.LLMProviderKindOpenRouter,
		BaseURL:              "https://openrouter.ai/api/v1",
		CredentialsEncrypted: []byte("sk-secret-blob-must-not-leak"),
		DefaultModel:         "openrouter/auto",
		Enabled:              true,
	}
	out, err := json.Marshal(p)
	require.NoError(t, err)
	body := string(out)
	assert.NotContains(t, body, "sk-secret-blob-must-not-leak",
		"CredentialsEncrypted blob must never be serialized to JSON")
	assert.NotContains(t, body, "credentials_encrypted",
		"credentials_encrypted field must be hidden with json:\"-\"")
}

func TestSprint15_37_ClaudeCodeSubscription_JSON_HidesTokens(t *testing.T) {
	s := &models.ClaudeCodeSubscription{
		ID:                   uuid.New(),
		UserID:               uuid.New(),
		OAuthAccessTokenEnc:  []byte("access-token-blob-must-not-leak"),
		OAuthRefreshTokenEnc: []byte("refresh-token-blob-must-not-leak"),
		TokenType:            "Bearer",
		Scopes:               "user:inference",
	}
	out, err := json.Marshal(s)
	require.NoError(t, err)
	body := string(out)
	assert.NotContains(t, body, "access-token-blob-must-not-leak")
	assert.NotContains(t, body, "refresh-token-blob-must-not-leak")
}

func TestSprint15_37_SandboxOptions_Log_MasksSecrets(t *testing.T) {
	// Sprint 15.14 — все три формы (api key / OAuth / proxy bearer) считаются sensitive.
	opts := sandbox.SandboxOptions{
		EnvVars: map[string]string{
			sandbox.EnvAnthropicAPIKey:        "sk-ant-api03-veryveryverylongsecret",
			sandbox.EnvClaudeCodeOAuthToken:   "claude-code-oauth-token-XYZ",
			sandbox.EnvAnthropicAuthToken:     "proxy-bearer-token-ABC",
			"BENIGN": "no-secret-here",
		},
	}
	logRendered := opts.LogSafe()
	assert.NotContains(t, logRendered, "sk-ant-api03-veryveryverylongsecret",
		"ANTHROPIC_API_KEY plaintext must not appear in log render")
	assert.NotContains(t, logRendered, "claude-code-oauth-token-XYZ",
		"CLAUDE_CODE_OAUTH_TOKEN plaintext must not appear in log render")
	assert.NotContains(t, logRendered, "proxy-bearer-token-ABC",
		"ANTHROPIC_AUTH_TOKEN plaintext must not appear in log render")
	// Не-секретный ключ остаётся читаемым.
	assert.Contains(t, logRendered, "BENIGN")
}

// Static audit: индексер Weaviate не должен ссылаться на поля Sprint 15
// (settings.json / sandbox_permissions / code_backend_settings).
func TestSprint15_37_IndexerDoesNotReferenceSandboxSettings(t *testing.T) {
	roots := []string{
		findBackendRoot(t) + "/internal/indexer",
		findBackendRoot(t) + "/pkg/vectorloader",
	}
	bad := []string{
		"SandboxPermissions",
		"CodeBackendSettings",
		"sandbox_permissions",
		"code_backend_settings",
		"settings.json",
	}
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			body := string(data)
			for _, b := range bad {
				if strings.Contains(body, b) {
					t.Fatalf("forbidden reference %q found in %s: indexer must not index sandbox settings", b, path)
				}
			}
			return nil
		})
		require.NoError(t, err, "walk %s", root)
	}
}

// findBackendRoot — каталог backend по go.mod относительно текущего теста.
func findBackendRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// тест лежит в backend/internal/service — поднимаемся на 2 уровня.
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}
