package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Sprint 15.N4 — строгая валидация code_backend_settings на этапе Update.

func TestValidateCodeBackendSettingsStrict_RejectsUnknownFields(t *testing.T) {
	err := validateCodeBackendSettingsStrict([]byte(`{"shell":"/bin/bash"}`))
	assert.Error(t, err, "unknown field must be rejected by DisallowUnknownFields")
}

func TestValidateCodeBackendSettingsStrict_RejectsBadModel(t *testing.T) {
	cases := []string{
		`{"model":"; rm -rf"}`,
		`{"model":"$(id)"}`,
		`{"model":"foo bar"}`,
		`{"model":"--malicious-flag"}`,
	}
	for _, c := range cases {
		err := validateCodeBackendSettingsStrict([]byte(c))
		assert.Error(t, err, "must reject model: %s", c)
	}
}

func TestValidateCodeBackendSettingsStrict_AllowsLegitModel(t *testing.T) {
	for _, c := range []string{
		`{"model":"anthropic/claude-3.5-sonnet"}`,
		`{"model":"gpt-4o"}`,
		`{"model":"claude-haiku-4-5-20251001"}`,
	} {
		err := validateCodeBackendSettingsStrict([]byte(c))
		assert.NoError(t, err, "must accept: %s", c)
	}
}

func TestValidateCodeBackendSettingsStrict_RejectsBadEnvKey(t *testing.T) {
	cases := []string{
		`{"env":{"path":"x"}}`,             // lowercase
		`{"env":{"BAD-KEY":"x"}}`,           // dash
		`{"env":{"BAD KEY":"x"}}`,           // space
		`{"env":{"1NUMERIC":"x"}}`,          // starts with digit
	}
	for _, c := range cases {
		err := validateCodeBackendSettingsStrict([]byte(c))
		assert.Error(t, err, "must reject env: %s", c)
	}
}

func TestValidateCodeBackendSettingsStrict_RejectsBadMCPName(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"mcp_servers":[{"name":"bad name!","env":{}}]}`))
	assert.Error(t, err)
}

func TestValidateCodeBackendSettingsStrict_RejectsBadMCPEnvKey(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"mcp_servers":[{"name":"github","env":{"oauth_token":"x"}}]}`))
	assert.Error(t, err, "lowercase env key must be rejected")
}

func TestValidateCodeBackendSettingsStrict_RejectsBadSkillName(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"skills":[{"name":"; rm","source":"builtin"}]}`))
	assert.Error(t, err)
}

// Sprint 22 — skills[].config.files (дерево файлов skill'а).
func TestValidateCodeBackendSettingsStrict_AllowsSkillWithFiles(t *testing.T) {
	err := validateCodeBackendSettingsStrict([]byte(`{
        "skills":[{"name":"deploy-check","source":"path","config":{"files":{
            "SKILL.md":"---\nname: deploy-check\n---\n",
            "scripts/check.py":"print('ok')"
        }}}]
    }`))
	assert.NoError(t, err)
}

func TestValidateCodeBackendSettingsStrict_RejectsSkillFilesWithoutSkillMD(t *testing.T) {
	err := validateCodeBackendSettingsStrict([]byte(`{
        "skills":[{"name":"x","source":"path","config":{"files":{"scripts/a.sh":"echo"}}}]
    }`))
	assert.Error(t, err, "config.files без SKILL.md должен отклоняться")
}

func TestValidateCodeBackendSettingsStrict_RejectsSkillFileTraversal(t *testing.T) {
	for _, rel := range []string{"../evil", "/abs", "a/../../b", ".hidden"} {
		payload := `{"skills":[{"name":"x","source":"path","config":{"files":{` +
			`"SKILL.md":"ok","` + rel + `":"evil"}}}]}`
		err := validateCodeBackendSettingsStrict([]byte(payload))
		assert.Error(t, err, "путь %q должен отклоняться", rel)
	}
}

func TestValidateCodeBackendSettingsStrict_RejectsSkillFilesNonObject(t *testing.T) {
	err := validateCodeBackendSettingsStrict([]byte(`{
        "skills":[{"name":"x","source":"path","config":{"files":"not-an-object"}}]
    }`))
	assert.Error(t, err)
}

func TestValidateCodeBackendSettingsStrict_AllowsCompleteValidPayload(t *testing.T) {
	payload := `{
        "model":"anthropic/claude-3.5-sonnet",
        "mcp_servers":[{"name":"github","env":{"GITHUB_TOKEN":"placeholder"}}],
        "skills":[{"name":"pdf","source":"builtin","config":{}}],
        "env":{"FOO":"bar"}
    }`
	err := validateCodeBackendSettingsStrict([]byte(payload))
	assert.NoError(t, err)
}

// Sprint 15.Major — Hooks whitelist.
func TestValidateCodeBackendSettingsStrict_RejectsUnknownHook(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"hooks":{"NotARealHook":"echo pwned"}}`))
	assert.Error(t, err)
}

func TestValidateCodeBackendSettingsStrict_AllowsKnownHook(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[]}]}}`))
	assert.NoError(t, err)
}

// Sprint 15.minor — hook value должен быть массивом, не raw shell-команды/числа.
func TestValidateCodeBackendSettingsStrict_RejectsHookStringValue(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"hooks":{"PreToolUse":"echo pwned"}}`))
	assert.Error(t, err)
}

// Sprint 15.Major — recursive DisallowUnknownFields.
func TestValidateCodeBackendSettingsStrict_RejectsUnknownFieldInMCPRef(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"mcp_servers":[{"name":"x","extra":"y"}]}`))
	assert.Error(t, err, "extra field in mcp_servers[] must be rejected")
}

func TestValidateCodeBackendSettingsStrict_RejectsUnknownFieldInSkillRef(t *testing.T) {
	err := validateCodeBackendSettingsStrict(
		[]byte(`{"skills":[{"name":"x","source":"builtin","shell":"/bin/bash"}]}`))
	assert.Error(t, err, "extra field in skills[] must be rejected")
}
