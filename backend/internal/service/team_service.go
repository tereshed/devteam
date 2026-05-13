package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrTeamNotFound    = errors.New("team not found")
	ErrTeamInvalidName = errors.New("team name cannot be empty")

	ErrTeamAgentNotFound           = errors.New("agent not found")
	ErrTeamAgentInvalidModel       = errors.New("invalid model")
	ErrTeamAgentInvalidCodeBackend = errors.New("invalid code_backend")
	ErrTeamAgentInvalidProviderKind = errors.New("invalid provider_kind")
	ErrTeamAgentConflict           = errors.New("agent update conflict")
	ErrTeamAgentInvalidToolBindings = errors.New("invalid or inactive tool_definition_id in tool_bindings")

	// Sprint 15.B (B4): ownership-check для /agents/:id/settings и MCP-инструментов agent_settings_*.
	ErrTeamAgentAccessDenied = errors.New("agent does not belong to current user's project")
)

// TeamService минимальная бизнес-обёртка над TeamRepository.
type TeamService interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error)
	PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error)
	// Sprint 15.23 — per-agent settings (code_backend_settings + sandbox_permissions).
	// Sprint 15.e2e: llm_provider_id удалён, kind вынесен в agent.provider_kind (PATCH /team/agents/:id).
	// Sprint 15.B (B4): актёр (userID, isAdmin) обязательно проверяется на ownership через
	// agent → team → project.user_id. Admin (isAdmin=true) пропускает проверку.
	GetAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID) (*models.Agent, error)
	UpdateAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID, req dto.UpdateAgentSettingsRequest) (*models.Agent, error)
}

// AgentSettingsActor — кто делает запрос. Sprint 15.B (B4).
type AgentSettingsActor struct {
	UserID  uuid.UUID
	IsAdmin bool
}

type teamService struct {
	teamRepo    repository.TeamRepository
	toolDefRepo repository.ToolDefinitionRepository
}

// NewTeamService создаёт сервис команд.
func NewTeamService(teamRepo repository.TeamRepository, toolDefRepo repository.ToolDefinitionRepository) TeamService {
	return &teamService{teamRepo: teamRepo, toolDefRepo: toolDefRepo}
}

func (s *teamService) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	team, err := s.teamRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	return team, nil
}

func (s *teamService) Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error) {
	team, err := s.teamRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return nil, ErrTeamInvalidName
		}
		team.Name = trimmed
	}
	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, err
	}
	return s.teamRepo.GetByProjectID(ctx, projectID)
}

const maxAgentModelLen = 128

const maxAgentToolBindings = 50

func dedupeSortedToolDefinitionIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}

