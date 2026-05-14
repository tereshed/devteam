// Package service / Sprint 16.C — ClaudeArtifactBuilder.
//
// Извлечена из AgentSettingsService.BuildArtifacts (Sprint 15.22): сборка
// ~/.claude/settings.json + .mcp.json + список Skills + permission_mode.
// Реализует ArtifactBuilder и регистрируется в ArtifactBuilderRegistry — из
// AgentSettingsService.BuildArtifacts больше нет хардкодного if'а под claude-code.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
)

// ClaudeArtifactBuilder — реализация ArtifactBuilder для CodeBackendClaudeCode.
type ClaudeArtifactBuilder struct{}

// NewClaudeArtifactBuilder — конструктор; зависимостей нет (всё через ArtifactBuilderDeps).
func NewClaudeArtifactBuilder() *ClaudeArtifactBuilder { return &ClaudeArtifactBuilder{} }

// Backend — claude-code.
func (b *ClaudeArtifactBuilder) Backend() models.CodeBackend { return models.CodeBackendClaudeCode }

// Build — собирает артефакты Claude по агенту.
//
// Делегирует общим хелперам decodeSandboxPermissions/decodeCodeBackendSettings,
// buildSettingsJSON и buildMCPJSON, которые остались в agent_settings_service.go
// (используются в legacy-пути и из тестов). Skills — конвертируются из
// AgentSkillRef → AgentSkillArtifact.
//
// Если deps.MCPRegistry == nil, а у агента есть MCP-ссылки — ошибка
// (тот же контракт, что был в исходном BuildArtifacts).
//
// project пока не используется (Claude .mcp.json env-template не поддерживает
// ${secret:NAME}-шаблоны), но принимается ради единого контракта ArtifactBuilder.
// Когда добавим резолв секретов и для Claude — у нас уже будет «якорь» владельца.
func (b *ClaudeArtifactBuilder) Build(_ context.Context, agent *models.Agent, _ *models.Project, deps ArtifactBuilderDeps) (*BackendArtifacts, error) {
	if agent == nil {
		return nil, errors.New("claude builder: agent is nil")
	}

	perms, err := decodeSandboxPermissions(agent.SandboxPermissions)
	if err != nil {
		return nil, fmt.Errorf("claude builder: sandbox_permissions: %w", err)
	}
	if err := ValidateSandboxPermissions(perms); err != nil {
		return nil, fmt.Errorf("claude builder: %w", err)
	}

	codeSettings, err := decodeCodeBackendSettings(agent.CodeBackendSettings)
	if err != nil {
		return nil, fmt.Errorf("claude builder: code_backend_settings: %w", err)
	}

	settingsJSON, err := buildSettingsJSON(perms, codeSettings)
	if err != nil {
		return nil, fmt.Errorf("claude builder: settings.json: %w", err)
	}

	mcpJSON, err := buildMCPJSON(codeSettings, deps.MCPRegistry)
	if err != nil {
		return nil, fmt.Errorf("claude builder: mcp.json: %w", err)
	}

	skills := make([]AgentSkillArtifact, 0, len(codeSettings.Skills))
	for _, sk := range codeSettings.Skills {
		if !sk.Source.IsValid() {
			return nil, fmt.Errorf("claude builder: invalid skill source for %q: %q", sk.Name, sk.Source)
		}
		skills = append(skills, AgentSkillArtifact{
			Name:   sk.Name,
			Source: sk.Source,
			Config: sk.Config,
		})
	}

	return &BackendArtifacts{
		SettingsJSON:   settingsJSON,
		MCPJSON:        mcpJSON,
		Skills:         skills,
		PermissionMode: perms.DefaultMode,
	}, nil
}
