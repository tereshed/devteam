package service

import (
	"strings"
	"testing"
)

// Sprint 16.C — validateCodeBackendSettingsStrict должен принять полный валидный
// hermes-payload и отклонить permission_mode∈{plan,default}.

func TestValidateCodeBackendSettings_HermesValidPayload(t *testing.T) {
	raw := `{
		"hermes": {
			"toolsets": ["file_ops", "shell"],
			"permission_mode": "yolo",
			"max_turns": 10,
			"temperature": 0.2,
			"skills": [{"name":"code-review","source":"builtin"}],
			"mcp_servers": [{"name":"github","transport":"stdio","command":"npx","env":{"GITHUB_TOKEN":"$x"}}]
		}
	}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err != nil {
		t.Fatalf("expected ok, got: %v", err)
	}
}

func TestValidateCodeBackendSettings_HermesRejectsPlan(t *testing.T) {
	raw := `{"hermes":{"permission_mode":"plan"}}`
	err := validateCodeBackendSettingsStrict([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "permission_mode") {
		t.Fatalf("expected permission_mode rejection, got: %v", err)
	}
}

func TestValidateCodeBackendSettings_HermesRejectsDefault(t *testing.T) {
	raw := `{"hermes":{"permission_mode":"default"}}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err == nil {
		t.Fatalf("expected rejection for permission_mode=default")
	}
}

func TestValidateCodeBackendSettings_HermesRejectsBadTransport(t *testing.T) {
	raw := `{"hermes":{"mcp_servers":[{"name":"foo","transport":"weird"}]}}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err == nil {
		t.Fatalf("expected rejection for unknown transport")
	}
}

func TestValidateCodeBackendSettings_HermesRejectsTempOutOfRange(t *testing.T) {
	raw := `{"hermes":{"temperature":3.5}}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err == nil {
		t.Fatalf("expected rejection for temperature>2")
	}
}

func TestValidateCodeBackendSettings_HermesRejectsBadToolsetName(t *testing.T) {
	raw := `{"hermes":{"toolsets":["bad name with space"]}}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err == nil {
		t.Fatalf("expected rejection for invalid toolset name")
	}
}

func TestValidateCodeBackendSettings_HermesRejectsBadSkillSource(t *testing.T) {
	raw := `{"hermes":{"skills":[{"name":"foo","source":"unknown"}]}}`
	if err := validateCodeBackendSettingsStrict([]byte(raw)); err == nil {
		t.Fatalf("expected rejection for invalid skill source")
	}
}