func (s *teamService) PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error) {
	agent, err := s.teamRepo.GetAgentInProject(ctx, projectID, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, err
	}

	// Сначала валидация tool_bindings, затем мутация agent: при раннем return указатель agent
	// не отражает «частично применённый» PATCH в памяти (см. 13.3.1 / ревью).
	var bindingIDs []uuid.UUID
	var doReplaceBindings bool
	if req.ToolBindingsPresent() {
		doReplaceBindings = true
		bindingIDs = dedupeSortedToolDefinitionIDs(req.ToolBindingsRawIDs())
		if len(bindingIDs) > maxAgentToolBindings {
			return nil, ErrTeamAgentInvalidToolBindings
		}
		if len(bindingIDs) > 0 {
			n, err := s.toolDefRepo.CountActiveInIDs(ctx, bindingIDs)
			if err != nil {
				return nil, err
			}
			if int(n) != len(bindingIDs) {
				return nil, ErrTeamAgentInvalidToolBindings
			}
		}
	}

	if req.ModelPresent() {
		if req.ModelClear() {
			agent.Model = nil
		} else if v, ok := req.ModelValue(); ok {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				agent.Model = nil
			} else if len(trimmed) > maxAgentModelLen {
				return nil, ErrTeamAgentInvalidModel
			} else {
				agent.Model = &trimmed
			}
		}
	}

	if req.PromptIDPresent() {
		if req.PromptIDClear() {
			agent.PromptID = nil
			agent.Prompt = nil
		} else if id, ok := req.PromptIDValue(); ok {
			agent.PromptID = &id
			agent.Prompt = nil
		}
	}

	if req.CodeBackendPresent() {
		if req.CodeBackendClear() {
			agent.CodeBackend = nil
		} else if v, ok := req.CodeBackendValue(); ok {
			cb := models.CodeBackend(v)
			if !cb.IsValid() {
				return nil, ErrTeamAgentInvalidCodeBackend
			}
			agent.CodeBackend = &cb
		}
	}

	if req.ProviderKindPresent() {
		if req.ProviderKindClear() {
			agent.ProviderKind = nil
		} else if v, ok := req.ProviderKindValue(); ok {
			pk := models.AgentProviderKind(v)
			if !pk.IsValid() {
				return nil, ErrTeamAgentInvalidProviderKind
			}
			agent.ProviderKind = &pk
		}
	}

	if req.IsActivePresent() {
		if v, ok := req.IsActiveValue(); ok {
			agent.IsActive = v
		}
	}

	if doReplaceBindings {
		if err := s.teamRepo.SaveAgentWithToolBindings(ctx, agent, true, bindingIDs); err != nil {
			if mapped, ok := mapAgentPatchPostgresFK(err); ok {
				return nil, mapped
			}
			return nil, err
		}
		return s.teamRepo.GetByProjectID(ctx, projectID)
	}

	if err := s.teamRepo.SaveAgent(ctx, agent); err != nil {
		if mapped, ok := mapAgentPatchPostgresFK(err); ok {
			return nil, mapped
		}
		return nil, err
	}

	return s.teamRepo.GetByProjectID(ctx, projectID)
}

// GetAgentSettings возвращает агента целиком — handler выбирает нужные поля
// (code_backend, code_backend_settings, sandbox_permissions).
//
// Sprint 15.B (B4): проверяется, что актёр — admin или owner проекта команды агента.
func (s *teamService) GetAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID) (*models.Agent, error) {
	if err := s.assertAgentOwner(ctx, actor, agentID); err != nil {
		return nil, err
	}
	a, err := s.teamRepo.GetAgentByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, err
	}
	return a, nil
}

// UpdateAgentSettings применяет частичное обновление полей агента (Sprint 15.23).
// Sprint 15.B (B4): тот же ownership-check, что и в GetAgentSettings.
//
// Валидируется:
//   - sandbox_permissions через ValidateSandboxPermissions;
//   - code_backend через models.CodeBackend.IsValid;
//   - code_backend_settings — что это валидный JSON-объект (структура — ответственность UI и MCP-инструментов).
func (s *teamService) UpdateAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID, req dto.UpdateAgentSettingsRequest) (*models.Agent, error) {
	if err := s.assertAgentOwner(ctx, actor, agentID); err != nil {
		return nil, err
	}
	a, err := s.teamRepo.GetAgentByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, err
	}

	if req.CodeBackend != nil {
		trimmed := strings.TrimSpace(*req.CodeBackend)
		if trimmed == "" {
			a.CodeBackend = nil
		} else {
			cb := models.CodeBackend(trimmed)
			if !cb.IsValid() {
				return nil, ErrTeamAgentInvalidCodeBackend
			}
			a.CodeBackend = &cb
		}
	}

	if len(req.CodeBackendSettings) > 0 {
		// Проверим, что это валидный JSON-объект.
		if !isJSONObject(req.CodeBackendSettings) {
			return nil, fmt.Errorf("code_backend_settings must be a JSON object")
		}
		// Sprint 15.N4 (extends M1): строгая валидация — DisallowUnknownFields отсекает мусор,
		// плюс regex-проверки на model/MCP-имена/env-ключи. Без этого «{"shell":"/bin/bash"}»
		// сохранится и попадёт в settings.json sandbox-контейнера.
		if err := validateCodeBackendSettingsStrict(req.CodeBackendSettings); err != nil {
			return nil, fmt.Errorf("code_backend_settings: %w", err)
		}
		a.CodeBackendSettings = append([]byte(nil), req.CodeBackendSettings...)
	}

	if len(req.SandboxPermissions) > 0 {
		var perms SandboxPermissions
		if err := json.Unmarshal(req.SandboxPermissions, &perms); err != nil {
			return nil, fmt.Errorf("sandbox_permissions: invalid JSON: %w", err)
		}
		if err := ValidateSandboxPermissions(perms); err != nil {
			return nil, err
		}
		a.SandboxPermissions = append([]byte(nil), req.SandboxPermissions...)
	}

	if err := s.teamRepo.SaveAgent(ctx, a); err != nil {
		if mapped, ok := mapAgentPatchPostgresFK(err); ok {
			return nil, mapped
		}
		return nil, err
	}
	return a, nil
}

