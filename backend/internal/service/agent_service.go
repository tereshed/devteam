package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
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
	Model           *string             // обязательно для llm; запрещено для sandbox
	CodeBackend     *models.CodeBackend // обязательно для sandbox; запрещено для llm
	Temperature     *float64
	MaxTokens       *int
	IsActive        *bool
}

// UpdateAgentInput — патч-параметры. Все поля опциональные; nil = не менять.
// Name/Role/ExecutionKind НЕ меняются через update (требуют пересоздания —
// смена runtime-режима меняет инвариант, какой executor использовать).
type UpdateAgentInput struct {
	RoleDescription *string
	SystemPrompt    *string
	Model           *string
	CodeBackend     *models.CodeBackend // Sprint 5 review fix #4: можно менять для sandbox-агента
	Temperature     *float64
	MaxTokens       *int
	IsActive        *bool
}

// ─────────────────────────────────────────────────────────────────────────────
// Service
// ─────────────────────────────────────────────────────────────────────────────

// AgentService — бизнес-фасад над AgentRepository + AgentSecretRepository.
//
// Sprint 5 review fix #2: txManager используется для атомарных операций (SetSecret,
// Update). Без него Read-Modify-Write образует TOCTOU гонку.
type AgentService struct {
	agentRepo  repository.AgentRepository
	secretRepo repository.AgentSecretRepository
	encryptor  Encryptor
	txManager  repository.TransactionManager
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
		Temperature:     in.Temperature,
		MaxTokens:       in.MaxTokens,
		IsActive:        true,
	}
	if in.IsActive != nil {
		a.IsActive = *in.IsActive
	}

	// Mutual exclusivity (зеркалит CHECK chk_agents_kind_requirements).
	switch in.ExecutionKind {
	case models.AgentExecutionKindLLM:
		if in.Model == nil || *in.Model == "" {
			return nil, fmt.Errorf("%w: llm-agent requires non-empty model", ErrAgentValidation)
		}
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

	// Model — только для llm-агентов.
	if in.Model != nil {
		if current.ExecutionKind != models.AgentExecutionKindLLM {
			return fmt.Errorf("%w: model is allowed only for llm-agents (current kind=%s)", ErrAgentValidation, current.ExecutionKind)
		}
		if *in.Model == "" {
			return fmt.Errorf("%w: model must be non-empty when set", ErrAgentValidation)
		}
		current.Model = in.Model
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
