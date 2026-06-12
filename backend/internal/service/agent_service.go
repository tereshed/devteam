package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// agent_service.go — Sprint 17 / Sprint 5 review fix #1 (layer violation).
//
// Бизнес-логика реестра агентов v2 (CRUD + секреты). MCP-инструменты и HTTP-хендлеры
// должны зависеть от ЭТОГО сервиса, не от repository напрямую. Здесь живут:
//   - валидация ExecutionKind / Model / CodeBackend mutual exclusivity (зеркалит CHECK chk_agents_kind_requirements)
//   - валидация диапазонов (Temperature 0..2, MaxTokens > 0)
//   - идемпотентность Set-Secret (delete-old + create-new)
//   - шифрование секретов через pkg/crypto + AAD = secret.id
//   - sentinel-ошибки уровня сервиса для маппинга в HTTP/MCP-ответ

// ─────────────────────────────────────────────────────────────────────────────
// Sentinel errors
// ─────────────────────────────────────────────────────────────────────────────

var (
	ErrAgentValidation        = errors.New("agent validation failed")
	ErrAgentNameAlreadyTaken  = errors.New("agent with this name already exists")
	ErrAgentNotInRegistry     = errors.New("agent not found in registry")
	ErrAgentSecretInvalidKey  = errors.New("invalid secret key_name (must match ^[A-Z][A-Z0-9_]{0,127}$)")
	ErrEncryptorNotConfigured = errors.New("encryptor is not configured (set ENCRYPTION_KEY)")
	// Sprint 5 review fix #2: optimistic concurrency violation. Caller может ретраить.
	ErrAgentConcurrentUpdate = errors.New("agent was modified concurrently, please retry")
)

// ─────────────────────────────────────────────────────────────────────────────
// Input DTOs (service-level, не путать с MCP-params или HTTP-DTO)
// ─────────────────────────────────────────────────────────────────────────────

// CreateAgentInput — параметры создания агента.
type CreateAgentInput struct {
	Name            string
	Role            models.AgentRole
	ExecutionKind   models.AgentExecutionKind
	RoleDescription *string
	SystemPrompt    *string
	PromptID        *uuid.UUID
	Model           *string                    // для llm; запрещено для sandbox. nil допустим ("не сконфигурирован").
	ProviderKind    *models.AgentProviderKind
	CodeBackend     *models.CodeBackend // обязательно для sandbox; запрещено для llm
	Temperature     *float64
	MaxTokens       *int
	IsActive        *bool
	TeamID          *uuid.UUID
	UserID          *uuid.UUID
}

// UpdateAgentInput — патч-параметры. Все поля опциональные; nil = не менять.
// Name/ExecutionKind НЕ меняются через update (требуют пересоздания —
// смена runtime-режима меняет инвариант, какой executor использовать).
// Role — редактируется только для кастомных агентов (docs/agents-rework-plan.md §5.3).
type UpdateAgentInput struct {
	Role               *models.AgentRole
	RoleDescription    *string
	SystemPrompt       *string
	PromptID           *uuid.UUID
	ClearPromptID      bool
	Model              *string
	ProviderKind       *models.AgentProviderKind
	CodeBackend        *models.CodeBackend // Sprint 5 review fix #4: можно менять для sandbox-агента
	Temperature        *float64
	MaxTokens          *int
	IsActive           *bool
	InternalMCPEnabled *bool
	Settings           *map[string]any
}

// ─────────────────────────────────────────────────────────────────────────────
// Service
// ─────────────────────────────────────────────────────────────────────────────

// AgentService — бизнес-фасад над AgentRepository + AgentSecretRepository.
//
// Sprint 5 review fix #2: txManager используется для атомарных операций (SetSecret,
// Update). Без него Read-Modify-Write образует TOCTOU гонку.
// ErrAgentProviderNotConnected — пользователь не подключил требуемый LLM-провайдер.
var ErrAgentProviderNotConnected = errors.New("LLM provider not connected")

type AgentService struct {
	agentRepo      repository.AgentRepository
	secretRepo     repository.AgentSecretRepository
	rolePromptRepo repository.AgentRolePromptRepository
	apiKeyRepo     repository.ApiKeyRepository
	llmCredRepo    repository.UserLlmCredentialRepository
	encryptor      Encryptor
	txManager      repository.TransactionManager
}

