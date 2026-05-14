package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
)

type fakeSecretResolver struct {
	secrets map[string]string
}

func (f *fakeSecretResolver) Resolve(_ context.Context, _ *models.Project, name string) (string, error) {
	v, ok := f.secrets[name]
	if !ok {
		return "", ErrSecretNotFound
	}
	return v, nil
}

func newAgentWithHermes(t *testing.T, payload map[string]any) *models.Agent {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cb := models.CodeBackendHermes
	return &models.Agent{
		CodeBackend:         &cb,
		CodeBackendSettings: body,
	}
}

func TestHermesArtifactBuilder_DefaultsOnNil(t *testing.T) {
	b := NewHermesArtifactBuilder()
	cb := models.CodeBackendHermes
	a := &models.Agent{CodeBackend: &cb} // CodeBackendSettings == nil
	out, err := b.Build(context.Background(), a, &models.Project{}, ArtifactBuilderDeps{})
	if err != nil {
		t.Fatalf("Build with nil settings: %v", err)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"]; got != "yolo" {
		t.Fatalf("permission_mode default = %q, want yolo", got)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_TOOLSETS"]; got != "file_ops,shell" {
		t.Fatalf("toolsets default = %q, want file_ops,shell", got)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_MAX_TURNS"]; got != "12" {
		t.Fatalf("max_turns default = %q, want 12", got)
	}
	if !strings.Contains(string(out.HermesConfigYAML), "file_ops") {
		t.Fatalf("config.yaml missing default toolset:\n%s", out.HermesConfigYAML)
	}
}

func TestHermesArtifactBuilder_FullPayload(t *testing.T) {
	b := NewHermesArtifactBuilder()
	temp := 0.1
	payload := map[string]any{
		"hermes": map[string]any{
			"toolsets":        []string{"file_ops", "web_fetch"},
			"permission_mode": "accept",
			"max_turns":       20,
			"temperature":     temp,
			"skills": []map[string]string{
				{"name": "code-review-checklist", "source": "builtin"},
			},
			"mcp_servers": []map[string]any{
				{
					"name":      "github",
					"transport": "stdio",
					"command":   "npx",
					"args":      []string{"-y", "@modelcontextprotocol/server-github"},
					"env":       map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "${secret:github_pat}"},
				},
			},
		},
	}
	a := newAgentWithHermes(t, payload)
	resolver := &fakeSecretResolver{secrets: map[string]string{"github_pat": "ghp_secret_xyz"}}
	out, err := b.Build(context.Background(), a, &models.Project{}, ArtifactBuilderDeps{SecretResolver: resolver})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"]; got != "accept" {
		t.Fatalf("permission_mode = %q", got)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_TOOLSETS"]; got != "file_ops,web_fetch" {
		t.Fatalf("toolsets = %q", got)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_MAX_TURNS"]; got != "20" {
		t.Fatalf("max_turns = %q", got)
	}
	if got := out.HermesEnv["DEVTEAM_HERMES_SKILLS"]; got != "code-review-checklist" {
		t.Fatalf("skills env = %q", got)
	}
	// secret должен быть в env, а в mcp.json — ссылка на env-имя.
	envName := "HERMES_MCP_GITHUB_GITHUB_PERSONAL_ACCESS_TOKEN"
	if got := out.HermesEnv[envName]; got != "ghp_secret_xyz" {
		t.Fatalf("secret env %s = %q, want ghp_secret_xyz", envName, got)
	}
	if !strings.Contains(string(out.HermesMCPJSON), "$"+envName) {
		t.Fatalf("mcp.json must reference env, got: %s", out.HermesMCPJSON)
	}
	if strings.Contains(string(out.HermesMCPJSON), "ghp_secret_xyz") {
		t.Fatalf("plaintext secret leaked into mcp.json:\n%s", out.HermesMCPJSON)
	}
	// skill-файл создан с безопасным путём
	if _, ok := out.HermesSkills["code-review-checklist/SKILL.md"]; !ok {
		t.Fatalf("skill file missing; got: %#v", out.HermesSkills)
	}
}

