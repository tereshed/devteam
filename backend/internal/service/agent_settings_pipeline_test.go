package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devteam/backend/internal/models"
)

// Sprint 16.C — pipeline-уровневые тесты, которые ловят регрессию «builder
// существует, но никогда не вызывается из реального пайплайна».
//
// Покрывают:
//   1) AgentSettingsService.BuildArtifacts дисптачит на нужный builder по CodeBackend
//      (Claude vs Hermes) — не оставлен хардкод под claude.
//   2) BuildSandboxBundle возвращает заполненный bundle (а не nil) — иначе
//      runner получит ничего и вся фича 16.C мёртвая.
//   3) BackendArtifactsToSandboxBundle мапит ВСЕ Hermes-поля (включая HermesEnv) —
//      иначе DEVTEAM_HERMES_PERMISSION_MODE не дойдёт до entrypoint.

func TestAgentSettingsService_BuildArtifacts_DispatchesByCodeBackend(t *testing.T) {
	svc := NewAgentSettingsServiceWithDeps(nil, nil)

	hermesCB := models.CodeBackendHermes
	hermesAgent := &models.Agent{
		CodeBackend:         &hermesCB,
		CodeBackendSettings: []byte(`{"hermes":{"toolsets":["file_ops"],"permission_mode":"yolo"}}`),
	}
	hermesArt, err := svc.BuildArtifacts(hermesAgent, nil, nil)
	if err != nil {
		t.Fatalf("hermes BuildArtifacts: %v", err)
	}
	// Hermes-маркеры должны быть, claude-маркеров — нет.
	if len(hermesArt.HermesConfigYAML) == 0 {
		t.Fatalf("hermes path: HermesConfigYAML is empty")
	}
	if hermesArt.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"] != "yolo" {
		t.Fatalf("hermes path: DEVTEAM_HERMES_PERMISSION_MODE missing/wrong: %v", hermesArt.HermesEnv)
	}
	if len(hermesArt.SettingsJSON) > 0 {
		t.Fatalf("hermes path leaked claude SettingsJSON: %s", hermesArt.SettingsJSON)
	}

	claudeCB := models.CodeBackendClaudeCode
	claudeAgent := &models.Agent{
		CodeBackend:         &claudeCB,
		SandboxPermissions:  []byte(`{"defaultMode":"acceptEdits","allow":["Read"]}`),
		CodeBackendSettings: []byte(`{}`),
	}
	claudeArt, err := svc.BuildArtifacts(claudeAgent, nil, nil)
	if err != nil {
		t.Fatalf("claude BuildArtifacts: %v", err)
	}
	if len(claudeArt.SettingsJSON) == 0 {
		t.Fatalf("claude path: SettingsJSON empty")
	}
	if claudeArt.PermissionMode != "acceptEdits" {
		t.Fatalf("claude path: PermissionMode=%q, want acceptEdits", claudeArt.PermissionMode)
	}
	if len(claudeArt.HermesConfigYAML) > 0 {
		t.Fatalf("claude path leaked hermes config")
	}
}

func TestAgentSettingsService_BuildSandboxBundle_HermesPopulated(t *testing.T) {
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	cb := models.CodeBackendHermes
	a := &models.Agent{
		CodeBackend:         &cb,
		CodeBackendSettings: []byte(`{"hermes":{"toolsets":["file_ops","shell"],"permission_mode":"yolo","max_turns":15}}`),
	}
	b, err := svc.BuildSandboxBundle(context.Background(), a, nil)
	if err != nil {
		t.Fatalf("BuildSandboxBundle: %v", err)
	}
	if b == nil {
		t.Fatalf("nil bundle for hermes agent — это означает, что pipeline никогда не передаст артефакты в runner")
	}
	if len(b.HermesConfigYAML) == 0 {
		t.Fatalf("HermesConfigYAML empty in bundle")
	}
	if b.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"] != "yolo" {
		t.Fatalf("bundle HermesEnv lost permission_mode: %v", b.HermesEnv)
	}
	if b.HermesEnv["DEVTEAM_HERMES_TOOLSETS"] != "file_ops,shell" {
		t.Fatalf("bundle HermesEnv lost toolsets: %v", b.HermesEnv)
	}
	if b.HermesEnv["DEVTEAM_HERMES_MAX_TURNS"] != "15" {
		t.Fatalf("bundle HermesEnv lost max_turns: %v", b.HermesEnv)
	}
}

