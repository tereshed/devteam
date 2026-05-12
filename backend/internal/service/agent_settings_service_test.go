package service

import (
	"encoding/json"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// --- ValidateAllowPattern / ValidateSandboxPermissions ---

func TestValidateAllowPattern_Accepts(t *testing.T) {
	for _, p := range []string{
		"Read", "Edit", "Write", "Glob", "Grep", "LS", "TodoWrite", "WebFetch",
		"Bash(git diff:*)", "Bash(go test:*)",
		"mcp__github__create_issue", "mcp__weaviate",
	} {
		assert.NoError(t, ValidateAllowPattern(p), "pattern: %q", p)
	}
}

func TestValidateAllowPattern_Rejects(t *testing.T) {
	for _, p := range []string{
		"", "rm -rf /", "Bash(", "Bash(noclose",
		"mcp__bad-server@", "Read; rm",
	} {
		err := ValidateAllowPattern(p)
		assert.Error(t, err, "pattern: %q", p)
	}
}

func TestValidateSandboxPermissions_FullCheck(t *testing.T) {
	perms := SandboxPermissions{
		Allow:       []string{"Read", "Edit", "Bash(go test:*)"},
		Deny:        []string{"Bash(rm -rf:*)"},
		DefaultMode: "acceptEdits",
	}
	require.NoError(t, ValidateSandboxPermissions(perms))

	perms.DefaultMode = "wat"
	assert.Error(t, ValidateSandboxPermissions(perms))

	perms.DefaultMode = "acceptEdits"
	perms.Allow = []string{"DefinitelyNotATool"}
	assert.Error(t, ValidateSandboxPermissions(perms))
}

// --- BuildArtifacts ---

type fakeMCPRegistry struct {
	rows map[string]*models.MCPServerRegistry
}

func (f *fakeMCPRegistry) LookupMCPServer(name string) (*models.MCPServerRegistry, bool) {
	r, ok := f.rows[name]
	return r, ok
}

func TestBuildArtifacts_SettingsJSON(t *testing.T) {
	svc := NewAgentSettingsService()

	perms := SandboxPermissions{
		Allow:       []string{"Read", "Edit", "Bash(git diff:*)"},
		Deny:        []string{"Bash(rm -rf:*)"},
		DefaultMode: "acceptEdits",
	}
	permJSON, _ := json.Marshal(perms)

	settings := AgentCodeBackendSettings{
		Env: map[string]string{"FOO": "bar"},
	}
	settingsJSON, _ := json.Marshal(settings)

	agent := &models.Agent{
		ID:                  uuid.New(),
		SandboxPermissions:  datatypes.JSON(permJSON),
		CodeBackendSettings: datatypes.JSON(settingsJSON),
	}

	art, err := svc.BuildArtifacts(agent, nil)
	require.NoError(t, err)
	require.NotNil(t, art)
	assert.Equal(t, "acceptEdits", art.PermissionMode)
	assert.NotEmpty(t, art.SettingsJSON)

	var parsed struct {
		Permissions struct {
			Allow       []string `json:"allow"`
			Deny        []string `json:"deny"`
			DefaultMode string   `json:"defaultMode"`
		} `json:"permissions"`
		Env map[string]string `json:"env"`
	}
	require.NoError(t, json.Unmarshal(art.SettingsJSON, &parsed))
	assert.Equal(t, "acceptEdits", parsed.Permissions.DefaultMode)
	assert.Contains(t, parsed.Permissions.Allow, "Bash(git diff:*)")
	assert.Equal(t, "bar", parsed.Env["FOO"])
}

func TestBuildArtifacts_MCPJSON_ResolvesFromRegistry(t *testing.T) {
	svc := NewAgentSettingsService()

	settings := AgentCodeBackendSettings{
		MCPServers: []AgentMCPServerRef{{Name: "github", Env: map[string]string{"OVERRIDE": "1"}}},
	}
	settingsJSON, _ := json.Marshal(settings)
	agent := &models.Agent{ID: uuid.New(), CodeBackendSettings: datatypes.JSON(settingsJSON)}

	registry := &fakeMCPRegistry{rows: map[string]*models.MCPServerRegistry{
		"github": {
			Name:        "github",
			Transport:   models.MCPTransportStdio,
			Command:     "mcp-github",
			Args:        datatypes.JSON(`["--port","3001"]`),
			EnvTemplate: datatypes.JSON(`{"GITHUB_TOKEN":"$GITHUB_TOKEN","DEFAULT":"v"}`),
		},
	}}

	art, err := svc.BuildArtifacts(agent, registry)
	require.NoError(t, err)
	require.NotNil(t, art.MCPJSON)

	var parsed struct {
		MCPServers map[string]struct {
			Transport string            `json:"transport"`
			Command   string            `json:"command"`
			Args      []string          `json:"args"`
			Env       map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal(art.MCPJSON, &parsed))
	gh := parsed.MCPServers["github"]
	assert.Equal(t, "stdio", gh.Transport)
	assert.Equal(t, "mcp-github", gh.Command)
	assert.Equal(t, []string{"--port", "3001"}, gh.Args)
	// merge env_template + override
	assert.Equal(t, "v", gh.Env["DEFAULT"])
	assert.Equal(t, "1", gh.Env["OVERRIDE"])
}

func TestBuildArtifacts_RejectsUnknownMCPServer(t *testing.T) {
	svc := NewAgentSettingsService()
	settings := AgentCodeBackendSettings{
		MCPServers: []AgentMCPServerRef{{Name: "ghost"}},
	}
	settingsJSON, _ := json.Marshal(settings)
	agent := &models.Agent{ID: uuid.New(), CodeBackendSettings: datatypes.JSON(settingsJSON)}

	registry := &fakeMCPRegistry{rows: map[string]*models.MCPServerRegistry{}}
	_, err := svc.BuildArtifacts(agent, registry)
	assert.Error(t, err)
}

func TestBuildArtifacts_BypassPermissionMode(t *testing.T) {
	svc := NewAgentSettingsService()
	permsJSON, _ := json.Marshal(SandboxPermissions{DefaultMode: "bypassPermissions"})
	agent := &models.Agent{ID: uuid.New(), SandboxPermissions: datatypes.JSON(permsJSON)}
	art, err := svc.BuildArtifacts(agent, nil)
	require.NoError(t, err)
	assert.Equal(t, "bypassPermissions", art.PermissionMode)
}

func TestBuildArtifacts_InvalidPermissionsBlock(t *testing.T) {
	svc := NewAgentSettingsService()
	permsJSON, _ := json.Marshal(SandboxPermissions{
		DefaultMode: "acceptEdits",
		Allow:       []string{"NotARealTool"},
	})
	agent := &models.Agent{ID: uuid.New(), SandboxPermissions: datatypes.JSON(permsJSON)}
	_, err := svc.BuildArtifacts(agent, nil)
	assert.Error(t, err)
}
