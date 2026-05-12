// Package service / Sprint 15.21 — генерация артефактов настроек агента для sandbox-контейнера.
//
// Артефакты:
//   1) ~/.claude/settings.json — permissions + env + hooks (Claude Code CLI).
//   2) ~/.claude/.mcp.json    — MCP-серверы (из таблицы mcp_servers_registry + agent bindings).
//   3) ~/.claude/skills/      — список Skills (имена/пути; реальные файлы кладутся отдельно).
//
// Service не пишет файлы напрямую (это работа sandbox runner'а — он копирует JSON через CopyToContainer).
// Здесь — только сборка структурированного содержимого.
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/devteam/backend/internal/models"
)

// AgentSettingsArtifacts — то, что sandbox runner должен положить в контейнер агента.
type AgentSettingsArtifacts struct {
	// SettingsJSON — содержимое ~/.claude/settings.json.
	SettingsJSON []byte
	// MCPJSON — содержимое ~/.claude/.mcp.json (только если есть MCP-серверы; иначе nil).
	MCPJSON []byte
	// Skills — Skills, которые должны быть смонтированы / подгружены в контейнер.
	Skills []AgentSkillArtifact
	// PermissionMode — значение для CLI флага `--permission-mode` (acceptEdits | plan | bypassPermissions).
	// Подставляется в entrypoint.sh при запуске claude.
	PermissionMode string
}

// AgentSkillArtifact — описание skill для sandbox runner'а.
// Source указывает, откуда взять содержимое skill (builtin/plugin/path).
type AgentSkillArtifact struct {
	Name   string
	Source models.AgentSkillSource
	Config map[string]any
}

// SandboxPermissions — JSON-структура из Agent.SandboxPermissions.
// Поля совпадают с форматом settings.json.permissions Claude Code CLI.
type SandboxPermissions struct {
	Allow       []string `json:"allow,omitempty"`
	Deny        []string `json:"deny,omitempty"`
	Ask         []string `json:"ask,omitempty"`
	DefaultMode string   `json:"defaultMode,omitempty"`
}

// IsValidPermissionMode проверяет допустимость mode.
func IsValidPermissionMode(mode string) bool {
	switch mode {
	case "", "default", "acceptEdits", "plan", "bypassPermissions":
		return true
	default:
		return false
	}
}

// AllowPattern — допустимые паттерны (claude-code allowed tools):
//   Read | Edit | Write | Glob | Grep | LS | NotebookEdit | WebFetch | TodoWrite
//   Bash(<glob>:*) | mcp__<server>__<tool> | mcp__<server>
//   (плюс служебные allow-all варианты вида Bash, WebSearch)
// Чтобы не дублировать каталог инструментов CLI, валидируем только базовый формат
// и небольшой whitelist префиксов — достаточно для защиты от мусора в БД.
var allowedToolBaseTokens = map[string]struct{}{
	"Read": {}, "Edit": {}, "Write": {}, "Glob": {}, "Grep": {}, "LS": {},
	"Bash": {}, "WebFetch": {}, "WebSearch": {}, "NotebookEdit": {}, "TodoWrite": {},
}

var (
	bashPattern = regexp.MustCompile(`^Bash\((?:[^)]+)\)$`)
	mcpPattern  = regexp.MustCompile(`^mcp__[a-zA-Z0-9_-]+(?:__[a-zA-Z0-9_-]+)?$`)
)

// ValidateAllowPattern проверяет, что строка похожа на легитимный allow/deny pattern Claude Code CLI.
// Возвращает ошибку с самим pattern для понятных сообщений.
func ValidateAllowPattern(pattern string) error {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return fmt.Errorf("empty permission pattern")
	}
	if _, ok := allowedToolBaseTokens[p]; ok {
		return nil
	}
	if bashPattern.MatchString(p) || mcpPattern.MatchString(p) {
		return nil
	}
	return fmt.Errorf("unsupported permission pattern: %q", p)
}