// NewAgentService — конструктор. encryptor может быть nil — тогда set/delete
// секрета вернут ErrEncryptorNotConfigured. txManager — обязателен для
// concurrent-safe SetSecret/Update (Sprint 5 review fix #2).
func NewAgentService(
	agentRepo repository.AgentRepository,
	secretRepo repository.AgentSecretRepository,
	encryptor Encryptor,
	txManager repository.TransactionManager,
) *AgentService {
	return &AgentService{
		agentRepo:  agentRepo,
		secretRepo: secretRepo,
		encryptor:  encryptor,
		txManager:  txManager,
	}
}

// WithRolePromptRepo sets the AgentRolePromptRepository (needed for factory methods).
func (s *AgentService) WithRolePromptRepo(repo repository.AgentRolePromptRepository) *AgentService {
	s.rolePromptRepo = repo
	return s
}

// WithApiKeyRepo sets the ApiKeyRepository (needed for MCP key provisioning).
func (s *AgentService) WithApiKeyRepo(repo repository.ApiKeyRepository) *AgentService {
	s.apiKeyRepo = repo
	return s
}

// WithLlmCredRepo sets the UserLlmCredentialRepository (needed for §4.3 provider validation).
func (s *AgentService) WithLlmCredRepo(repo repository.UserLlmCredentialRepository) *AgentService {
	s.llmCredRepo = repo
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent CRUD
// ─────────────────────────────────────────────────────────────────────────────

// List — пагинированный список агентов. Filter может быть zero-value.
func (s *AgentService) List(ctx context.Context, filter repository.AgentFilter) ([]models.Agent, int64, error) {
	return s.agentRepo.List(ctx, filter)
}

// GetByID — полная запись агента (включая system_prompt).
func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	a, err := s.agentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, ErrAgentNotInRegistry
		}
		return nil, err
	}
	return a, nil
}

