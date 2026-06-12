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

	ErrTeamAgentNotFound            = errors.New("agent not found")
	ErrTeamAgentInvalidModel        = errors.New("invalid model")
	ErrTeamAgentInvalidCodeBackend  = errors.New("invalid code_backend")
	ErrTeamAgentInvalidProviderKind = errors.New("invalid provider_kind")
	ErrTeamAgentConflict            = errors.New("agent update conflict")
	ErrTeamAgentInvalidToolBindings = errors.New("invalid or inactive tool_definition_id in tool_bindings")
	ErrTeamAgentRoleImmutable       = errors.New("cannot change role of a system agent")
	ErrTeamAgentInvalidRole         = errors.New("invalid role: must be a custom (non-system) snake_case role")

	// Sprint 15.B (B4): ownership-check для /agents/:id/settings и MCP-инструментов agent_settings_*.
	ErrTeamAgentAccessDenied = errors.New("agent does not belong to current user's project")

	ErrTeamTypeAlreadyExists       = errors.New("team of this type already exists in the project")
	ErrTeamCannotDeleteDevelopment = errors.New("cannot delete the development team")
	ErrTeamTypeInvalid             = errors.New("invalid team type")
	ErrTeamTypeInUse               = errors.New("cannot delete team type that is currently in use")
	ErrTeamTypeCannotDeleteSystem   = errors.New("cannot delete system team type")
)

// TeamService минимальная бизнес-обёртка над TeamRepository.
type TeamService interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.Team, error)
	Create(ctx context.Context, projectID uuid.UUID, req dto.CreateTeamRequest) (*models.Team, error)
	Delete(ctx context.Context, projectID, teamID uuid.UUID) error
	Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error)
	CreateAgent(ctx context.Context, projectID uuid.UUID, teamID uuid.UUID, req dto.CreateTeamAgentRequest) (*models.Agent, error)
	DeleteAgent(ctx context.Context, projectID, agentID uuid.UUID) error
	PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error)
	// Sprint 15.23 — per-agent settings (code_backend_settings + sandbox_permissions).
	// Sprint 15.e2e: llm_provider_id удалён, kind вынесен в agent.provider_kind (PATCH /team/agents/:id).
	// Sprint 15.B (B4): актёр (userID, isAdmin) обязательно проверяется на ownership через
	// agent → team → project.user_id. Admin (isAdmin=true) пропускает проверку.
	GetAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID) (*models.Agent, error)
	UpdateAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID, req dto.UpdateAgentSettingsRequest) (*models.Agent, error)

	ListTeamTypes(ctx context.Context) ([]models.TeamTypeModel, error)
	CreateTeamType(ctx context.Context, req dto.CreateTeamTypeRequest) (*models.TeamTypeModel, error)
	DeleteTeamType(ctx context.Context, code string) error
}

// AgentSettingsActor — кто делает запрос. Sprint 15.B (B4).
type AgentSettingsActor struct {
	UserID  uuid.UUID
	IsAdmin bool
}

type teamService struct {
	teamRepo    repository.TeamRepository
	toolDefRepo repository.ToolDefinitionRepository
	agentSvc    *AgentService
	txManager   repository.TransactionManager
}

// NewTeamService создаёт сервис команд.
func NewTeamService(teamRepo repository.TeamRepository, toolDefRepo repository.ToolDefinitionRepository) TeamService {
	return &teamService{teamRepo: teamRepo, toolDefRepo: toolDefRepo}
}

// WithAgentServiceForTeam sets the AgentService on TeamService.
func WithAgentServiceForTeam(svc TeamService, agentSvc *AgentService) TeamService {
	if ts, ok := svc.(*teamService); ok {
		ts.agentSvc = agentSvc
	}
	return svc
}