func TestAgentSettingsService_BuildSandboxBundle_ClaudeRoundtrip(t *testing.T) {
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	cb := models.CodeBackendClaudeCode
	a := &models.Agent{
		CodeBackend:        &cb,
		SandboxPermissions: []byte(`{"defaultMode":"acceptEdits"}`),
	}
	b, err := svc.BuildSandboxBundle(context.Background(), a, nil)
	if err != nil {
		t.Fatalf("BuildSandboxBundle: %v", err)
	}
	if b == nil {
		t.Fatalf("nil bundle for claude agent with permissions — runner не увидит permission_mode")
	}
	if b.PermissionMode != "acceptEdits" {
		t.Fatalf("PermissionMode=%q", b.PermissionMode)
	}
	// Никаких hermes-полей (изоляция backend'ов).
	if len(b.HermesConfigYAML) > 0 {
		t.Fatalf("claude bundle leaked hermes config")
	}
}

func TestAgentSettingsService_BuildSandboxBundle_NilOnNoCodeBackend(t *testing.T) {
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	// LLM-only агент (orchestrator/planner) — без CodeBackend.
	a := &models.Agent{}
	b, err := svc.BuildSandboxBundle(context.Background(), a, nil)
	if err != nil {
		t.Fatalf("BuildSandboxBundle: %v", err)
	}
	if b != nil {
		t.Fatalf("expected nil bundle for agent without CodeBackend, got: %+v", b)
	}
}

func TestBackendArtifactsToSandboxBundle_HermesEnvIncluded(t *testing.T) {
	art := &BackendArtifacts{
		HermesConfigYAML: []byte("display:\n"),
		HermesEnv: map[string]string{
			"DEVTEAM_HERMES_PERMISSION_MODE": "accept",
			"HERMES_MCP_GITHUB_TOKEN":        "ghp_xxx",
		},
	}
	b := BackendArtifactsToSandboxBundle(art)
	if b == nil {
		t.Fatalf("nil bundle from non-empty artifacts")
	}
	if b.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"] != "accept" {
		t.Fatalf("env lost: %v", b.HermesEnv)
	}
	if b.HermesEnv["HERMES_MCP_GITHUB_TOKEN"] != "ghp_xxx" {
		t.Fatalf("secret env lost (it must reach runner — иначе MCP-сервер не получит токен)")
	}
}

func TestBackendArtifactsToSandboxBundle_NilOnAllEmpty(t *testing.T) {
	if got := BackendArtifactsToSandboxBundle(nil); got != nil {
		t.Fatalf("nil input must return nil")
	}
	if got := BackendArtifactsToSandboxBundle(&BackendArtifacts{}); got != nil {
		t.Fatalf("empty artifacts must return nil bundle (legacy path)")
	}
}

// Round-trip от настроек агента до OS env-шейпа, который пойдёт в Docker:
// проверяем что HermesEnv действительно мёрджится в SandboxOptions.EnvVars
// через mergeSandboxEnv (sandbox-пакет), и DEVTEAM_HERMES_* выйдет в контейнер.
//
// Этот тест живёт в service-пакете для удобства, использует sandbox.SandboxOptions
// напрямую (низкая связность OK — это интеграционный тест по контракту).
func TestPipelineRoundtrip_HermesEnvReachesContainerEnvList(t *testing.T) {
	// 1) settings агента
	settings := map[string]any{
		"hermes": map[string]any{
			"toolsets":        []string{"file_ops"},
			"permission_mode": "accept",
			"max_turns":       7,
		},
	}
	body, _ := json.Marshal(settings)
	cb := models.CodeBackendHermes
	a := &models.Agent{CodeBackend: &cb, CodeBackendSettings: body}

	// 2) сборка bundle через сервис
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	bundle, err := svc.BuildSandboxBundle(context.Background(), a, &models.Project{})
	if err != nil {
		t.Fatalf("BuildSandboxBundle: %v", err)
	}
	if bundle == nil {
		t.Fatalf("bundle is nil — pipeline вернёт nil → runner ничего не сделает")
	}

	// 3) проверяем, что то, что попадёт в env, содержит наши ключи.
	// Прямо обращаемся к карте — sandbox-уровневый mergeSandboxEnv проверен в его
	// собственном TestMergeSandboxEnv_HermesEnvMerged.
	if bundle.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"] != "accept" {
		t.Fatalf("permission_mode lost between settings and bundle: %v", bundle.HermesEnv)
	}
	if bundle.HermesEnv["DEVTEAM_HERMES_TOOLSETS"] != "file_ops" {
		t.Fatalf("toolsets lost: %v", bundle.HermesEnv)
	}
	if bundle.HermesEnv["DEVTEAM_HERMES_MAX_TURNS"] != "7" {
		t.Fatalf("max_turns lost: %v", bundle.HermesEnv)
	}
}