// isJSONObject проверяет, что входной payload — корневой JSON-объект (а не массив/литерал).
func isJSONObject(raw []byte) bool {
	trimmed := bytes.TrimLeft(raw, " \t\n\r")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// Sprint 15.N4 — regex'ы для validateCodeBackendSettingsStrict.
var (
	// model: "anthropic/claude-3.5-sonnet", "gpt-4o", "claude-haiku-4-5-20251001" — буквы/цифры/-/_/./
	// Sprint 15.minor: убираем @ — в реальных именах LLM-моделей не встречается.
	codeBackendModelRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_./\-]*$`)
	// MCP server name: соответствует MCPServerRegistry.Name pattern.
	codeBackendMCPNameRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	// env-ключ: shell-конвенция UPPER_SNAKE_CASE, без spec-символов и инъекций.
	codeBackendEnvKeyRE = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	// Skill name: буквы/цифры/-/_ (как у Claude Code skills).
	codeBackendSkillNameRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// validateCodeBackendSettingsStrict — Sprint 15.N4 + Major fixes.
// Decoder с DisallowUnknownFields ловит extra-ключи на верхнем уровне.
// Дополнительно проводим РЕКУРСИВНУЮ верификацию: парсим в map[string]any и сверяем
// набор ключей в подобъектах (mcp_servers[], skills[]) с whitelist'ом.
// Также белый список Hooks-имён (Claude Code CLI хуки выполняют shell — без whitelist'а
// PUT /agents/{id}/settings становится вектором инъекции).
func validateCodeBackendSettingsStrict(raw []byte) error {
	var parsed AgentCodeBackendSettings
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if parsed.Model != "" && !codeBackendModelRE.MatchString(parsed.Model) {
		return fmt.Errorf("model contains disallowed characters: %q", parsed.Model)
	}
	for i, ref := range parsed.MCPServers {
		if !codeBackendMCPNameRE.MatchString(ref.Name) {
			return fmt.Errorf("mcp_servers[%d].name must match [a-zA-Z][a-zA-Z0-9_-]*, got %q", i, ref.Name)
		}
		for k := range ref.Env {
			if !codeBackendEnvKeyRE.MatchString(k) {
				return fmt.Errorf("mcp_servers[%d].env[%q]: key must be UPPER_SNAKE_CASE", i, k)
			}
		}
	}
	for i, sk := range parsed.Skills {
		if !codeBackendSkillNameRE.MatchString(sk.Name) {
			return fmt.Errorf("skills[%d].name must match [a-zA-Z][a-zA-Z0-9_-]*, got %q", i, sk.Name)
		}
		if !sk.Source.IsValid() {
			return fmt.Errorf("skills[%d].source invalid: %q", i, sk.Source)
		}
	}
	for k, v := range parsed.Env {
		if !codeBackendEnvKeyRE.MatchString(k) {
			return fmt.Errorf("env[%q]: key must be UPPER_SNAKE_CASE", k)
		}
		// Sprint 15.minor: env-value не должен содержать newline / shell-meta —
		// иначе через ANTHROPIC_API_KEY=$(rm -rf) можно подсунуть команду.
		if strings.ContainsAny(v, "\n\r\x00") {
			return fmt.Errorf("env[%q]: value contains control characters", k)
		}
	}
	// Sprint 15.Major: hooks whitelist + Sprint 15.minor: value structure check.
	// Claude Code CLI поддерживает hooks (event-name → []{matcher, hooks: []{type, command}}).
	// Без проверки value — PUT /agents/{id}/settings → settings.json → произвольная shell-команда.
	for hookName, hookValue := range parsed.Hooks {
		if !claudeCodeHookNameRE.MatchString(hookName) {
			return fmt.Errorf("hooks[%q]: hook name not in allowlist", hookName)
		}
		// Sprint 15.minor: value должен быть JSON-массивом объектов с matcher/hooks-структурой,
		// а не raw shell-командой типа "echo pwned" или числом. Сейчас Claude Code CLI ожидает массив.
		if _, isArr := hookValue.([]any); !isArr {
			return fmt.Errorf("hooks[%q]: value must be array (Claude Code hooks schema)", hookName)
		}
	}
	// Sprint 15.Major recursive DisallowUnknownFields: проверяем raw JSON на extra-keys
	// в mcp_servers[] и skills[] (Go json.Decoder.DisallowUnknownFields рекурсивен только
	// при использовании конкретных struct'ур, но Env-карты и hooks отображены в map[string]any,
	// где extra-ключи проходят — поэтому верифицируем top-level отдельно).
	return validateCodeBackendNestedKeys(raw)
}

// claudeCodeHookNameRE — белый список имён хуков Claude Code CLI.
// Список расширяется по мере необходимости; неизвестное имя считается ошибкой.
// Имена должны соответствовать docs.claude.com (PreToolUse, PostToolUse, Notification и т.п.).
var claudeCodeHookNameRE = regexp.MustCompile(
	`^(PreToolUse|PostToolUse|Notification|Stop|SubagentStop|UserPromptSubmit)$`)

// validateCodeBackendNestedKeys — Sprint 15.Major recursive DisallowUnknownFields.
// Парсим в map[string]any и явно whitelist'им ключи MCPServerRef/AgentSkillRef.
var allowedMCPServerRefKeys = map[string]struct{}{"name": {}, "env": {}}
var allowedSkillRefKeys = map[string]struct{}{"name": {}, "source": {}, "config": {}}

func validateCodeBackendNestedKeys(raw []byte) error {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil // top-level уже отвалидировано через Decoder.
	}
	if arr, ok := top["mcp_servers"].([]any); ok {
		for i, e := range arr {
			obj, _ := e.(map[string]any)
			for k := range obj {
				if _, ok := allowedMCPServerRefKeys[k]; !ok {
					return fmt.Errorf("mcp_servers[%d]: unknown field %q", i, k)
				}
			}
		}
	}
	if arr, ok := top["skills"].([]any); ok {
		for i, e := range arr {
			obj, _ := e.(map[string]any)
			for k := range obj {
				if _, ok := allowedSkillRefKeys[k]; !ok {
					return fmt.Errorf("skills[%d]: unknown field %q", i, k)
				}
			}
		}
	}
	return nil
}

// assertAgentOwner — Sprint 15.B (B4): admin или owner проекта команды агента.
// Если агента нет — возвращаем ErrTeamAgentNotFound (не «access denied», чтобы не утекать существование чужого ID).
func (s *teamService) assertAgentOwner(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID) error {
	if actor.IsAdmin {
		return nil
	}
	if actor.UserID == uuid.Nil {
		return ErrTeamAgentAccessDenied
	}
	owner, err := s.teamRepo.GetAgentOwnerUserID(ctx, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return ErrTeamAgentNotFound
		}
		return err
	}
	if owner != actor.UserID {
		// Не утекаем «такого агента нет» vs «нет доступа» — оба → 404 на handler-уровне.
		return ErrTeamAgentNotFound
	}
	return nil
}
