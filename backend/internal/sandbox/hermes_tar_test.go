package sandbox

import (
	"archive/tar"
	"io"
	"strings"
	"testing"
)

// Sprint 16.C — buildHermesHomeTar собирает корректный tar c правами 0600/0700/0644
// и не пропускает path-traversal в ключах Skills.

func TestBuildHermesHomeTar_NilBundleEmpty(t *testing.T) {
	rc, err := buildHermesHomeTar(nil)
	if err != nil {
		t.Fatalf("nil bundle: %v", err)
	}
	if rc != nil {
		t.Fatalf("nil bundle: expected nil reader")
	}
}

func TestBuildHermesHomeTar_NoHermesArtifactsEmpty(t *testing.T) {
	b := &AgentSettingsBundle{SettingsJSON: []byte("{}")}
	rc, err := buildHermesHomeTar(b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rc != nil {
		t.Fatalf("expected nil tar (no hermes fields)")
	}
}

func TestBuildHermesHomeTar_FullBundle(t *testing.T) {
	b := &AgentSettingsBundle{
		HermesConfigYAML: []byte("display:\n  banner: false\n"),
		HermesMCPJSON:    []byte(`{"mcpServers":[]}`),
		HermesSkills: map[string][]byte{
			"checklist/SKILL.md": []byte("# checklist"),
			"styleguide/SKILL.md": []byte("# style"),
		},
	}
	rc, err := buildHermesHomeTar(b)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rc == nil {
		t.Fatalf("nil reader for non-empty bundle")
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	type fileSpec struct{ name string; mode int64; isDir bool }
	got := map[string]fileSpec{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		got[hdr.Name] = fileSpec{name: hdr.Name, mode: hdr.Mode, isDir: hdr.Typeflag == tar.TypeDir}
	}
	wantPerms := map[string]int64{
		"home/sandbox/.hermes":                         0o700,
		"home/sandbox/.hermes/config.yaml":             0o600,
		"home/sandbox/.hermes/mcp.json":                0o600,
		"home/sandbox/.hermes/skills":                  0o700,
		"home/sandbox/.hermes/skills/checklist/SKILL.md":  0o644,
		"home/sandbox/.hermes/skills/styleguide/SKILL.md": 0o644,
	}
	for name, mode := range wantPerms {
		fs, ok := got[name]
		if !ok {
			t.Errorf("missing %q in tar", name)
			continue
		}
		if fs.mode != mode {
			t.Errorf("%q mode = %o, want %o", name, fs.mode, mode)
		}
	}
}

func TestBuildHermesHomeTar_RejectsTraversalKey(t *testing.T) {
	b := &AgentSettingsBundle{
		HermesSkills: map[string][]byte{
			"../../etc/passwd": []byte("evil"),
		},
	}
	if _, err := buildHermesHomeTar(b); err == nil || !strings.Contains(err.Error(), "parent traversal") {
		t.Fatalf("expected parent-traversal rejection, got %v", err)
	}
}

func TestBuildHermesHomeTar_RejectsAbsoluteKey(t *testing.T) {
	b := &AgentSettingsBundle{
		HermesSkills: map[string][]byte{
			"/tmp/evil": []byte("x"),
		},
	}
	if _, err := buildHermesHomeTar(b); err == nil {
		t.Fatalf("expected rejection of absolute path")
	}
}

func TestValidateEnvKeys_HermesMCPPrefixAllowed(t *testing.T) {
	if err := ValidateEnvKeys(map[string]string{
		"HERMES_MCP_GITHUB_TOKEN": "x",
		"DEVTEAM_HERMES_TOOLSETS": "file_ops,shell",
	}); err != nil {
		t.Fatalf("expected accepted, got %v", err)
	}
}

func TestValidateEnvKeys_RejectsArbitraryHermesKey(t *testing.T) {
	if err := ValidateEnvKeys(map[string]string{"HERMES_RANDOM_FOO": "x"}); err == nil {
		t.Fatalf("expected rejection")
	}
}

// Sprint 16.C — mergeSandboxEnv должен включать HermesEnv из AgentSettings, иначе
// DEVTEAM_HERMES_PERMISSION_MODE никогда не доедет до entrypoint, и фича умрёт
// по дороге между сервисом и контейнером.
func TestMergeSandboxEnv_HermesEnvMerged(t *testing.T) {
	opts := SandboxOptions{
		RepoURL: "https://example.com/r.git",
		Branch:  "main",
		Backend: CodeBackendHermes,
		AgentSettings: &AgentSettingsBundle{
			HermesEnv: map[string]string{
				"DEVTEAM_HERMES_PERMISSION_MODE": "accept",
				"DEVTEAM_HERMES_TOOLSETS":        "file_ops,shell",
				"HERMES_MCP_FOO_TOKEN":           "abc",
			},
		},
	}
	env := mergeSandboxEnv(opts)
	want := map[string]bool{
		"DEVTEAM_HERMES_PERMISSION_MODE=accept": true,
		"DEVTEAM_HERMES_TOOLSETS=file_ops,shell": true,
		"HERMES_MCP_FOO_TOKEN=abc":               true,
	}
	for _, e := range env {
		delete(want, e)
	}
	if len(want) > 0 {
		t.Fatalf("missing env entries after merge: %v\nfull env: %v", want, env)
	}
}

func TestMergeSandboxEnv_EnvVarsOverrideHermesEnv(t *testing.T) {
	// Пользовательский EnvVars побеждает HermesEnv (тот же ключ).
	opts := SandboxOptions{
		RepoURL: "https://example.com/r.git",
		Branch:  "main",
		Backend: CodeBackendHermes,
		EnvVars: map[string]string{"DEVTEAM_HERMES_PERMISSION_MODE": "yolo"},
		AgentSettings: &AgentSettingsBundle{
			HermesEnv: map[string]string{"DEVTEAM_HERMES_PERMISSION_MODE": "accept"},
		},
	}
	env := mergeSandboxEnv(opts)
	gotAccept, gotYolo := false, false
	for _, e := range env {
		if e == "DEVTEAM_HERMES_PERMISSION_MODE=accept" {
			gotAccept = true
		}
		if e == "DEVTEAM_HERMES_PERMISSION_MODE=yolo" {
			gotYolo = true
		}
	}
	if gotAccept {
		t.Fatalf("HermesEnv overrode user EnvVars (priority wrong)")
	}
	if !gotYolo {
		t.Fatalf("user EnvVars value lost")
	}
}