// WithTransactionManager sets the TransactionManager on TeamService.
func WithTransactionManager(svc TeamService, tx repository.TransactionManager) TeamService {
	if ts, ok := svc.(*teamService); ok {
		ts.txManager = tx
	}
	return svc
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

func (s *teamService) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.Team, error) {
	return s.teamRepo.ListByProjectID(ctx, projectID)
}

func (s *teamService) Create(ctx context.Context, projectID uuid.UUID, req dto.CreateTeamRequest) (*models.Team, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrTeamInvalidName
	}
	tt := models.TeamType(req.Type)
	if _, err := s.teamRepo.GetTeamTypeByCode(ctx, string(tt)); err != nil {
		return nil, ErrTeamTypeInvalid
	}

	// Проверяем, существует ли уже команда такого типа в проекте
	existing, err := s.teamRepo.ListByProjectID(ctx, projectID)
	if err == nil {
		for _, t := range existing {
			if t.Type == tt {
				return nil, ErrTeamTypeAlreadyExists
			}
		}
	}

	team := &models.Team{
		ProjectID: projectID,
		Name:      name,
		Type:      tt,
	}

	runCreate := func(txCtx context.Context) error {
		if err := s.teamRepo.Create(txCtx, team); err != nil {
			return err
		}
		if s.agentSvc != nil {
			if err := s.agentSvc.CreateDefaultProjectAgents(txCtx, team.ID, string(team.Type)); err != nil {
				return err
			}
		}
		return nil
	}

	if s.txManager != nil {
		if err := s.txManager.WithTransaction(ctx, runCreate); err != nil {
			return nil, err
		}
	} else {
		if err := runCreate(ctx); err != nil {
			return nil, err
		}
	}

	// Возвращаем созданную команду
	return s.teamRepo.GetByID(ctx, team.ID)
}

func (s *teamService) Delete(ctx context.Context, projectID, teamID uuid.UUID) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return ErrTeamNotFound
		}
		return err
	}

	if team.ProjectID != projectID {
		return ErrTeamNotFound
	}

	if team.Type == models.TeamTypeDevelopment {
		return ErrTeamCannotDeleteDevelopment
	}

	runDelete := func(txCtx context.Context) error {
		// Сначала удаляем всех агентов команды
		for _, a := range team.Agents {
			if s.agentSvc != nil {
				if err := s.agentSvc.Delete(txCtx, a.ID); err != nil {
					return err
				}
			}
		}
		// Затем удаляем саму команду
		return s.teamRepo.Delete(txCtx, teamID)
	}

	if s.txManager != nil {
		return s.txManager.WithTransaction(ctx, runDelete)
	}
	return runDelete(ctx)
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

func (s *teamService) CreateAgent(ctx context.Context, projectID uuid.UUID, teamID uuid.UUID, req dto.CreateTeamAgentRequest) (*models.Agent, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	// Verify projectID matches
	if team.ProjectID != projectID {
		return nil, ErrTeamNotFound
	}
	
	in := CreateAgentInput{
		Name:            req.Name,
		Role:            models.AgentRole(req.Role),
		ExecutionKind:   models.AgentExecutionKind(req.ExecutionKind),
		RoleDescription: req.RoleDescription,
		SystemPrompt:    req.SystemPrompt,
		TeamID:          &teamID,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
	}
	if req.Model != nil && *req.Model != "" {
		in.Model = req.Model
	}
	if req.ProviderKind != nil && *req.ProviderKind != "" {
		pk := models.AgentProviderKind(*req.ProviderKind)
		in.ProviderKind = &pk
	}
	if req.CodeBackend != nil && *req.CodeBackend != "" {
		cb := models.CodeBackend(*req.CodeBackend)
		in.CodeBackend = &cb
	}
	
	if s.agentSvc == nil {
		return nil, fmt.Errorf("AgentService is not configured")
	}
	
	return s.agentSvc.Create(ctx, in)
}

