package sandbox

import (
	"archive/tar"
	"encoding/json"
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
		{"dotdot substring in name", "a..b", "", true},
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

func TestMergeSandboxEnv_InjectedEnvFilesManifest(t *testing.T) {
	opts := SandboxOptions{
		RepoURL: "https://example.com/r.git",
		Branch:  "main",
		Backend: CodeBackendClaudeCode,
		InjectedEnvFiles: []InjectedEnvFileSpec{
			{FileName: ".env", TargetDir: "", Content: "SECRET=should-not-appear"},
			{FileName: "local.json", TargetDir: "config", Content: "{\"x\":1}"},
		},
	}
	env := mergeSandboxEnv(opts)

	var manifest string
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvInjectEnvFiles+"=") {
			manifest = strings.TrimPrefix(kv, EnvInjectEnvFiles+"=")
		}
		// Содержимое (секрет) не должно протекать в env.
		if strings.Contains(kv, "should-not-appear") {
			t.Fatalf("injected file content leaked into env: %q", kv)
		}
	}
	if manifest == "" {
		t.Fatalf("expected %s in env, got %v", EnvInjectEnvFiles, env)
	}
	var entries []injectedEnvFileManifestEntry
	if err := json.Unmarshal([]byte(manifest), &entries); err != nil {
		t.Fatalf("manifest is not valid JSON: %v (%q)", err, manifest)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 manifest entries, got %d", len(entries))
	}
	if entries[0].Name != ".env" || entries[0].Idx != 0 {
		t.Errorf("entry0 mismatch: %+v", entries[0])
	}
	if entries[1].Name != "local.json" || entries[1].Dir != "config" || entries[1].Idx != 1 {
		t.Errorf("entry1 mismatch: %+v", entries[1])
	}
}

func TestMergeSandboxEnv_NoInjectedEnvFiles(t *testing.T) {
	opts := SandboxOptions{RepoURL: "https://example.com/r.git", Branch: "main", Backend: CodeBackendClaudeCode}
	env := mergeSandboxEnv(opts)
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvInjectEnvFiles+"=") {
			t.Fatalf("did not expect inject-env manifest without files, got %q", kv)
		}
	}
}

func TestBuildInjectedEnvFilesTar(t *testing.T) {
	if rc, _ := buildInjectedEnvFilesTar(nil); rc != nil {
		t.Fatal("expected nil reader for nil specs")
	}
	if rc, _ := buildInjectedEnvFilesTar([]InjectedEnvFileSpec{{FileName: ""}}); rc != nil {
		t.Fatal("expected nil reader when no valid file names")
	}

	specs := []InjectedEnvFileSpec{
		{FileName: ".env", Content: "A=1\n"},
		{FileName: "local.json", TargetDir: "config", Content: "{\"b\":2}"},
	}
	rc, err := buildInjectedEnvFilesTar(specs)
	if err != nil {
		t.Fatalf("buildInjectedEnvFilesTar: %v", err)
	}
	defer rc.Close()

	got := map[string]string{}
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar Next: %v", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			if hdr.Name != ".inject_env/" {
				t.Errorf("unexpected dir entry %q", hdr.Name)
			}
			continue
		}
		if hdr.Uid != 1001 || hdr.Gid != 1001 || hdr.Mode != 0o600 {
			t.Errorf("file %q wrong perms: uid=%d gid=%d mode=%o", hdr.Name, hdr.Uid, hdr.Gid, hdr.Mode)
		}
		body, _ := io.ReadAll(tr)
		got[hdr.Name] = string(body)
	}
	// idx совпадает с порядком в specs (см. injectedEnvFilesManifest).
	if got[".inject_env/0"] != "A=1\n" {
		t.Errorf("staged file 0 mismatch: %q", got[".inject_env/0"])
	}
	if got[".inject_env/1"] != "{\"b\":2}" {
		t.Errorf("staged file 1 mismatch: %q", got[".inject_env/1"])
	}
}

func TestInjectedEnvFilesManifest_SkipsEmptyNames(t *testing.T) {
	specs := []InjectedEnvFileSpec{
		{FileName: "", Content: "x"},
		{FileName: ".env", Content: "y"},
	}
	var entries []injectedEnvFileManifestEntry
	if err := json.Unmarshal([]byte(injectedEnvFilesManifest(specs)), &entries); err != nil {
		t.Fatalf("manifest unmarshal: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != ".env" || entries[0].Idx != 1 {
		t.Fatalf("expected single entry with idx=1, got %+v", entries)
	}
	// idx=1 указывает на staged-файл .inject_env/1, не на /0 — целостность пары манифест↔tar.
	_ = slices.Clip(entries)
}
