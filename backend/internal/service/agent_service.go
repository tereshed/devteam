package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

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
// Name/Role/ExecutionKind НЕ меняются через update (требуют пересоздания —
// смена runtime-режима меняет инвариант, какой executor использовать).
type UpdateAgentInput struct {
	RoleDescription    *string
	SystemPrompt       *string
	Model              *string
	ProviderKind       *models.AgentProviderKind
	CodeBackend        *models.CodeBackend // Sprint 5 review fix #4: можно менять для sandbox-агента
	Temperature        *float64
	MaxTokens          *int
	IsActive           *bool
	InternalMCPEnabled *bool
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

	a := &models.Agent{
		Name:            in.Name,
		Role:            in.Role,
		ExecutionKind:   in.ExecutionKind,
		RoleDescription: in.RoleDescription,
		SystemPrompt:    in.SystemPrompt,
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
			return nil, fmt.Errorf("%w: invalid code_backend %q (allowed: claude-code/aider/hermes/custom)", ErrAgentValidation, *in.CodeBackend)
		}
		if in.Model != nil && *in.Model != "" {
			return nil, fmt.Errorf("%w: sandbox-agent must NOT have model", ErrAgentValidation)
		}
		a.CodeBackend = in.CodeBackend
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
	if in.RoleDescription != nil {
		current.RoleDescription = in.RoleDescription
	}
	if in.SystemPrompt != nil {
		current.SystemPrompt = in.SystemPrompt
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
			return fmt.Errorf("%w: invalid code_backend %q (allowed: claude-code/aider/hermes/custom)", ErrAgentValidation, *in.CodeBackend)
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

// CreateDefaultProjectAgents creates orchestrator + router for a team.
// Each gets a system prompt from agent_role_prompts. LLM settings are left NULL.
func (s *AgentService) CreateDefaultProjectAgents(ctx context.Context, teamID uuid.UUID) error {
	if s.rolePromptRepo == nil {
		return fmt.Errorf("AgentService: rolePromptRepo is required for CreateDefaultProjectAgents")
	}

	roles := []struct {
		name string
		role models.AgentRole
	}{
		{"orchestrator", models.AgentRoleOrchestrator},
		{"router", models.AgentRoleRouter},
	}
	for _, r := range roles {
		prompt, err := s.rolePromptRepo.GetByRole(ctx, string(r.role))
		if err != nil {
			return fmt.Errorf("default prompt for %s: %w", r.role, err)
		}

		agent := newBaseAgent(r.name, r.role, models.AgentExecutionKindLLM, &prompt.Content)
		agent.TeamID = &teamID

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
