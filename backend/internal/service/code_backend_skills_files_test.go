package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
)

// Sprint 22 — buildCodeBackendSkillsFiles: дерево файлов skills для
// claude-code / antigravity (config.files → map rel-path → content).

func TestBuildCodeBackendSkillsFiles_EmptyNil(t *testing.T) {
	out, err := buildCodeBackendSkillsFiles(nil)
	if err != nil {
		t.Fatalf("nil skills: %v", err)
	}
	if out != nil {
		t.Fatalf("nil skills: expected nil map, got %v", out)
	}
}

func TestBuildCodeBackendSkillsFiles_PlaceholderWithoutFiles(t *testing.T) {
	out, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
		{Name: "pdf", Source: models.AgentSkillSourceBuiltin},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	body, ok := out["pdf/SKILL.md"]
	if !ok {
		t.Fatalf("placeholder SKILL.md missing, got keys %v", mapKeys(out))
	}
	if !strings.Contains(string(body), "name: pdf") || !strings.Contains(string(body), "builtin") {
		t.Fatalf("placeholder content unexpected:\n%s", body)
	}
}

func TestBuildCodeBackendSkillsFiles_TreeFromConfigFiles(t *testing.T) {
	out, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
		{
			Name:   "deploy-check",
			Source: models.AgentSkillSourcePath,
			Config: map[string]any{
				"files": map[string]any{
					"SKILL.md":          "---\nname: deploy-check\n---\nRun the script.",
					"scripts/check.py":  "#!/usr/bin/env python3\nprint('ok')\n",
					"reference/spec.md": "# spec",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{
		"deploy-check/SKILL.md",
		"deploy-check/scripts/check.py",
		"deploy-check/reference/spec.md",
	}
	for _, k := range want {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %q, got keys %v", k, mapKeys(out))
		}
	}
	if got := string(out["deploy-check/scripts/check.py"]); !strings.Contains(got, "print('ok')") {
		t.Errorf("script content lost: %q", got)
	}
}

func TestBuildCodeBackendSkillsFiles_RequiresSkillMD(t *testing.T) {
	_, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
		{
			Name:   "broken",
			Source: models.AgentSkillSourcePath,
			Config: map[string]any{
				"files": map[string]any{"scripts/x.sh": "echo hi"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("expected SKILL.md-required error, got %v", err)
	}
}

func TestBuildCodeBackendSkillsFiles_RejectsTraversalAndBadPaths(t *testing.T) {
	bad := []string{
		"../evil.md",
		"scripts/../../evil.md",
		"/etc/passwd",
		"~/x",
		".hidden/SKILL.md",
		"scripts\\win.cmd",
		"a//b.md",
	}
	for _, rel := range bad {
		_, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
			{
				Name:   "sk",
				Source: models.AgentSkillSourcePath,
				Config: map[string]any{
					"files": map[string]any{"SKILL.md": "ok", rel: "evil"},
				},
			},
		})
		if err == nil {
			t.Errorf("path %q: expected error, got nil", rel)
		}
	}
}

func TestBuildCodeBackendSkillsFiles_RejectsNonStringContent(t *testing.T) {
	_, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
		{
			Name:   "sk",
			Source: models.AgentSkillSourcePath,
			Config: map[string]any{
				"files": map[string]any{"SKILL.md": 42},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "string") {
		t.Fatalf("expected non-string content error, got %v", err)
	}
}

func TestBuildCodeBackendSkillsFiles_RejectsOversizedFile(t *testing.T) {
	_, err := buildCodeBackendSkillsFiles([]AgentSkillRef{
		{
			Name:   "sk",
			Source: models.AgentSkillSourcePath,
			Config: map[string]any{
				"files": map[string]any{"SKILL.md": strings.Repeat("a", maxSkillFileBytes+1)},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too-large error, got %v", err)
	}
}

// Wiring: оба билдера claude-семейства кладут дерево в BackendArtifacts.SkillsFiles,
// а BackendArtifactsToSandboxBundle прокидывает его в sandbox-бандл.
func TestClaudeFamilyBuilders_PopulateSkillsFiles(t *testing.T) {
	settings, err := json.Marshal(map[string]any{
		"skills": []map[string]any{
			{
				"name":   "deploy-check",
				"source": "path",
				"config": map[string]any{
					"files": map[string]any{
						"SKILL.md":         "---\nname: deploy-check\n---\n",
						"scripts/check.sh": "echo ok",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	builders := map[string]ArtifactBuilder{
		"claude":      NewClaudeArtifactBuilder(),
		"antigravity": NewAntigravityArtifactBuilder(),
	}
	for name, b := range builders {
		cb := b.Backend()
		agent := &models.Agent{CodeBackend: &cb, CodeBackendSettings: settings}
		out, err := b.Build(context.Background(), agent, &models.Project{}, ArtifactBuilderDeps{})
		if err != nil {
			t.Fatalf("%s: Build: %v", name, err)
		}
		if _, ok := out.SkillsFiles["deploy-check/scripts/check.sh"]; !ok {
			t.Fatalf("%s: SkillsFiles missing script, got %v", name, mapKeys(out.SkillsFiles))
		}
		bundle := BackendArtifactsToSandboxBundle(out)
		if bundle == nil || len(bundle.SkillsFiles) != 2 {
			t.Fatalf("%s: bundle SkillsFiles not propagated: %+v", name, bundle)
		}
	}
}

// Sprint 22 — hermes на том же контракте: config.files → дерево в ~/.hermes/skills/.
func TestHermesBuilder_SkillsFilesTree(t *testing.T) {
	settings, err := json.Marshal(map[string]any{
		"hermes": map[string]any{
			"toolsets":        []string{"file_ops", "shell"},
			"permission_mode": "yolo",
			"skills": []map[string]any{
				{
					"name":   "deploy-check",
					"source": "agentskills",
					"config": map[string]any{
						"files": map[string]any{
							"SKILL.md":         "---\nname: deploy-check\ndescription: check deploys\n---\n",
							"scripts/check.sh": "echo ok",
						},
					},
				},
				{"name": "bare", "source": "builtin"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cb := models.CodeBackendHermes
	agent := &models.Agent{CodeBackend: &cb, CodeBackendSettings: settings}
	out, err := NewHermesArtifactBuilder().Build(context.Background(), agent, &models.Project{}, ArtifactBuilderDeps{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := string(out.HermesSkills["deploy-check/scripts/check.sh"]); got != "echo ok" {
		t.Fatalf("script content = %q; keys: %v", got, mapKeys(out.HermesSkills))
	}
	if !strings.Contains(string(out.HermesSkills["deploy-check/SKILL.md"]), "check deploys") {
		t.Fatalf("SKILL.md content lost")
	}
	// skill без files — placeholder, как раньше.
	if _, ok := out.HermesSkills["bare/SKILL.md"]; !ok {
		t.Fatalf("placeholder for bare skill missing")
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_SKILLS"]; got != "deploy-check,bare" {
		t.Fatalf("skills env = %q", got)
	}
}

func TestValidateCodeBackendSettings_HermesSkillFiles(t *testing.T) {
	ok := `{"hermes":{"skills":[{"name":"x","source":"builtin","config":{"files":{"SKILL.md":"ok","scripts/a.sh":"echo"}}}]}}`
	if err := validateCodeBackendSettingsStrict([]byte(ok)); err != nil {
		t.Fatalf("valid hermes skill files rejected: %v", err)
	}
	bad := []string{
		`{"hermes":{"skills":[{"name":"x","source":"builtin","config":{"files":{"scripts/a.sh":"echo"}}}]}}`, // нет SKILL.md
		`{"hermes":{"skills":[{"name":"x","source":"builtin","config":{"files":{"SKILL.md":"ok","../e":"x"}}}]}}`, // traversal
		`{"hermes":{"skills":[{"name":"x","source":"builtin","config":{"files":"str"}}]}}`, // не объект
	}
	for _, c := range bad {
		if err := validateCodeBackendSettingsStrict([]byte(c)); err == nil {
			t.Errorf("expected rejection: %s", c)
		}
	}
}

func mapKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