func TestHermesArtifactBuilder_RejectsPlanMode(t *testing.T) {
	b := NewHermesArtifactBuilder()
	payload := map[string]any{
		"hermes": map[string]any{"permission_mode": "plan"},
	}
	a := newAgentWithHermes(t, payload)
	if _, err := b.Build(context.Background(), a, &models.Project{}, ArtifactBuilderDeps{}); err == nil {
		t.Fatalf("expected error on permission_mode=plan, got nil")
	}
}

func TestHermesArtifactBuilder_SecretMissingResolver(t *testing.T) {
	b := NewHermesArtifactBuilder()
	payload := map[string]any{
		"hermes": map[string]any{
			"mcp_servers": []map[string]any{
				{
					"name":      "github",
					"transport": "stdio",
					"command":   "npx",
					"env":       map[string]string{"X": "${secret:foo}"},
				},
			},
		},
	}
	a := newAgentWithHermes(t, payload)
	_, err := b.Build(context.Background(), a, &models.Project{}, ArtifactBuilderDeps{})
	if err == nil || !strings.Contains(err.Error(), "secret resolver") {
		t.Fatalf("expected resolver error, got %v", err)
	}
}

// Sprint 16.C-2 — если в env есть ${secret:NAME}, но project не передан
// (нет «якоря владельца»), билдер должен отказать ДО вызова резолвера.
// Иначе резолвер не сможет понять, чьи user_llm_credentials читать → утечка
// между проектами.
func TestHermesArtifactBuilder_SecretRequiresProject(t *testing.T) {
	b := NewHermesArtifactBuilder()
	payload := map[string]any{
		"hermes": map[string]any{
			"mcp_servers": []map[string]any{
				{
					"name":      "github",
					"transport": "stdio",
					"command":   "npx",
					"env":       map[string]string{"X": "${secret:openrouter}"},
				},
			},
		},
	}
	a := newAgentWithHermes(t, payload)
	resolver := &fakeSecretResolver{secrets: map[string]string{"openrouter": "x"}}
	_, err := b.Build(context.Background(), a, nil, ArtifactBuilderDeps{SecretResolver: resolver})
	if err == nil || !strings.Contains(err.Error(), "project context required") {
		t.Fatalf("expected project-required error, got %v", err)
	}
}

func TestHermesArtifactBuilder_SecretNotFound(t *testing.T) {
	b := NewHermesArtifactBuilder()
	payload := map[string]any{
		"hermes": map[string]any{
			"mcp_servers": []map[string]any{
				{
					"name":      "github",
					"transport": "stdio",
					"command":   "npx",
					"env":       map[string]string{"X": "${secret:missing}"},
				},
			},
		},
	}
	a := newAgentWithHermes(t, payload)
	resolver := &fakeSecretResolver{}
	_, err := b.Build(context.Background(), a, &models.Project{}, ArtifactBuilderDeps{SecretResolver: resolver})
	if err == nil || !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestAssertSafeRelativePath(t *testing.T) {
	const base = "/home/sandbox/.hermes/skills"
	cases := map[string]bool{
		"foo/SKILL.md":         true,
		"foo/bar/SKILL.md":     true,
		"":                     false,
		"/etc/passwd":          false,
		"~/evil":               false,
		"../../etc/passwd":     false,
		"foo/../../../etc":     false,
		"foo/\x00":             false,
	}
	for rel, ok := range cases {
		err := assertSafeRelativePath(base, rel)
		if ok && err != nil {
			t.Errorf("%q: want OK, got %v", rel, err)
		}
		if !ok && err == nil {
			t.Errorf("%q: want error, got nil", rel)
		}
	}
}

func TestArtifactBuilderRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()
	r := NewArtifactBuilderRegistry()
	r.Register(NewHermesArtifactBuilder())
	r.Register(NewHermesArtifactBuilder())
}

func TestArtifactBuilderRegistry_GetByBackend(t *testing.T) {
	r := NewArtifactBuilderRegistry()
	r.Register(NewHermesArtifactBuilder())
	b, ok := r.Get(models.CodeBackendHermes)
	if !ok {
		t.Fatalf("registry missing hermes builder")
	}
	if b.Backend() != models.CodeBackendHermes {
		t.Fatalf("backend mismatch: %v", b.Backend())
	}
	if _, ok := r.Get(models.CodeBackendClaudeCode); ok {
		t.Fatalf("registry should not have claude builder")
	}
}
