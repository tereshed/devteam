package sandbox

import (
	"errors"
	"testing"
)

func TestSandboxOptions_Clone_deepCopiesEnvVars(t *testing.T) {
	opts := SandboxOptions{
		TaskID: "550e8400-e29b-41d4-a716-446655440000",
		EnvVars: map[string]string{
			EnvAnthropicAPIKey: "secret",
		},
	}
	cl := opts.Clone()
	if cl.EnvVars == nil {
		t.Fatal("nil env")
	}
	cl.EnvVars["X"] = "y"
	if _, ok := opts.EnvVars["X"]; ok {
		t.Fatal("mutating clone mutated original map")
	}
}

func TestSandboxOptions_Clone_nilEnvVars(t *testing.T) {
	opts := SandboxOptions{TaskID: "550e8400-e29b-41d4-a716-446655440000"}
	cl := opts.Clone()
	if cl.EnvVars != nil {
		t.Fatalf("want nil map, got %v", cl.EnvVars)
	}
}

func TestSandboxOptions_Validate_rejectsTaskIDSurroundingSpace(t *testing.T) {
	o := SandboxOptions{
		TaskID:      " 550e8400-e29b-41d4-a716-446655440000",
		Backend:     CodeBackendClaudeCode,
		Image:       "img",
		RepoURL:     "https://github.com/octocat/Hello-World.git",
		Branch:      "main",
		Instruction: "x",
	}
	err := o.Validate(nil)
	if err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected ErrInvalidOptions, got %v", err)
	}
}
