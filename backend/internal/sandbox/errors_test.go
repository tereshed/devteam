package sandbox

import (
	"errors"
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	ok := []string{"main", "feat/foo-bar", "issue_123", "release/v2.0.1"}
	for _, b := range ok {
		if err := ValidateBranchName(b); err != nil {
			t.Fatalf("expected ok for %q: %v", b, err)
		}
	}

	bad := []string{
		"",
		"-rf",
		".hidden",
		"/abs",
		"trailing/",
		"bad..branch",
		"x.lock",
		"has space",
		"a@{b",
		"q~x",
		"too-long-" + string(make([]byte, 250)),
	}
	for _, b := range bad {
		if err := ValidateBranchName(b); err == nil {
			t.Fatalf("expected error for %q", b)
		} else if !errors.Is(err, ErrInvalidBranchName) {
			t.Fatalf("expected ErrInvalidBranchName for %q: %v", b, err)
		}
	}
}

func TestValidateEnvKeys(t *testing.T) {
	if err := ValidateEnvKeys(nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvKeys(map[string]string{
		EnvRepoURL: "x",
		"APP_EXTRA": "y",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvKeys(map[string]string{"PATH": "/tmp"}); err == nil || !errors.Is(err, ErrInvalidEnvKeys) {
		t.Fatalf("PATH: %v", err)
	}
	if err := ValidateEnvKeys(map[string]string{"LD_PRELOAD": "/evil.so"}); err == nil || !errors.Is(err, ErrInvalidEnvKeys) {
		t.Fatalf("LD_PRELOAD: %v", err)
	}
	if err := ValidateEnvKeys(map[string]string{"BAD=KEY": "v"}); err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestValidateRepoURL(t *testing.T) {
	ok := []string{
		"https://github.com/octocat/Hello-World.git",
		"http://git.example.com/project.git",
		"git://git.example.com/project.git",
		"ssh://git@github.com/octocat/Hello-World.git",
		"git@github.com:octocat/Hello-World.git",
	}
	for _, u := range ok {
		if err := ValidateRepoURL(u); err != nil {
			t.Fatalf("expected ok %q: %v", u, err)
		}
	}
	bad := []string{
		"",
		"   ",
		"file:///etc/passwd",
		"ftp://git.example.com/x.git",
		"https://127.0.0.1/x.git",
		"https://localhost/x.git",
		"http://169.254.169.254/latest/meta-data/",
		"git@127.0.0.1:evil.git",
	}
	for _, u := range bad {
		if err := ValidateRepoURL(u); err == nil || !errors.Is(err, ErrInvalidRepoURL) {
			t.Fatalf("expected ErrInvalidRepoURL for %q: %v", u, err)
		}
	}
}

func TestSandboxStatus_HasResult(t *testing.T) {
	if (*SandboxStatus)(nil).HasResult() {
		t.Fatal("nil receiver")
	}
	s := &SandboxStatus{Result: nil}
	if s.HasResult() {
		t.Fatal("nil Result")
	}
	s.Result = &CodeResult{}
	if !s.HasResult() {
		t.Fatal("want true")
	}
}