// Create — валидирует входные данные, заполняет дефолты, сохраняет.
func (s *AgentService) Create(ctx context.Context, in CreateAgentInput) (*models.Agent, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrAgentValidation)
	}
	if !in.Role.IsValid() {
		return nil, fmt.Errorf("%w: invalid role %q", ErrAgentValidation, in.Role)
	}
	if !in.ExecutionKind.IsValid() {
		return nil, fmt.Errorf("%w: invalid execution_kind %q (allowed: llm, sandbox)", ErrAgentValidation, in.ExecutionKind)
	}

	// Кастомная (не-системная) роль не имеет seed-промпта в agent_role_prompts,
	// поэтому требует явных инструкций: без role_description роутер не поймёт, что
	// агент делает (и не назначит его), а без system_prompt сам агент не знает, что
	// исполнять. Системные роли освобождены — у них есть дефолтные промпты.
	if !in.Role.IsSystem() {
		hasDesc := in.RoleDescription != nil && strings.TrimSpace(*in.RoleDescription) != ""
		hasPrompt := (in.SystemPrompt != nil && strings.TrimSpace(*in.SystemPrompt) != "") || in.PromptID != nil
		if !hasDesc || !hasPrompt {
			return nil, fmt.Errorf("%w: custom-role agent requires role_description and system_prompt", ErrAgentValidation)
		}
	}

	// role_description обязателен для видимости агента в каталоге Router'а:
	// loadRouterState отфильтровывает агентов с пустым role_description, и такой
	// агент никогда не получит задачу. Если вызывающий (напр. форма «Добавить агента»)
	// не передал описание — подставляем дефолт из role-промпта.
	roleDescription := in.RoleDescription
	if roleDescription == nil || strings.TrimSpace(*roleDescription) == "" {
		roleDescription = s.defaultRoleDescription(ctx, in.Role)
	}

	a := &models.Agent{
		Name:            in.Name,
		Role:            in.Role,
		ExecutionKind:   in.ExecutionKind,
		RoleDescription: roleDescription,
		SystemPrompt:    in.SystemPrompt,
		PromptID:        in.PromptID,
		ProviderKind:    in.ProviderKind,
		Temperature:     in.Temperature,
		MaxTokens:       in.MaxTokens,
		TeamID:          in.TeamID,
		UserID:          in.UserID,
		IsActive:        true,
	}
	if in.IsActive != nil {
		a.IsActive = *in.IsActive
	}

	// Mutual exclusivity (зеркалит CHECK chk_agents_kind_requirements).
	// Phase 1 §1.3: LLM-агент может иметь model=nil ("не сконфигурирован").
	// Валидация полноты (model+provider_kind) — при попытке запуска, не при создании.
	switch in.ExecutionKind {
	case models.AgentExecutionKindLLM:
		if in.CodeBackend != nil {
			return nil, fmt.Errorf("%w: llm-agent must NOT have code_backend", ErrAgentValidation)
		}
		a.Model = in.Model
	case models.AgentExecutionKindSandbox:
		if in.CodeBackend == nil {
			return nil, fmt.Errorf("%w: sandbox-agent requires code_backend", ErrAgentValidation)
		}
		if !in.CodeBackend.IsValid() {
			return nil, fmt.Errorf("%w: invalid code_backend %q (allowed: claude-code/aider/hermes/custom/antigravity)", ErrAgentValidation, *in.CodeBackend)
		}
		a.CodeBackend = in.CodeBackend
		// Для sandbox модель не хранится в колонке agents.model (CHECK chk_agents_kind_requirements),
		// а кладётся в code_backend_settings.model — оттуда её берёт сборщик артефактов бекенда.
		if in.Model != nil && strings.TrimSpace(*in.Model) != "" {
			settings := AgentCodeBackendSettings{Model: strings.TrimSpace(*in.Model)}
			raw, err := json.Marshal(settings)
			if err != nil {
				return nil, fmt.Errorf("marshal code_backend_settings: %w", err)
			}
			a.CodeBackendSettings = raw
		}
	}

	// Валидация provider_kind (если передан явно): мусорное значение → 400, а не
	// запись «битого» агента, которого потом не сможет запустить sandbox_auth_resolver.
	if a.ProviderKind != nil && !a.ProviderKind.IsValid() {
		return nil, fmt.Errorf("%w: invalid provider_kind %q", ErrAgentValidation, *a.ProviderKind)
	}

	// Auto-infer ProviderKind from Model if not explicitly provided
	if a.ProviderKind == nil && a.Model != nil && *a.Model != "" {
		model := *a.Model
		var pk models.AgentProviderKind
		if strings.Contains(model, "/") {
			pk = models.AgentProviderKindOpenRouter
			a.ProviderKind = &pk
		} else if strings.HasPrefix(model, "claude-") {
			pk = models.AgentProviderKindAnthropic
			a.ProviderKind = &pk
		} else if strings.HasPrefix(model, "deepseek-") {
			pk = models.AgentProviderKindDeepSeek
			a.ProviderKind = &pk
		} else if strings.HasPrefix(model, "glm-") {
			pk = models.AgentProviderKindZhipu
			a.ProviderKind = &pk
		} else if strings.HasPrefix(model, "antigravity-") {
			pk = models.AgentProviderKindAntigravity
			a.ProviderKind = &pk
		}
	}

	if err := s.validateRanges(in.Temperature, in.MaxTokens); err != nil {
		return nil, err
	}

	if err := s.agentRepo.Create(ctx, a); err != nil {
		if errors.Is(err, repository.ErrAgentNameTaken) {
			return nil, ErrAgentNameAlreadyTaken
		}
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

// defaultRoleDescription возвращает непустое описание роли: сначала пробует
// role-промпт (тот же источник, что у CreateDefaultProjectAgents), затем
// generic-fallback. Никогда не возвращает nil/пустую строку — иначе агент
// окажется невидим для Router'а.
func (s *AgentService) defaultRoleDescription(ctx context.Context, role models.AgentRole) *string {
	if s.rolePromptRepo != nil {
		if prompt, err := s.rolePromptRepo.GetByRole(ctx, string(role)); err == nil &&
			prompt.Description != nil && strings.TrimSpace(*prompt.Description) != "" {
			return prompt.Description
		}
	}
	desc := "Default " + string(role) + " agent"
	return &desc
}

// Update — частичное обновление с валидацией. Возвращает обновлённую запись.
// Name/Role/ExecutionKind не меняются через этот метод.
//
// Sprint 5 review fix #2 (Race condition): обёрнут в TransactionManager.WithTransaction
// + `SELECT ... FOR UPDATE` через GetByIDForUpdate. Это блокирует строку до конца tx,
// две параллельные UPDATE'а сериализуются. Дополнительно — optimistic concurrency
// через expected_updated_at (defence-in-depth: даже если FOR UPDATE по какой-то
// причине пропустил, обновим только если updated_at не изменился).
func (s *AgentService) Update(ctx context.Context, id uuid.UUID, in UpdateAgentInput) (*models.Agent, error) {
	if s.txManager == nil {
		return nil, fmt.Errorf("AgentService: txManager is not configured (required for concurrent-safe Update)")
	}
	var updated *models.Agent
	txErr := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// 1. Lock row до конца tx — параллельные Update сериализуются.
		current, err := s.agentRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return ErrAgentNotInRegistry
			}
			return err
		}
		expectedUpdatedAt := current.UpdatedAt

		// 2. Применяем патчи с валидацией.
		if err := s.applyUpdatePatch(current, in); err != nil {
			return err
		}

		// 3. Записываем с optimistic check (защита от ситуации когда FOR UPDATE
		//    был обойдён — например, через прямой UPDATE мимо нашего сервиса).
		if err := s.agentRepo.Update(txCtx, current, expectedUpdatedAt); err != nil {
			if errors.Is(err, repository.ErrAgentNameTaken) {
				return ErrAgentNameAlreadyTaken
			}
			if errors.Is(err, repository.ErrAgentConcurrentUpdate) {
				return ErrAgentConcurrentUpdate
			}
			return fmt.Errorf("update agent %s: %w", id, err)
		}
		updated = current
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return updated, nil
}