// ValidateSandboxPermissions проверяет все поля Permission-объекта.
func ValidateSandboxPermissions(p SandboxPermissions) error {
	if !IsValidPermissionMode(p.DefaultMode) {
		return fmt.Errorf("invalid permissions.defaultMode: %q", p.DefaultMode)
	}
	for _, pat := range p.Allow {
		if err := ValidateAllowPattern(pat); err != nil {
			return fmt.Errorf("permissions.allow: %w", err)
		}
	}
	for _, pat := range p.Deny {
		if err := ValidateAllowPattern(pat); err != nil {
			return fmt.Errorf("permissions.deny: %w", err)
		}
	}
	for _, pat := range p.Ask {
		if err := ValidateAllowPattern(pat); err != nil {
			return fmt.Errorf("permissions.ask: %w", err)
		}
	}
	return nil
}

// AgentCodeBackendSettings — JSON-структура из Agent.CodeBackendSettings.
// Содержит per-agent параметры code-backend (model override, MCP-сервера, Skills, env, hooks).
type AgentCodeBackendSettings struct {
	Model      string                 `json:"model,omitempty"`
	MCPServers []AgentMCPServerRef    `json:"mcp_servers,omitempty"`
	Skills     []AgentSkillRef        `json:"skills,omitempty"`
	Env        map[string]string      `json:"env,omitempty"`
	Hooks      map[string]any         `json:"hooks,omitempty"`
}

// AgentMCPServerRef — ссылка на MCP-сервер (по имени из mcp_servers_registry).
type AgentMCPServerRef struct {
	Name     string            `json:"name"`
	Env      map[string]string `json:"env,omitempty"`
}

// AgentSkillRef — ссылка на skill (имя + source + опциональный config).
type AgentSkillRef struct {
	Name   string                 `json:"name"`
	Source models.AgentSkillSource `json:"source"`
	Config map[string]any          `json:"config,omitempty"`
}

// AgentSettingsService — собирает артефакты настроек для конкретного агента.
type AgentSettingsService interface {
	BuildArtifacts(agent *models.Agent, registry MCPRegistryLookup) (*AgentSettingsArtifacts, error)
}

// MCPRegistryLookup — резолвит MCP-серверы по имени для подстановки в .mcp.json.
// Реализуется поверх mcp_servers_registry-репозитория; передаётся в BuildArtifacts.
type MCPRegistryLookup interface {
	LookupMCPServer(name string) (*models.MCPServerRegistry, bool)
}

type agentSettingsService struct{}

// NewAgentSettingsService собирает сервис без зависимостей.
func NewAgentSettingsService() AgentSettingsService {
	return &agentSettingsService{}
}

// BuildArtifacts собирает settings.json + .mcp.json + список Skills для агента.
func (s *agentSettingsService) BuildArtifacts(agent *models.Agent, registry MCPRegistryLookup) (*AgentSettingsArtifacts, error) {
	if agent == nil {
		return nil, errors.New("agent is nil")
	}

	perms, err := decodeSandboxPermissions(agent.SandboxPermissions)
	if err != nil {
		return nil, fmt.Errorf("sandbox_permissions: %w", err)
	}
	if err := ValidateSandboxPermissions(perms); err != nil {
		return nil, err
	}

	codeSettings, err := decodeCodeBackendSettings(agent.CodeBackendSettings)
	if err != nil {
		return nil, fmt.Errorf("code_backend_settings: %w", err)
	}

	settingsJSON, err := buildSettingsJSON(perms, codeSettings)
	if err != nil {
		return nil, err
	}

	mcpJSON, err := buildMCPJSON(codeSettings, registry)
	if err != nil {
		return nil, err
	}

	skills := make([]AgentSkillArtifact, 0, len(codeSettings.Skills))
	for _, sk := range codeSettings.Skills {
		if !sk.Source.IsValid() {
			return nil, fmt.Errorf("invalid skill source for %q: %q", sk.Name, sk.Source)
		}
		skills = append(skills, AgentSkillArtifact{
			Name:   sk.Name,
			Source: sk.Source,
			Config: sk.Config,
		})
	}

	return &AgentSettingsArtifacts{
		SettingsJSON:   settingsJSON,
		MCPJSON:        mcpJSON,
		Skills:         skills,
		PermissionMode: perms.DefaultMode,
	}, nil
}

