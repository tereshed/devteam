package dto

import (
	"encoding/json"

	"github.com/devteam/backend/internal/models"
)

// Sprint 15.Major (rawOrEmpty unify) — общая map-функция для REST и MCP-tools.
// До этого жили две копии rawOrEmpty/rawOrEmptyObject, обе возвращали `{}`, но шанс
// разойтись был высок. Здесь — единственный источник правды.

// AgentSettingsResponseFromModel — DRY-маппинг models.Agent → AgentSettingsResponse.
func AgentSettingsResponseFromModel(a *models.Agent) AgentSettingsResponse {
	resp := AgentSettingsResponse{
		AgentID:             a.ID,
		LLMProviderID:       a.LLMProviderID,
		CodeBackendSettings: jsonObjectOrEmpty(a.CodeBackendSettings),
		SandboxPermissions:  jsonObjectOrEmpty(a.SandboxPermissions),
	}
	if a.CodeBackend != nil {
		s := string(*a.CodeBackend)
		resp.CodeBackend = &s
	}
	return resp
}

// jsonObjectOrEmpty — пустой ввод → `{}`; иначе пробрасываем raw.
// Sprint 15 — оба JSON-поля (code_backend_settings / sandbox_permissions) объекты,
// поэтому консистентно отдаём `{}` (не `null`/`[]`).
func jsonObjectOrEmpty(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}