// applyUpdatePatch — чисто in-memory мутация *current по UpdateAgentInput.
// Никакого I/O; всё валидируется сразу и возвращает ErrAgentValidation.
func (s *AgentService) applyUpdatePatch(current *models.Agent, in UpdateAgentInput) error {
	if in.Role != nil {
		if !in.Role.IsValid() {
			return fmt.Errorf("%w: invalid role %q", ErrAgentValidation, *in.Role)
		}
		if current.Role.IsAutoCreated() {
			return fmt.Errorf("%w: cannot change role of auto-created agent (current role=%s)", ErrAgentValidation, current.Role)
		}
		current.Role = *in.Role
	}
	if in.RoleDescription != nil {
		current.RoleDescription = in.RoleDescription
	}
	if in.SystemPrompt != nil {
		current.SystemPrompt = in.SystemPrompt
	}
	if in.ClearPromptID {
		current.PromptID = nil
		current.Prompt = nil
	} else if in.PromptID != nil {
		current.PromptID = in.PromptID
		current.Prompt = nil
	}

	// Model — только для llm-агентов. Phase 1 §1.3: допускаем nil (сброс к "не сконфигурирован").
	if in.Model != nil {
		if current.ExecutionKind != models.AgentExecutionKindLLM {
			return fmt.Errorf("%w: model is allowed only for llm-agents (current kind=%s)", ErrAgentValidation, current.ExecutionKind)
		}
		current.Model = in.Model
	}
	if in.ProviderKind != nil {
		if !in.ProviderKind.IsValid() {
			return fmt.Errorf("%w: invalid provider_kind %q", ErrAgentValidation, *in.ProviderKind)
		}
		current.ProviderKind = in.ProviderKind
	}

	// Sprint 5 review fix #4: CodeBackend — только для sandbox-агентов.
	if in.CodeBackend != nil {
		if current.ExecutionKind != models.AgentExecutionKindSandbox {
			return fmt.Errorf("%w: code_backend is allowed only for sandbox-agents (current kind=%s)", ErrAgentValidation, current.ExecutionKind)
		}
		if !in.CodeBackend.IsValid() {
			return fmt.Errorf("%w: invalid code_backend %q (allowed: claude-code/aider/hermes/custom/antigravity)", ErrAgentValidation, *in.CodeBackend)
		}
		current.CodeBackend = in.CodeBackend
	}

	if err := s.validateRanges(in.Temperature, in.MaxTokens); err != nil {
		return err
	}
	if in.Temperature != nil {
		current.Temperature = in.Temperature
	}
	if in.MaxTokens != nil {
		current.MaxTokens = in.MaxTokens
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.InternalMCPEnabled != nil {
		current.InternalMCPEnabled = *in.InternalMCPEnabled
	}
	if in.Settings != nil {
		bytes, err := json.Marshal(*in.Settings)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal settings JSON: %w", ErrAgentValidation, err)
		}
		current.Settings = datatypes.JSON(bytes)
	}
	return nil
}