// DeleteAgent удаляет агента команды. Принадлежность агента проекту проверяется
// через GetAgentInProject (тот же гард, что у PatchAgent), чтобы нельзя было удалить
// чужого агента по прямому ID.
func (s *teamService) DeleteAgent(ctx context.Context, projectID, agentID uuid.UUID) error {
	if _, err := s.teamRepo.GetAgentInProject(ctx, projectID, agentID); err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return ErrTeamAgentNotFound
		}
		return err
	}
	if s.agentSvc == nil {
		return fmt.Errorf("AgentService is not configured")
	}
	return s.agentSvc.Delete(ctx, agentID)
}

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

	if req.CodeBackendPresent() {
		if req.CodeBackendClear() {
			agent.CodeBackend = nil
			agent.ExecutionKind = models.AgentExecutionKindLLM
			// transition model from settings JSON to column if needed
			var settings AgentCodeBackendSettings
			if len(agent.CodeBackendSettings) > 0 {
				_ = json.Unmarshal(agent.CodeBackendSettings, &settings)
			}
			if settings.Model != "" {
				m := settings.Model
				agent.Model = &m
				settings.Model = ""
				bytes, _ := json.Marshal(settings)
				agent.CodeBackendSettings = bytes
			}
		} else if v, ok := req.CodeBackendValue(); ok {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				agent.CodeBackend = nil
				agent.ExecutionKind = models.AgentExecutionKindLLM
				// transition model from settings JSON to column if needed
				var settings AgentCodeBackendSettings
				if len(agent.CodeBackendSettings) > 0 {
					_ = json.Unmarshal(agent.CodeBackendSettings, &settings)
				}
				if settings.Model != "" {
					m := settings.Model
					agent.Model = &m
					settings.Model = ""
					bytes, _ := json.Marshal(settings)
					agent.CodeBackendSettings = bytes
				}
			} else {
				cb := models.CodeBackend(trimmed)
				if !cb.IsValid() {
					return nil, ErrTeamAgentInvalidCodeBackend
				}
				agent.CodeBackend = &cb
				agent.ExecutionKind = models.AgentExecutionKindSandbox
				// transition model from column to settings JSON if needed
				if agent.Model != nil {
					var settings AgentCodeBackendSettings
					if len(agent.CodeBackendSettings) > 0 {
						_ = json.Unmarshal(agent.CodeBackendSettings, &settings)
					}
					settings.Model = *agent.Model
					bytes, _ := json.Marshal(settings)
					agent.CodeBackendSettings = bytes
					agent.Model = nil
				}
			}
		}
	}

	if req.ModelPresent() {
		if agent.ExecutionKind == models.AgentExecutionKindSandbox {
			var settings AgentCodeBackendSettings
			if len(agent.CodeBackendSettings) > 0 {
				if err := json.Unmarshal(agent.CodeBackendSettings, &settings); err != nil {
					return nil, err
				}
			}
			if req.ModelClear() {
				settings.Model = ""
			} else if v, ok := req.ModelValue(); ok {
				trimmed := strings.TrimSpace(v)
				if len(trimmed) > maxAgentModelLen {
					return nil, ErrTeamAgentInvalidModel
				}
				settings.Model = trimmed
			}
			settingsBytes, err := json.Marshal(settings)
			if err != nil {
				return nil, err
			}
			agent.CodeBackendSettings = settingsBytes
			agent.Model = nil
		} else {
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

	if req.SystemPromptPresent() {
		if req.SystemPromptClear() {
			agent.SystemPrompt = nil
		} else if v, ok := req.SystemPromptValue(); ok {
			agent.SystemPrompt = &v
		}
	}

	if req.RoleDescriptionPresent() {
		if req.RoleDescriptionClear() {
			agent.RoleDescription = nil
		} else if v, ok := req.RoleDescriptionValue(); ok {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				agent.RoleDescription = nil
			} else {
				agent.RoleDescription = &trimmed
			}
		}
	}

	if req.RolePresent() {
		if v, ok := req.RoleValue(); ok {
			// Менять роль можно только у кастомных (не-системных) агентов, и только на
			// другую кастомную роль: на системных ролях завязана механика оркестрации
			// (branch-policy, дефолтные агенты, исключение assistant из каталога Router'а).
			if agent.Role.IsSystem() {
				return nil, ErrTeamAgentRoleImmutable
			}
			newRole := models.AgentRole(strings.TrimSpace(v))
			if !newRole.IsValid() || newRole.IsSystem() {
				return nil, ErrTeamAgentInvalidRole
			}
			agent.Role = newRole
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
		if agent.TeamID != nil && *agent.TeamID != uuid.Nil {
			return s.teamRepo.GetByID(ctx, *agent.TeamID)
		}
		return s.teamRepo.GetByProjectID(ctx, projectID)
	}

	if err := s.teamRepo.SaveAgent(ctx, agent); err != nil {
		if mapped, ok := mapAgentPatchPostgresFK(err); ok {
			return nil, mapped
		}
		return nil, err
	}

	if agent.TeamID != nil && *agent.TeamID != uuid.Nil {
		return s.teamRepo.GetByID(ctx, *agent.TeamID)
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
			a.ExecutionKind = models.AgentExecutionKindLLM
			// transition model from settings JSON to column
			var settings AgentCodeBackendSettings
			if len(a.CodeBackendSettings) > 0 {
				_ = json.Unmarshal(a.CodeBackendSettings, &settings)
			}
			if settings.Model != "" {
				m := settings.Model
				a.Model = &m
				settings.Model = ""
				bytes, _ := json.Marshal(settings)
				a.CodeBackendSettings = bytes
			}
		} else {
			cb := models.CodeBackend(trimmed)
			if !cb.IsValid() {
				return nil, ErrTeamAgentInvalidCodeBackend
			}
			a.CodeBackend = &cb
			a.ExecutionKind = models.AgentExecutionKindSandbox
			// transition model from column to settings JSON
			if a.Model != nil {
				var settings AgentCodeBackendSettings
				if len(a.CodeBackendSettings) > 0 {
					_ = json.Unmarshal(a.CodeBackendSettings, &settings)
				}
				settings.Model = *a.Model
				bytes, _ := json.Marshal(settings)
				a.CodeBackendSettings = bytes
				a.Model = nil
			}
		}
	}

	if len(req.CodeBackendSettings) > 0 {
		// Проверим, что это валидный JSON-объект.
		if !isJSONObject(req.CodeBackendSettings) {
			return nil, fmt.Errorf("code_backend_settings must be a JSON object")
		}
		// If transitioning/transitioned to sandbox, and model is not set in incoming settings, copy it from previous settings.
		if a.ExecutionKind == models.AgentExecutionKindSandbox {
			var newSettings AgentCodeBackendSettings
			if err := json.Unmarshal(req.CodeBackendSettings, &newSettings); err == nil && newSettings.Model == "" {
				var currentSettings AgentCodeBackendSettings
				if len(a.CodeBackendSettings) > 0 {
					_ = json.Unmarshal(a.CodeBackendSettings, &currentSettings)
				}
				if currentSettings.Model != "" {
					newSettings.Model = currentSettings.Model
					req.CodeBackendSettings, _ = json.Marshal(newSettings)
				}
			}
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
	// HTTP header name (RFC token, упрощённо): буквы/цифры/дефис — Authorization, X-Api-Key и т.п.
	codeBackendHeaderNameRE = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
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
		// Инлайн-определение (type/url/headers заданы прямо у агента): валидируем transport
		// и имена заголовков (HTTP-токены, не UPPER_SNAKE). Значения headers могут содержать
		// ${secret:NAME} — резолвятся при сборке .mcp.json.
		if ref.Type != "" {
			switch ref.Type {
			case "stdio", "sse", "http":
			default:
				return fmt.Errorf("mcp_servers[%d].type: invalid %q (allowed: stdio|sse|http)", i, ref.Type)
			}
		}
		for k := range ref.Headers {
			if !codeBackendHeaderNameRE.MatchString(k) {
				return fmt.Errorf("mcp_servers[%d].headers[%q]: invalid HTTP header name", i, k)
			}
		}
	}
	skillsTotalBytes := 0
	for i, sk := range parsed.Skills {
		if !codeBackendSkillNameRE.MatchString(sk.Name) {
			return fmt.Errorf("skills[%d].name must match [a-zA-Z][a-zA-Z0-9_-]*, got %q", i, sk.Name)
		}
		if !sk.Source.IsValid() {
			return fmt.Errorf("skills[%d].source invalid: %q", i, sk.Source)
		}
		// Sprint 22 — skills с контентом (config.files): валидируем форму и пути
		// на сохранении, чтобы агент не падал на сборке артефактов в рантайме.
		n, err := validateSkillConfigFiles(sk.Config)
		if err != nil {
			return fmt.Errorf("skills[%d] (%s): %w", i, sk.Name, err)
		}
		skillsTotalBytes += n
		if skillsTotalBytes > maxSkillsTotalBytes {
			return fmt.Errorf("skills: total content size exceeds %d bytes", maxSkillsTotalBytes)
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
	// Sprint 16.C — отдельная валидация hermes-секции (toolsets/permission_mode/skills/mcp_servers).
	// Permission_mode∈{plan,default} → 400 (защита от interactive prompt в headless-контейнере).
	if parsed.Hermes != nil {
		if err := validateHermesSection(parsed.Hermes); err != nil {
			return fmt.Errorf("hermes: %w", err)
		}
	}
	// Sprint 15.Major recursive DisallowUnknownFields: проверяем raw JSON на extra-keys
	// в mcp_servers[] и skills[] (Go json.Decoder.DisallowUnknownFields рекурсивен только
	// при использовании конкретных struct'ур, но Env-карты и hooks отображены в map[string]any,
	// где extra-ключи проходят — поэтому верифицируем top-level отдельно).
	return validateCodeBackendNestedKeys(raw)
}

// validateHermesSection — Sprint 16.C: проверки полей AgentCodeBackendSettings.Hermes.
//
// Жёсткое правило: для Hermes допустимы только permission_mode "yolo" и "accept";
// "plan" и "default" отклоняются (interactive prompt в headless-контейнере → hang/timeout).
// Никакой тихой подмены на yolo — пользователь должен исправить настройки явно.
func validateHermesSection(h *HermesAgentSettings) error {
	if h.PermissionMode != "" {
		switch h.PermissionMode {
		case "yolo", "accept":
			// ok
		case "plan", "default":
			return fmt.Errorf(
				"permission_mode=%q is not allowed for hermes in DevTeam (headless sandbox); "+
					"use \"yolo\" or \"accept\"", h.PermissionMode)
		default:
			return fmt.Errorf("permission_mode: unknown value %q (allowed: yolo|accept)", h.PermissionMode)
		}
	}
	if h.MaxTurns < 0 || h.MaxTurns > 200 {
		return fmt.Errorf("max_turns: out of range 0..200, got %d", h.MaxTurns)
	}
	if h.Temperature != nil {
		t := *h.Temperature
		if t < 0 || t > 2 {
			return fmt.Errorf("temperature: out of range 0..2, got %v", t)
		}
	}
	for i, ts := range h.Toolsets {
		if !codeBackendSkillNameRE.MatchString(ts) {
			return fmt.Errorf("toolsets[%d]: invalid name %q (must match [a-zA-Z][a-zA-Z0-9_-]*)", i, ts)
		}
	}
	hermesSkillsTotalBytes := 0
	for i, sk := range h.Skills {
		if !codeBackendSkillNameRE.MatchString(sk.Name) {
			return fmt.Errorf("skills[%d].name: invalid %q", i, sk.Name)
		}
		switch sk.Source {
		case "builtin", "agentskills", "path":
			// ok
		default:
			return fmt.Errorf("skills[%d].source: invalid %q (allowed: builtin|agentskills|path)", i, sk.Source)
		}
		// Sprint 22 — config.files: тот же контракт, что у claude-семейства
		// (дерево файлов skill'а, копируется в ~/.hermes/skills/<name>/).
		n, err := validateSkillConfigFiles(sk.Config)
		if err != nil {
			return fmt.Errorf("skills[%d] (%s): %w", i, sk.Name, err)
		}
		hermesSkillsTotalBytes += n
		if hermesSkillsTotalBytes > maxSkillsTotalBytes {
			return fmt.Errorf("skills: total content size exceeds %d bytes", maxSkillsTotalBytes)
		}
	}
	for i, m := range h.MCPServers {
		if !codeBackendMCPNameRE.MatchString(m.Name) {
			return fmt.Errorf("mcp_servers[%d].name: invalid %q", i, m.Name)
		}
		switch m.Transport {
		case "stdio", "http":
			// ok
		default:
			return fmt.Errorf("mcp_servers[%d].transport: invalid %q (allowed: stdio|http)", i, m.Transport)
		}
		for k := range m.Env {
			if !codeBackendEnvKeyRE.MatchString(k) {
				return fmt.Errorf("mcp_servers[%d].env[%q]: key must be UPPER_SNAKE_CASE", i, k)
			}
		}
	}
	return nil
}

// claudeCodeHookNameRE — белый список имён хуков Claude Code CLI.
// Список расширяется по мере необходимости; неизвестное имя считается ошибкой.
// Имена должны соответствовать docs.claude.com (PreToolUse, PostToolUse, Notification и т.п.).
var claudeCodeHookNameRE = regexp.MustCompile(
	`^(PreToolUse|PostToolUse|Notification|Stop|SubagentStop|UserPromptSubmit)$`)

// validateCodeBackendNestedKeys — Sprint 15.Major recursive DisallowUnknownFields.
// Парсим в map[string]any и явно whitelist'им ключи MCPServerRef/AgentSkillRef.
var allowedMCPServerRefKeys = map[string]struct{}{
	"name": {}, "env": {},
	// Инлайн-определение MCP-сервера на уровне агента команды.
	"type": {}, "url": {}, "command": {}, "args": {}, "headers": {},
}
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

func (s *teamService) ListTeamTypes(ctx context.Context) ([]models.TeamTypeModel, error) {
	return s.teamRepo.ListTeamTypes(ctx)
}

func (s *teamService) CreateTeamType(ctx context.Context, req dto.CreateTeamTypeRequest) (*models.TeamTypeModel, error) {
	code := strings.TrimSpace(strings.ToLower(req.Code))
	if code == "" {
		return nil, ErrTeamTypeInvalid
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrTeamTypeInvalid
	}

	// Check if already exists
	if _, err := s.teamRepo.GetTeamTypeByCode(ctx, code); err == nil {
		return nil, ErrTeamTypeAlreadyExists
	}

	tt := &models.TeamTypeModel{
		Code:     code,
		Name:     name,
		IsSystem: false,
	}

	if err := s.teamRepo.CreateTeamType(ctx, tt); err != nil {
		return nil, err
	}

	return tt, nil
}

func (s *teamService) DeleteTeamType(ctx context.Context, code string) error {
	code = strings.TrimSpace(strings.ToLower(code))
	if code == "" {
		return ErrTeamTypeInvalid
	}

	// Check if system
	tt, err := s.teamRepo.GetTeamTypeByCode(ctx, code)
	if err != nil {
		return ErrTeamTypeInvalid
	}
	if tt.IsSystem {
		return ErrTeamTypeCannotDeleteSystem
	}

	// Check if in use
	count, err := s.teamRepo.CountTeamsByType(ctx, code)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrTeamTypeInUse
	}

	return s.teamRepo.DeleteTeamType(ctx, code)
}
