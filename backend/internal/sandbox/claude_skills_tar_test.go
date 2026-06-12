package sandbox

import (
	"archive/tar"
	"io"
	"strings"
	"testing"
)

// Sprint 22 — buildClaudeSkillsTar: доставка skills claude-семейства в home
// контейнера (~/.claude/skills | ~/.gemini/antigravity/skills) с защитой от traversal.

func TestBuildClaudeSkillsTar_NilAndEmpty(t *testing.T) {
	if rc, err := buildClaudeSkillsTar(nil, CodeBackendClaudeCode); err != nil || rc != nil {
		t.Fatalf("nil bundle: rc=%v err=%v", rc, err)
	}
	b := &AgentSettingsBundle{SettingsJSON: []byte("{}")}
	if rc, err := buildClaudeSkillsTar(b, CodeBackendClaudeCode); err != nil || rc != nil {
		t.Fatalf("empty skills: rc=%v err=%v", rc, err)
	}
}

func readTarEntries(t *testing.T, rc io.ReadCloser) map[string]struct {
	mode    int64
	isDir   bool
	content string
} {
	t.Helper()
	defer rc.Close()
	tr := tar.NewReader(rc)
	out := map[string]struct {
		mode    int64
		isDir   bool
		content string
	}{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		var body strings.Builder
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.Copy(&body, tr); err != nil { //nolint:gosec // тестовые данные
				t.Fatalf("tar body: %v", err)
			}
		}
		out[hdr.Name] = struct {
			mode    int64
			isDir   bool
			content string
		}{mode: hdr.Mode, isDir: hdr.Typeflag == tar.TypeDir, content: body.String()}
	}
	return out
}

func TestBuildClaudeSkillsTar_ClaudeCodeLayout(t *testing.T) {
	b := &AgentSettingsBundle{
		SkillsFiles: map[string][]byte{
			"deploy-check/SKILL.md":        []byte("# skill"),
			"deploy-check/scripts/run.py":  []byte("print('ok')"),
			"style/SKILL.md":               []byte("# style"),
		},
	}
	rc, err := buildClaudeSkillsTar(b, CodeBackendClaudeCode)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rc == nil {
		t.Fatalf("nil reader for non-empty skills")
	}
	got := readTarEntries(t, rc)

	wantDirs := []string{
		"home/sandbox/.claude",
		"home/sandbox/.claude/skills",
		"home/sandbox/.claude/skills/deploy-check",
		"home/sandbox/.claude/skills/deploy-check/scripts",
		"home/sandbox/.claude/skills/style",
	}
	for _, d := range wantDirs {
		e, ok := got[d]
		if !ok || !e.isDir {
			t.Errorf("dir %q missing or not a dir (entries: %d)", d, len(got))
		}
	}
	f, ok := got["home/sandbox/.claude/skills/deploy-check/scripts/run.py"]
	if !ok || f.isDir {
		t.Fatalf("script file missing")
	}
	if f.mode != 0o644 {
		t.Errorf("script mode = %o, want 0644", f.mode)
	}
	if f.content != "print('ok')" {
		t.Errorf("script content = %q", f.content)
	}
}

func TestBuildClaudeSkillsTar_AntigravityBase(t *testing.T) {
	b := &AgentSettingsBundle{
		SkillsFiles: map[string][]byte{"sk/SKILL.md": []byte("x")},
	}
	rc, err := buildClaudeSkillsTar(b, CodeBackendAntigravity)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	got := readTarEntries(t, rc)
	if _, ok := got["home/sandbox/.gemini/antigravity/skills/sk/SKILL.md"]; !ok {
		t.Fatalf("antigravity skills path missing, got %d entries", len(got))
	}
}

func TestBuildClaudeSkillsTar_RejectsUnsupportedBackend(t *testing.T) {
	b := &AgentSettingsBundle{
		SkillsFiles: map[string][]byte{"sk/SKILL.md": []byte("x")},
	}
	if _, err := buildClaudeSkillsTar(b, CodeBackendHermes); err == nil {
		t.Fatalf("hermes backend with claude SkillsFiles must error (config bug, fail loud)")
	}
}

func TestBuildClaudeSkillsTar_RejectsTraversalKeys(t *testing.T) {
	for _, key := range []string{"../evil", "/abs/path", "a/../../b", "~/home"} {
		b := &AgentSettingsBundle{
			SkillsFiles: map[string][]byte{key: []byte("evil")},
		}
		if _, err := buildClaudeSkillsTar(b, CodeBackendClaudeCode); err == nil {
			t.Errorf("key %q: expected traversal error", key)
		}
	}
}