// validateRanges проверяет temperature (0..2) и max_tokens (>0). nil-поля пропускает.
// Эти инварианты также продублированы в CHECK constraints миграции 031, но
// предпочитаем явную валидацию на service-уровне для понятных user-facing ошибок.
func (s *AgentService) validateRanges(temperature *float64, maxTokens *int) error {
	if temperature != nil && (*temperature < 0 || *temperature > 2) {
		return fmt.Errorf("%w: temperature must be in [0, 2]", ErrAgentValidation)
	}
	if maxTokens != nil && *maxTokens <= 0 {
		return fmt.Errorf("%w: max_tokens must be > 0", ErrAgentValidation)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent secrets
// ─────────────────────────────────────────────────────────────────────────────

// SetSecretInput — параметры установки/обновления секрета.
type SetSecretInput struct {
	AgentID uuid.UUID
	KeyName string
	Value   string
}

// SetSecretOutput — результат set (id новой записи).
type SetSecretOutput struct {
	SecretID uuid.UUID
	AgentID  uuid.UUID
	KeyName  string
}

// SetSecret — идемпотентная атомарная установка секрета.
//
// Sprint 5 review fix #2 (TOCTOU race): обёрнут в TransactionManager.WithTransaction.
// Внутри tx:
//   1. Валидация key_name (regex).
//   2. Шифрование с новым UUID как AAD (происходит ДО tx, чтобы не держать lock
//      пока crypto работает).
//   3. SELECT existing FOR UPDATE — блокирует existing row до commit'а (если есть).
//   4. Если existing — DELETE.
//   5. INSERT new.
//   6. COMMIT.
//
// Race-safety: при concurrent SetSecret для одной (agent_id, key_name):
//   - Tx A берёт FOR UPDATE lock первой → Tx B ждёт.
//   - Tx A коммитит → Tx B продолжает, видит новую row, DELETE'ит и INSERT'ит свою.
//   - Один из двух INSERT может всё равно конфликтовать по unique (agent_id, key_name)
//     если timing патологический — caller получит обёрнутую ошибку и может ретраить.
//
// Back-read невозможен через этот сервис (это by design — read нужен только Sandbox-executor'у
// при запуске агента; через MCP/HTTP секреты НЕ читаются).
func (s *AgentService) SetSecret(ctx context.Context, in SetSecretInput) (*SetSecretOutput, error) {
	if s.encryptor == nil {
		return nil, ErrEncryptorNotConfigured
	}
	if s.txManager == nil {
		return nil, fmt.Errorf("AgentService: txManager is not configured (required for concurrent-safe SetSecret)")
	}
	if in.AgentID == uuid.Nil {
		return nil, fmt.Errorf("%w: agent_id is required", ErrAgentValidation)
	}
	if !models.ValidateAgentSecretKeyName(in.KeyName) {
		return nil, ErrAgentSecretInvalidKey
	}
	if in.Value == "" {
		return nil, fmt.Errorf("%w: value must be non-empty", ErrAgentValidation)
	}

	// Шифрование ВНЕ транзакции (CPU-bound, не должно держать DB-lock).
	// AAD = ID.String() — генерируем UUID заранее, чтобы encrypt'ить им же.
	secret := &models.AgentSecret{
		ID:      uuid.New(),
		AgentID: in.AgentID,
		KeyName: in.KeyName,
	}
	blob, err := s.encryptor.Encrypt([]byte(in.Value), []byte(secret.ID.String()))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	if len(blob) < crypto.MinCiphertextBlobLen {
		return nil, fmt.Errorf("%w: encryptor produced unexpectedly small blob (NoopEncryptor?)", ErrEncryptorNotConfigured)
	}
	secret.EncryptedValue = blob

	txErr := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Идемпотентность: DELETE old + INSERT new (rotates UUID, обнуляет AAD-историю).
		existing, getErr := s.secretRepo.GetByName(txCtx, in.AgentID, in.KeyName)
		if getErr != nil && !errors.Is(getErr, repository.ErrAgentSecretNotFound) {
			return fmt.Errorf("check existing secret: %w", getErr)
		}
		if existing != nil {
			if delErr := s.secretRepo.Delete(txCtx, existing.ID); delErr != nil {
				return fmt.Errorf("remove existing secret: %w", delErr)
			}
		}
		if createErr := s.secretRepo.Create(txCtx, secret); createErr != nil {
			return fmt.Errorf("persist secret: %w", createErr)
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return &SetSecretOutput{
		SecretID: secret.ID,
		AgentID:  secret.AgentID,
		KeyName:  secret.KeyName,
	}, nil
}

// DeleteSecret — удаление по UUID записи.
func (s *AgentService) DeleteSecret(ctx context.Context, secretID uuid.UUID) error {
	if err := s.secretRepo.Delete(ctx, secretID); err != nil {
		if errors.Is(err, repository.ErrAgentSecretNotFound) {
			return ErrAgentNotInRegistry // переиспользуем; caller покажет "secret not found"
		}
		return fmt.Errorf("delete secret %s: %w", secretID, err)
	}
	return nil
}

// Delete — hard-delete агента из реестра. Hard-delete потому что у нас уже
// есть soft-disable через is_active=false (для backward-compat с in-flight задачами).
// Кейс использования: cleanup тестовых агентов через `t.Cleanup` в featuresmoke,
// плюс ручное удаление никем не использующейся записи через UI.
//
// Sprint 5: репозиторий сам каскадно не чистит agent_secrets / agent_tool_bindings —
// сделать это надо в service-слое в одной транзакции, иначе останутся orphan-записи.
func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Сначала секреты (FK на agents.id мог быть с RESTRICT — точно не знаем).
		if err := s.secretRepo.DeleteByAgentID(txCtx, id); err != nil {
			if !errors.Is(err, repository.ErrAgentSecretNotFound) {
				return fmt.Errorf("delete agent secrets: %w", err)
			}
		}
		// Затем сам агент.
		if err := s.agentRepo.Delete(txCtx, id); err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return ErrAgentNotInRegistry
			}
			return fmt.Errorf("delete agent %s: %w", id, err)
		}
		return nil
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 4 §4.3 — provider validation
// ─────────────────────────────────────────────────────────────────────────────

// ValidateProviderConnected проверяет что у пользователя подключён LLM-провайдер
// соответствующий agent.ProviderKind. Для anthropic_oauth проверка не требуется
// (ключ из claude_code_subscriptions, не из user_llm_credentials).
func (s *AgentService) ValidateProviderConnected(ctx context.Context, userID uuid.UUID, providerKind *models.AgentProviderKind) error {
	if providerKind == nil {
		return nil
	}
	if s.llmCredRepo == nil {
		return nil
	}
	if *providerKind == models.AgentProviderKindAnthropicOAuth {
		return nil
	}

	llmProvider := providerKind.UserLLMProvider()
	if llmProvider == "" {
		return nil
	}

	_, err := s.llmCredRepo.GetByUserAndProvider(ctx, userID, llmProvider)
	if err != nil {
		if errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
			return fmt.Errorf("%w: provider %s is not connected — configure it in settings", ErrAgentProviderNotConnected, *providerKind)
		}
		return fmt.Errorf("check provider credentials: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 2: Factory methods — auto-creation of agents
// ─────────────────────────────────────────────────────────────────────────────

// newBaseAgent builds an Agent with all NOT NULL JSONB defaults filled in.
func newBaseAgent(name string, role models.AgentRole, kind models.AgentExecutionKind, prompt *string) *models.Agent {
	return &models.Agent{
		Name:                name,
		Role:                role,
		ExecutionKind:       kind,
		IsActive:            true,
		SystemPrompt:        prompt,
		Skills:              datatypes.JSON([]byte(`[]`)),
		Settings:            datatypes.JSON([]byte(`{}`)),
		ModelConfig:         datatypes.JSON([]byte(`{}`)),
		CodeBackendSettings: datatypes.JSON([]byte(`{}`)),
		SandboxPermissions:  datatypes.JSON([]byte(`{}`)),
	}
}

// CreateDefaultAssistant creates a per-user assistant with system prompt from
// agent_role_prompts and a scoped MCP API key. LLM settings (provider_kind,
// model) are left NULL — user configures them via UI.
// EnsureAssistantAgent — user-агент ассистента текущего пользователя; при
// отсутствии провиженится из дефолтного role-промпта (та же логика, что в
// чате ассистента) — нужен вкладке настроек, открытой до первого чата.
func (s *AgentService) EnsureAssistantAgent(ctx context.Context, userID uuid.UUID) (*models.Agent, error) {
	agent, err := s.agentRepo.GetByUserAndRole(ctx, userID, string(models.AgentRoleAssistant))
	if err == nil {
		return agent, nil
	}
	if !errors.Is(err, repository.ErrAgentNotFound) {
		return nil, err
	}
	if err := s.CreateDefaultAssistant(ctx, userID); err != nil {
		return nil, err
	}
	return s.agentRepo.GetByUserAndRole(ctx, userID, string(models.AgentRoleAssistant))
}

// ResolveAssistantPromptForUser — действующий промпт ассистента пользователя
// для наследования копией (copy-on-create проекта): user-агент → дефолт роли.
// Пустая строка — только если нет ни того, ни другого (вызывающий не наследует).
func (s *AgentService) ResolveAssistantPromptForUser(ctx context.Context, userID uuid.UUID) string {
	if agent, err := s.agentRepo.GetByUserAndRole(ctx, userID, string(models.AgentRoleAssistant)); err == nil &&
		agent.SystemPrompt != nil && strings.TrimSpace(*agent.SystemPrompt) != "" {
		return *agent.SystemPrompt
	}
	if s.rolePromptRepo != nil {
		if prompt, err := s.rolePromptRepo.GetByRole(ctx, string(models.AgentRoleAssistant)); err == nil {
			return prompt.Content
		}
	}
	return ""
}

// EnsureEnhancerAgent — user-агент энхансера текущего пользователя; при
// отсутствии провижинится из дефолтного role-промпта (agent_role_prompts),
// как assistant. MCP-ключ не выпускается: энхансер работает in-process через
// agentloop, внешний MCP HTTP ему не нужен. LLM-настройки (provider_kind,
// model) остаются NULL — наследуются логикой резолвера/пользователем через UI.
func (s *AgentService) EnsureEnhancerAgent(ctx context.Context, userID uuid.UUID) (*models.Agent, error) {
	agent, err := s.agentRepo.GetByUserAndRole(ctx, userID, string(models.AgentRoleEnhancer))
	if err == nil {
		return agent, nil
	}
	if !errors.Is(err, repository.ErrAgentNotFound) {
		return nil, err
	}
	if s.rolePromptRepo == nil {
		return nil, fmt.Errorf("AgentService: rolePromptRepo is required for EnsureEnhancerAgent")
	}
	prompt, err := s.rolePromptRepo.GetByRole(ctx, string(models.AgentRoleEnhancer))
	if err != nil {
		return nil, fmt.Errorf("default prompt for enhancer: %w", err)
	}
	created := newBaseAgent("enhancer", models.AgentRoleEnhancer, models.AgentExecutionKindLLM, &prompt.Content)
	created.UserID = &userID
	if err := s.agentRepo.Create(ctx, created); err != nil {
		return nil, fmt.Errorf("create enhancer agent: %w", err)
	}
	return s.agentRepo.GetByUserAndRole(ctx, userID, string(models.AgentRoleEnhancer))
}

func (s *AgentService) CreateDefaultAssistant(ctx context.Context, userID uuid.UUID) error {
	if s.rolePromptRepo == nil {
		return fmt.Errorf("AgentService: rolePromptRepo is required for CreateDefaultAssistant")
	}

	prompt, err := s.rolePromptRepo.GetByRole(ctx, string(models.AgentRoleAssistant))
	if err != nil {
		return fmt.Errorf("default prompt for assistant: %w", err)
	}

	agent := newBaseAgent("assistant", models.AgentRoleAssistant, models.AgentExecutionKindLLM, &prompt.Content)
	agent.UserID = &userID

	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return fmt.Errorf("create assistant agent: %w", err)
	}

	return s.provisionMCPKey(ctx, agent.ID, userID)
}

// CreateDefaultProjectAgents creates default agents for a team.
// For development teams, all 7 worker/decision agents (router, planner, decomposer, reviewer, developer, tester, merger) are created.
// For other team types, only the router agent is created.
// The orchestrator is a Go engine (orchestrator_v2.go), not an LLM agent, so no orchestrator agent row is seeded.
// Each gets a system prompt and description from agent_role_prompts. Default LLM/sandbox settings are pre-configured.
func (s *AgentService) CreateDefaultProjectAgents(ctx context.Context, teamID uuid.UUID, teamType string) error {
	if s.rolePromptRepo == nil {
		return fmt.Errorf("AgentService: rolePromptRepo is required for CreateDefaultProjectAgents")
	}

	sHelper := func(str string) *string { return &str }
	fHelper := func(f float64) *float64 { return &f }
	iHelper := func(i int) *int { return &i }
	pHelper := func(p models.AgentProviderKind) *models.AgentProviderKind { return &p }
	cbHelper := func(cb models.CodeBackend) *models.CodeBackend { return &cb }

	roles := []struct {
		name         string
		role         models.AgentRole
		kind         models.AgentExecutionKind
		providerKind *models.AgentProviderKind
		model        *string
		codeBackend  *models.CodeBackend
		temperature  *float64
		maxTokens    *int
		settings     string
		perms        string
	}{
		// Агент роли router — единственный LLM в петле оркестрации: Orchestrator-движок
		// (Go, orchestrator_v2.go) на каждом шаге зовёт именно его (RouterService.Decide).
		// Отдельный агент роли orchestrator НЕ создаётся: движок — это код, а не LLM, и
		// запись orchestrator-агента раньше никем не загружалась (мёртвый сид). Роль
		// orchestrator остаётся зарезервированной системной (см. AgentRole.IsSystem).
		{
			name:         "router",
			role:         models.AgentRoleRouter,
			kind:         models.AgentExecutionKindLLM,
			providerKind: pHelper(models.AgentProviderKindOpenRouter),
			model:        sHelper("deepseek/deepseek-v4-flash"),
			temperature:  fHelper(0.2),
			maxTokens:    iHelper(4096),
		},
	}

	if teamType == "development" {
		roles = append(roles, []struct {
			name         string
			role         models.AgentRole
			kind         models.AgentExecutionKind
			providerKind *models.AgentProviderKind
			model        *string
			codeBackend  *models.CodeBackend
			temperature  *float64
			maxTokens    *int
			settings     string
			perms        string
		}{
			{
				name:         "planner",
				role:         models.AgentRolePlanner,
				kind:         models.AgentExecutionKindLLM,
				providerKind: pHelper(models.AgentProviderKindOpenRouter),
				model:        sHelper("deepseek/deepseek-v4-flash"),
				temperature:  fHelper(0.3),
				maxTokens:    iHelper(8192),
			},
			{
				name:         "decomposer",
				role:         models.AgentRoleDecomposer,
				kind:         models.AgentExecutionKindLLM,
				providerKind: pHelper(models.AgentProviderKindOpenRouter),
				model:        sHelper("deepseek/deepseek-v4-flash"),
				temperature:  fHelper(0.3),
				maxTokens:    iHelper(8192),
			},
			{
				name:        "reviewer",
				role:        models.AgentRoleReviewer,
				kind:        models.AgentExecutionKindSandbox,
				codeBackend: cbHelper(models.CodeBackendClaudeCode),
				settings:    `{"permission_mode": "auto"}`,
				perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
			},
			{
				name:        "developer",
				role:        models.AgentRoleDeveloper,
				kind:        models.AgentExecutionKindSandbox,
				codeBackend: cbHelper(models.CodeBackendClaudeCode),
				settings:    `{"permission_mode": "auto"}`,
				perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
			},
			{
				name:        "tester",
				role:        models.AgentRoleTester,
				kind:        models.AgentExecutionKindSandbox,
				codeBackend: cbHelper(models.CodeBackendClaudeCode),
				settings:    `{"permission_mode": "auto"}`,
				perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
			},
			{
				name:        "merger",
				role:        models.AgentRoleMerger,
				kind:        models.AgentExecutionKindSandbox,
				codeBackend: cbHelper(models.CodeBackendClaudeCode),
				settings:    `{"permission_mode": "auto"}`,
				perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
			},
		}...)
	}

	for _, r := range roles {
		prompt, err := s.rolePromptRepo.GetByRole(ctx, string(r.role))
		if err != nil {
			return fmt.Errorf("default prompt for %s: %w", r.role, err)
		}

		agent := newBaseAgent(r.name, r.role, r.kind, &prompt.Content)
		agent.TeamID = &teamID
		agent.ProviderKind = r.providerKind
		agent.Model = r.model
		agent.CodeBackend = r.codeBackend
		agent.Temperature = r.temperature
		agent.MaxTokens = r.maxTokens
		if prompt.Description != nil {
			agent.RoleDescription = prompt.Description
		} else {
			desc := "Default " + r.name + " agent"
			agent.RoleDescription = &desc
		}
		if r.settings != "" {
			agent.CodeBackendSettings = datatypes.JSON([]byte(r.settings))
		}
		if r.perms != "" {
			agent.SandboxPermissions = datatypes.JSON([]byte(r.perms))
		}
		if r.kind == models.AgentExecutionKindSandbox {
			agent.RequiresCodeContext = true
		}

		if err := s.agentRepo.Create(ctx, agent); err != nil {
			return fmt.Errorf("create %s agent: %w", r.name, err)
		}
	}
	return nil
}

// provisionMCPKey generates a scoped API key for the assistant's MCP access.
func (s *AgentService) provisionMCPKey(ctx context.Context, agentID, userID uuid.UUID) error {
	if s.apiKeyRepo == nil {
		return fmt.Errorf("AgentService: apiKeyRepo is required for provisionMCPKey")
	}
	if s.encryptor == nil {
		return ErrEncryptorNotConfigured
	}

	rawKey, err := generateMCPKey()
	if err != nil {
		return fmt.Errorf("generate MCP key: %w", err)
	}

	keyHash := hashMCPKey(rawKey)
	keyPrefix := rawKey[:12]

	apiKey := &models.ApiKey{
		UserID:    userID,
		Name:      fmt.Sprintf("assistant-mcp-%s", agentID),
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    `"mcp"`,
	}
	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return fmt.Errorf("create MCP api key: %w", err)
	}

	secret := &models.AgentSecret{
		ID:      uuid.New(),
		AgentID: agentID,
		KeyName: "DEVTEAM_MCP_TOKEN",
	}
	blob, err := s.encryptor.Encrypt([]byte(rawKey), []byte(secret.ID.String()))
	if err != nil {
		return fmt.Errorf("encrypt MCP key: %w", err)
	}
	secret.EncryptedValue = blob

	if err := s.secretRepo.Create(ctx, secret); err != nil {
		return fmt.Errorf("persist MCP secret: %w", err)
	}
	return nil
}

func generateMCPKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "wibe_" + hex.EncodeToString(bytes), nil
}

func hashMCPKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}
