// Package service / Sprint 21 — AntigravityArtifactBuilder.
//
// Реализует ArtifactBuilder и регистрируется в ArtifactBuilderRegistry — из
// AgentSettingsService.BuildArtifacts.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
)

// AntigravityArtifactBuilder — реализация ArtifactBuilder для CodeBackendAntigravity.
type AntigravityArtifactBuilder struct{}

// NewAntigravityArtifactBuilder — конструктор.
func NewAntigravityArtifactBuilder() *AntigravityArtifactBuilder { return &AntigravityArtifactBuilder{} }

// Backend — antigravity.
func (b *AntigravityArtifactBuilder) Backend() models.CodeBackend { return models.CodeBackendAntigravity }

// Build — собирает артефакты Antigravity по агенту.
//
// Как и для Claude Code, используется settings.json и mcp.json.
func (b *AntigravityArtifactBuilder) Build(_ context.Context, agent *models.Agent, _ *models.Project, deps ArtifactBuilderDeps) (*BackendArtifacts, error) {
	if agent == nil {
		return nil, errors.New("antigravity builder: agent is nil")
	}

	perms, err := decodeSandboxPermissions(agent.SandboxPermissions)
	if err != nil {
		return nil, fmt.Errorf("antigravity builder: sandbox_permissions: %w", err)
	}
	if err := ValidateSandboxPermissions(perms); err != nil {
		return nil, fmt.Errorf("antigravity builder: %w", err)
	}

	codeSettings, err := decodeCodeBackendSettings(agent.CodeBackendSettings)
	if err != nil {
		return nil, fmt.Errorf("antigravity builder: code_backend_settings: %w", err)
	}

	settingsJSON, err := buildSettingsJSON(perms, codeSettings)
	if err != nil {
		return nil, fmt.Errorf("antigravity builder: settings.json: %w", err)
	}

	mcpJSON, err := buildMCPJSON(codeSettings, deps.MCPRegistry)
	if err != nil {
		return nil, fmt.Errorf("antigravity builder: mcp.json: %w", err)
	}

	skills := make([]AgentSkillArtifact, 0, len(codeSettings.Skills))
	for _, sk := range codeSettings.Skills {
		if !sk.Source.IsValid() {
			return nil, fmt.Errorf("antigravity builder: invalid skill source for %q: %q", sk.Name, sk.Source)
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
