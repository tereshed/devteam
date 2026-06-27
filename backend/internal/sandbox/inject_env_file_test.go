package sandbox

import (
	"archive/tar"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestValidateInjectedEnvFile(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		dir     string
		wantErr bool
	}{
		{"simple root", ".env", "", false},
		{"nested dir", "local.json", "config/dev", false},
		{"empty file", "", "", true},
		{"slash in name", "a/b", "", true},
		{"dotdot in name", "..", "", true},
		{"dotdot segment in name", "a..b", "", true}, // "a..b" contains ".." substring → rejected (conservative)
		{"absolute dir", ".env", "/etc", true},
		{"dotdot in dir", ".env", "../secrets", true},
		{"dotdot deep in dir", ".env", "config/../../x", true},
		{"backslash name", ".env\\x", "", true},
		{"newline name", ".env\nFOO=1", "", true},
		{"backslash dir", ".env", "con\\fig", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateInjectedEnvFile(c.file, c.dir)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for file=%q dir=%q, got nil", c.file, c.dir)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error for file=%q dir=%q: %v", c.file, c.dir, err)
			}
		})
	}
}

func TestMergeSandboxEnv_InjectedEnvFileMetadata(t *testing.T) {
	opts := SandboxOptions{
		RepoURL: "https://example.com/r.git",
		Branch:  "main",
		Backend: CodeBackendClaudeCode,
		InjectedEnvFile: &InjectedEnvFileSpec{
			FileName:  ".env",
			TargetDir: "config",
			Content:   "SECRET=should-not-appear-as-env",
		},
	}
	env := mergeSandboxEnv(opts)

	if !slices.Contains(env, EnvInjectEnvFileName+"=.env") {
		t.Errorf("expected %s=.env in env, got %v", EnvInjectEnvFileName, env)
	}
	if !slices.Contains(env, EnvInjectEnvFileDir+"=config") {
		t.Errorf("expected %s=config in env, got %v", EnvInjectEnvFileDir, env)
	}
	// Содержимое (секрет) НЕ должно протекать в env.
	for _, kv := range env {
		if strings.Contains(kv, "should-not-appear-as-env") {
			t.Fatalf("injected file content leaked into env: %q", kv)
		}
	}
}

func TestMergeSandboxEnv_NoInjectedEnvFile(t *testing.T) {
	opts := SandboxOptions{RepoURL: "https://example.com/r.git", Branch: "main", Backend: CodeBackendClaudeCode}
	env := mergeSandboxEnv(opts)
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvInjectEnvFileName+"=") || strings.HasPrefix(kv, EnvInjectEnvFileDir+"=") {
			t.Fatalf("did not expect inject-env metadata without InjectedEnvFile, got %q", kv)
		}
	}
}

func TestBuildInjectedEnvFileTar(t *testing.T) {
	if rc, _ := buildInjectedEnvFileTar(nil); rc != nil {
		t.Fatal("expected nil reader for nil spec")
	}
	if rc, _ := buildInjectedEnvFileTar(&InjectedEnvFileSpec{FileName: ""}); rc != nil {
		t.Fatal("expected nil reader for empty file name")
	}

	const content = "DATABASE_URL=postgres://x\nDEBUG=1\n"
	rc, err := buildInjectedEnvFileTar(&InjectedEnvFileSpec{FileName: ".env", TargetDir: "config", Content: content})
	if err != nil {
		t.Fatalf("buildInjectedEnvFileTar: %v", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tar Next: %v", err)
	}
	if hdr.Name != ".inject_env_file" {
		t.Errorf("expected staged name .inject_env_file, got %q", hdr.Name)
	}
	if hdr.Uid != 1001 || hdr.Gid != 1001 {
		t.Errorf("expected uid/gid 1001, got %d/%d", hdr.Uid, hdr.Gid)
	}
	if hdr.Mode != 0o600 {
		t.Errorf("expected mode 0600, got %o", hdr.Mode)
	}
	body, _ := io.ReadAll(tr)
	if string(body) != content {
		t.Errorf("staged content mismatch: got %q", string(body))
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Errorf("expected single tar entry, got more")
	}
}