func decodeSandboxPermissions(raw []byte) (SandboxPermissions, error) {
	if len(raw) == 0 {
		return SandboxPermissions{}, nil
	}
	var p SandboxPermissions
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	return p, nil
}

func decodeCodeBackendSettings(raw []byte) (AgentCodeBackendSettings, error) {
	if len(raw) == 0 {
		return AgentCodeBackendSettings{}, nil
	}
	var s AgentCodeBackendSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, err
	}
	return s, nil
}

// settingsFile — корневой формат ~/.claude/settings.json (минимальный набор полей Claude Code CLI).
type settingsFile struct {
	Permissions *settingsPermissions `json:"permissions,omitempty"`
	Env         map[string]string    `json:"env,omitempty"`
	Hooks       map[string]any       `json:"hooks,omitempty"`
}

type settingsPermissions struct {
	Allow       []string `json:"allow,omitempty"`
	Deny        []string `json:"deny,omitempty"`
	Ask         []string `json:"ask,omitempty"`
	DefaultMode string   `json:"defaultMode,omitempty"`
}

func buildSettingsJSON(perms SandboxPermissions, codeSettings AgentCodeBackendSettings) ([]byte, error) {
	f := settingsFile{Env: codeSettings.Env, Hooks: codeSettings.Hooks}
	if len(perms.Allow)+len(perms.Deny)+len(perms.Ask) > 0 || perms.DefaultMode != "" {
		f.Permissions = &settingsPermissions{
			Allow:       perms.Allow,
			Deny:        perms.Deny,
			Ask:         perms.Ask,
			DefaultMode: perms.DefaultMode,
		}
	}
	return json.MarshalIndent(f, "", "  ")
}

// mcpFile — формат ~/.claude/.mcp.json (Claude Code CLI читает его при старте).
type mcpFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

func buildMCPJSON(settings AgentCodeBackendSettings, registry MCPRegistryLookup) ([]byte, error) {
	if len(settings.MCPServers) == 0 {
		return nil, nil
	}
	if registry == nil {
		return nil, errors.New("mcp servers configured but registry lookup is not provided")
	}
	f := mcpFile{MCPServers: map[string]mcpServerEntry{}}
	for _, ref := range settings.MCPServers {
		srv, ok := registry.LookupMCPServer(ref.Name)
		if !ok {
			return nil, fmt.Errorf("mcp server %q not found in registry", ref.Name)
		}
		if !srv.Transport.IsValid() {
			return nil, fmt.Errorf("mcp server %q: invalid transport %q", ref.Name, srv.Transport)
		}
		var args []string
		if len(srv.Args) > 0 {
			if err := json.Unmarshal(srv.Args, &args); err != nil {
				return nil, fmt.Errorf("mcp server %q: parse args: %w", ref.Name, err)
			}
		}
		envMap := map[string]string{}
		if len(srv.EnvTemplate) > 0 {
			tmpl := map[string]string{}
			if err := json.Unmarshal(srv.EnvTemplate, &tmpl); err != nil {
				return nil, fmt.Errorf("mcp server %q: parse env_template: %w", ref.Name, err)
			}
			for k, v := range tmpl {
				envMap[k] = v
			}
		}
		for k, v := range ref.Env { // overrides win
			envMap[k] = v
		}
		f.MCPServers[ref.Name] = mcpServerEntry{
			Transport: string(srv.Transport),
			Command:   srv.Command,
			Args:      args,
			URL:       srv.URL,
			Env:       envMap,
		}
	}
	return json.MarshalIndent(f, "", "  ")
}
