package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSandboxOptions_EffectiveTimeout(t *testing.T) {
	if d := (SandboxOptions{Timeout: 5 * time.Minute}).EffectiveTimeout(); d != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", d)
	}
	if d := (SandboxOptions{Timeout: 0}).EffectiveTimeout(); d != DefaultSandboxTimeout {
		t.Fatalf("expected DefaultSandboxTimeout, got %v", d)
	}
	if d := (SandboxOptions{Timeout: -1 * time.Second}).EffectiveTimeout(); d != DefaultSandboxTimeout {
		t.Fatalf("negative timeout should use default, got %v", d)
	}
}

func TestSandboxOptions_EffectiveStopGrace(t *testing.T) {
	if d := (SandboxOptions{StopGracePeriod: 3 * time.Second}).EffectiveStopGrace(); d != 3*time.Second {
		t.Fatalf("expected 3s, got %v", d)
	}
	if d := (SandboxOptions{StopGracePeriod: 0}).EffectiveStopGrace(); d != DefaultSandboxStopGrace {
		t.Fatalf("expected DefaultSandboxStopGrace, got %v", d)
	}
}

func TestValidateTaskID(t *testing.T) {
	ok := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"task-1",
		"abc_xyz-12",
	}
	for _, id := range ok {
		if err := ValidateTaskID(id); err != nil {
			t.Fatalf("expected ok %q: %v", id, err)
		}
	}
	bad := []string{"", " ", "bad branch", "a/b", "../x", strings.Repeat("x", 200)}
	for _, id := range bad {
		if err := ValidateTaskID(id); err == nil || !errors.Is(err, ErrInvalidTaskID) {
			t.Fatalf("expected ErrInvalidTaskID for %q: %v", id, err)
		}
	}
}

func TestValidateProjectID_emptyAllowed(t *testing.T) {
	if err := ValidateProjectID(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProjectID("   "); err == nil || !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("whitespace-only project id: %v", err)
	}
	if err := ValidateProjectID("550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxOptions_Validate(t *testing.T) {
	base := SandboxOptions{
		TaskID:      "550e8400-e29b-41d4-a716-446655440000",
		ProjectID:   "",
		Backend:     CodeBackendClaudeCode,
		Image:       "devteam/sandbox-claude:local",
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "feat/x",
		Instruction: "do work",
		EnvVars: map[string]string{
			EnvAnthropicAPIKey: "sk-ant-api03-test",
		},
		Timeout: 10 * time.Minute,
	}
	if err := base.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}

	emptyTask := base
	emptyTask.TaskID = " "
	if err := emptyTask.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("empty task: %v", err)
	}

	badBranch := base
	badBranch.Branch = "-evil"
	if err := badBranch.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) || !errors.Is(err, ErrInvalidBranchName) {
		t.Fatalf("bad branch: %v", err)
	}

	badRepo := base
	badRepo.RepoURL = "file:///etc/passwd"
	if err := badRepo.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) || !errors.Is(err, ErrInvalidRepoURL) {
		t.Fatalf("bad repo: %v", err)
	}

	negTimeout := base
	negTimeout.Timeout = -1 * time.Hour
	if err := negTimeout.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("negative timeout: %v", err)
	}

	negGrace := base
	negGrace.StopGracePeriod = -1 * time.Second
	if err := negGrace.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("negative stop grace: %v", err)
	}

	zeroTimeout := base
	zeroTimeout.Timeout = 0
	if err := zeroTimeout.Validate(context.Background()); err != nil {
		t.Fatalf("zero timeout must be valid for Validate: %v", err)
	}

	badProject := base
	badProject.ProjectID = "oops space"
	if err := badProject.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("bad project id: %v", err)
	}
}

func TestSandboxOptions_Validate_rejectsBranchDoubleSlash(t *testing.T) {
	base := SandboxOptions{
		TaskID:      "550e8400-e29b-41d4-a716-446655440000",
		Backend:     CodeBackendClaudeCode,
		Image:       "devteam/sandbox-claude:local",
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "feature//login",
		Instruction: "do",
		EnvVars:     map[string]string{EnvAnthropicAPIKey: "k"},
	}
	if err := base.Validate(context.Background()); err == nil || !errors.Is(err, ErrInvalidBranchName) {
		t.Fatalf("expected branch error: %v", err)
	}
}

func TestSandboxOptions_Validate_instructionAndContextAllowSurroundingWhitespace(t *testing.T) {
	o := SandboxOptions{
		TaskID:      "550e8400-e29b-41d4-a716-446655440000",
		Backend:     CodeBackendClaudeCode,
		Image:       "img",
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "main",
		Instruction: "\n# prompt\nrun tests\n",
		Context:       " context block \n",
		EnvVars:       map[string]string{EnvAnthropicAPIKey: "k"},
	}
	if err := o.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
