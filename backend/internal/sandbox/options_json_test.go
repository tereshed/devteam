package sandbox

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSandboxOptions_MarshalJSON_masksSecrets(t *testing.T) {
	opts := SandboxOptions{
		TaskID:      "550e8400-e29b-41d4-a716-446655440000",
		ProjectID:   "p1",
		Backend:     CodeBackendClaudeCode,
		Image:       "devteam/sandbox-claude:local",
		RepoURL:     "https://user:secretpass@github.com/org/repo.git",
		Branch:      "main",
		Instruction: strings.Repeat("z", 100),
		Context:     "c",
		EnvVars: map[string]string{
			EnvAnthropicAPIKey: "sk-ant-api03-abcdefghijklmnop",
			"PLAIN":            "visible",
			"CUSTOM_KEY":       "hide-me",
		},
		Timeout: 1 * time.Second,
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "secretpass") {
		t.Fatal("repo password leaked in JSON")
	}
	if strings.Contains(s, "sk-ant-api03") {
		t.Fatal("anthropic key leaked")
	}
	if strings.Contains(s, "hide-me") {
		t.Fatal("CUSTOM_KEY value leaked")
	}
	if !strings.Contains(s, `"PLAIN":"visible"`) {
		t.Fatal("expected non-secret env visible")
	}
	if !strings.Contains(s, "100 bytes") {
		t.Fatalf("expected instruction size placeholder, got: %s", s)
	}
}
