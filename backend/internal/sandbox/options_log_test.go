package sandbox

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSandboxOptions_LogSafe_masksSecrets(t *testing.T) {
	opts := SandboxOptions{
		TaskID:      "t1",
		ProjectID:   "p1",
		Backend:     CodeBackendClaudeCode,
		Image:       "devteam/sandbox-claude:local",
		RepoURL:     "https://user:supersecret@github.com/org/repo.git",
		Branch:      "feat/x",
		Instruction: strings.Repeat("x", 5000),
		Context:     "ctx",
		EnvVars: map[string]string{
			EnvAnthropicAPIKey: "sk-ant-api03-abcdefghijklmnop",
			"PLAIN":            "visible",
			"GITHUB_TOKEN":     "ghp_abcdefghijklmnopqrst",
		},
	}
	s := opts.LogSafe()
	if strings.Contains(s, "supersecret") {
		t.Fatal("RepoURL password leaked")
	}
	if strings.Contains(s, "sk-ant-api03") {
		t.Fatal("API key leaked")
	}
	if strings.Contains(s, "ghp_abc") {
		t.Fatal("GITHUB_TOKEN leaked")
	}
	if !strings.Contains(s, "visible") {
		t.Fatal("expected non-secret env preserved")
	}
	if !strings.Contains(s, "<5000 bytes>") {
		t.Fatal("expected instruction size hint")
	}
	if strings.Contains(s, strings.Repeat("x", 100)) {
		t.Fatal("instruction body should not appear")
	}
}

func TestSandboxOptions_String_sameAsLogSafe(t *testing.T) {
	opts := SandboxOptions{EnvVars: map[string]string{EnvAnthropicAPIKey: "x"}}
	if opts.String() != opts.LogSafe() {
		t.Fatal("String() must match LogSafe()")
	}
}

func Test_maskRepoURL_scpMasksUserBeforeAt(t *testing.T) {
	raw := "ghp_supersecret12345@github.com:org/repo.git"
	out := maskRepoURL(raw)
	if strings.Contains(out, "supersecret") {
		t.Fatal("SCP-style token leaked")
	}
	if !strings.HasPrefix(out, "***@github.com:") {
		t.Fatalf("unexpected: %q", out)
	}
}

func Test_maskSecretValue_runeSafePrefix(t *testing.T) {
	// длинная строка с кириллицей: срез по рунам не должен давать битый UTF-8 в %q
	v := "пароль" + strings.Repeat("x", 20) + "конец"
	s := maskSecretValue(v)
	if !utf8.ValidString(s) {
		t.Fatalf("invalid UTF-8 in log mask: %q", s)
	}
}
