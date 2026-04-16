package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionInput_String_MasksEnvSecrets(t *testing.T) {
	t.Parallel()

	secretValue := "super-secret-token-must-not-leak-7f3a9c2e"
	in := ExecutionInput{
		TaskID:    "task-1",
		ProjectID: "proj-1",
		EnvSecrets: map[string]string{
			"GITHUB_TOKEN": secretValue,
			"API_KEY":      "another-" + secretValue,
		},
	}

	out := in.String()
	require.NotContains(t, out, secretValue, "secret values must never appear in String()")
	require.NotContains(t, out, "another-", "secret values must never appear in String()")
	require.Contains(t, out, `"API_KEY":***`)
	require.Contains(t, out, `"GITHUB_TOKEN":***`)
}

func TestExecutionInput_String_EmptyEnvSecrets(t *testing.T) {
	t.Parallel()

	in := ExecutionInput{TaskID: "t", EnvSecrets: map[string]string{}}
	require.Contains(t, in.String(), "EnvSecrets:{}")
}

func TestNormalizeJSONForParse(t *testing.T) {
	t.Parallel()

	require.Equal(t, "{}", string(NormalizeJSONForParse(nil)))
	require.Equal(t, "{}", string(NormalizeJSONForParse(jsonRawEmpty())))

	raw := json.RawMessage(`{"a":1}`)
	require.Equal(t, `{"a":1}`, string(NormalizeJSONForParse(raw)))
}

// json.RawMessage(nil) and json.RawMessage{} both have len 0
func jsonRawEmpty() json.RawMessage {
	var r json.RawMessage
	return r
}
