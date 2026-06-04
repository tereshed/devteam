package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
)

// memTxManager — in-memory mock TransactionManager.
//
// Sprint 5 review fix #2: серьёзный момент — для теста concurrent SetSecret
// мы ДОЛЖНЫ моделировать сериализацию транзакций (как делает real БД через
// FOR UPDATE / serializable isolation). Без mutex в самом tx-manager'е
// in-memory мок пропускает гонки, которые real-postgres tx поймал бы.
//
// Global mutex — упрощение (real БД сериализует только конкурирующие транзакции
// по lock'нутым ресурсам), но для unit-тестов этого достаточно.
type memTxManager struct{ mu sync.Mutex }

func (m *memTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(ctx)
}

// newMemTxManager — pointer-конструктор для shared-state mutex'а.
func newMemTxManager() *memTxManager { return &memTxManager{} }

// newAgentSvcForTest — стандартная инициализация сервиса для тестов с дефолтными моками.
func newAgentSvcForTest(t *testing.T) *AgentService {
	t.Helper()
	return NewAgentService(newMemAgentRepo(), newMemSecretRepo(), makeAESEncryptor(t), newMemTxManager())
}
func newAgentSvcWithRepos(t *testing.T, agentRepo *memAgentRepo, secretRepo *memSecretRepo, enc Encryptor) *AgentService {
	t.Helper()
	return NewAgentService(agentRepo, secretRepo, enc, newMemTxManager())
}

// agent_service_test.go — Sprint 5 review fix #1 (layer violation). Тесты бизнес-логики
// поверх in-memory mock repos. Цель: убедиться что валидация / mutual exclusivity /
// идемпотентность секретов / CodeBackend update — корректны на service-уровне.

// ─────────────────────────────────────────────────────────────────────────────
// In-memory mock repos
// ─────────────────────────────────────────────────────────────────────────────

type memAgentRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*models.Agent
	byName  map[string]*models.Agent
}

func newMemAgentRepo() *memAgentRepo {
	return &memAgentRepo{byID: map[uuid.UUID]*models.Agent{}, byName: map[string]*models.Agent{}}
}

func (r *memAgentRepo) Create(_ context.Context, a *models.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[a.Name]; exists {
		return repository.ErrAgentNameTaken
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	cp := *a
	r.byID[a.ID] = &cp
	r.byName[a.Name] = &cp
	return nil
}
func (r *memAgentRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *memAgentRepo) GetByIDForUpdate(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	// В in-memory моке FOR UPDATE моделируется тем же mu.Lock — операции
	// GetByIDForUpdate → ... → Update идут под одним лок'ом (см. memTxManager.WithTransaction).
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *memAgentRepo) GetByName(_ context.Context, name string) (*models.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byName[name]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *memAgentRepo) List(_ context.Context, _ repository.AgentFilter) ([]models.Agent, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Agent, 0, len(r.byID))
	for _, a := range r.byID {
		out = append(out, *a)
	}
	return out, int64(len(out)), nil
}
func (r *memAgentRepo) Update(_ context.Context, a *models.Agent, expectedUpdatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[a.ID]
	if !ok {
		return repository.ErrAgentNotFound
	}
	if !existing.UpdatedAt.Equal(expectedUpdatedAt) {
		return repository.ErrAgentConcurrentUpdate
	}
	cp := *a
	cp.UpdatedAt = time.Now().UTC()
	r.byID[a.ID] = &cp
	r.byName[a.Name] = &cp
	return nil
}
func (r *memAgentRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return repository.ErrAgentNotFound
	}
	delete(r.byID, id)
	delete(r.byName, a.Name)
	return nil
}

type memSecretRepo struct {
	mu      sync.Mutex
	secrets map[uuid.UUID]*models.AgentSecret
}

func newMemSecretRepo() *memSecretRepo {
	return &memSecretRepo{secrets: map[uuid.UUID]*models.AgentSecret{}}
}
func (r *memSecretRepo) Create(_ context.Context, s *models.AgentSecret) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	cp := *s
	r.secrets[s.ID] = &cp
	return nil
}
func (r *memSecretRepo) GetByName(_ context.Context, agentID uuid.UUID, keyName string) (*models.AgentSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.secrets {
		if s.AgentID == agentID && s.KeyName == keyName {
			cp := *s
			return &cp, nil
		}
	}
	return nil, repository.ErrAgentSecretNotFound
}
func (r *memSecretRepo) ListByAgentID(_ context.Context, agentID uuid.UUID) ([]models.AgentSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.AgentSecret, 0)
	for _, s := range r.secrets {
		if s.AgentID == agentID {
			out = append(out, *s)
		}
	}
	return out, nil
}
func (r *memSecretRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.secrets[id]; !ok {
		return repository.ErrAgentSecretNotFound
	}
	delete(r.secrets, id)
	return nil
}
func (r *memSecretRepo) DeleteByAgentID(_ context.Context, agentID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.secrets {
		if s.AgentID == agentID {
			delete(r.secrets, id)
		}
	}
	return nil
}

// makeAESEncryptor — реальный AES-GCM encryptor для тестов с проверяемой длиной blob.
func makeAESEncryptor(t *testing.T) Encryptor {
	t.Helper()
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := crypto.NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("crypto.NewAESEncryptor: %v", err)
	}
	return enc
}

// ─────────────────────────────────────────────────────────────────────────────
// Create — validation
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentService_Create_LLMHappyPath(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude-sonnet-4-6"
	a, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "test-planner",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.Name != "test-planner" || a.Model == nil || *a.Model != model {
		t.Errorf("created agent mismatch: %+v", a)
	}
	if a.CodeBackend != nil {
		t.Errorf("llm-agent must NOT have code_backend, got %v", *a.CodeBackend)
	}
	if a.ProviderKind == nil || *a.ProviderKind != models.AgentProviderKindAnthropic {
		t.Errorf("expected auto-inferred provider_kind 'anthropic', got %v", a.ProviderKind)
	}
}

func TestAgentService_Create_LLMProviderInference(t *testing.T) {
	svc := newAgentSvcForTest(t)
	tests := []struct {
		model            string
		expectedProvider models.AgentProviderKind
	}{
		{"deepseek/deepseek-v4-flash", models.AgentProviderKindOpenRouter},
		{"claude-haiku-4-5-20251001", models.AgentProviderKindAnthropic},
		{"deepseek-chat", models.AgentProviderKindDeepSeek},
		{"glm-4", models.AgentProviderKindZhipu},
		{"antigravity-default", models.AgentProviderKindAntigravity},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			a, err := svc.Create(context.Background(), CreateAgentInput{
				Name:          "agent-" + uuid.New().String(),
				Role:          models.AgentRolePlanner,
				ExecutionKind: models.AgentExecutionKindLLM,
				Model:         &tt.model,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if a.ProviderKind == nil || *a.ProviderKind != tt.expectedProvider {
				t.Errorf("for model %q, expected provider %q, got %v", tt.model, tt.expectedProvider, a.ProviderKind)
			}
		})
	}
}

// Phase 1 §1.3: LLM-агент может быть создан без model ("не сконфигурирован").
func TestAgentService_Create_LLMWithoutModel_Allowed(t *testing.T) {
	svc := newAgentSvcForTest(t)
	a, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "unconfigured",
		Role:          models.AgentRoleReviewer,
		ExecutionKind: models.AgentExecutionKindLLM,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if a.Model != nil {
		t.Errorf("expected model=nil, got %v", *a.Model)
	}
}

func TestAgentService_Create_RejectsLLMWithCodeBackend(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude"
	cb := models.CodeBackendClaudeCode
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "mixed",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
		CodeBackend:   &cb, // ⚠️ violates mutual exclusivity
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_SandboxHappyPath(t *testing.T) {
	svc := newAgentSvcForTest(t)
	cb := models.CodeBackendHermes
	a, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "test-developer-hermes",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.CodeBackend == nil || *a.CodeBackend != models.CodeBackendHermes {
		t.Errorf("CodeBackend mismatch: %+v", a.CodeBackend)
	}
	if a.Model != nil {
		t.Errorf("sandbox-agent must NOT have model, got %v", *a.Model)
	}
}

func TestAgentService_Create_SandboxModelGoesToSettings(t *testing.T) {
	svc := newAgentSvcForTest(t)
	cb := models.CodeBackendClaudeCode
	model := "claude-haiku-4-5-20251001"
	a, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "sandbox-with-model",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
		Model:         &model,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Колонка model должна остаться пустой (CHECK chk_agents_kind_requirements),
	// а модель — уехать в code_backend_settings.model.
	if a.Model != nil {
		t.Errorf("sandbox-agent must NOT have model column, got %v", *a.Model)
	}
	var settings AgentCodeBackendSettings
	if err := json.Unmarshal(a.CodeBackendSettings, &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if settings.Model != model {
		t.Errorf("expected settings.model=%q, got %q", model, settings.Model)
	}
}

func TestAgentService_Create_RejectsInvalidProviderKind(t *testing.T) {
	svc := newAgentSvcForTest(t)
	pk := models.AgentProviderKind("not-a-real-provider")
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "bad-provider",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		ProviderKind:  &pk,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_CustomRoleHappyPath(t *testing.T) {
	svc := newAgentSvcForTest(t)
	desc := "Пишет SMM-посты для соцсетей по брифу."
	prompt := "Ты SMM-копирайтер. Пиши вовлекающие посты."
	a, err := svc.Create(context.Background(), CreateAgentInput{
		Name:            "smm-writer",
		Role:            models.AgentRole("smm_writer"),
		ExecutionKind:   models.AgentExecutionKindLLM,
		RoleDescription: &desc,
		SystemPrompt:    &prompt,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.Role != models.AgentRole("smm_writer") {
		t.Errorf("role mismatch: %q", a.Role)
	}
}

func TestAgentService_Create_CustomRoleRequiresInstructions(t *testing.T) {
	svc := newAgentSvcForTest(t)
	// Кастомная роль без role_description/system_prompt — отклоняем.
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "naked-custom",
		Role:          models.AgentRole("smm_writer"),
		ExecutionKind: models.AgentExecutionKindLLM,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_RejectsMalformedRole(t *testing.T) {
	svc := newAgentSvcForTest(t)
	desc := "x"
	prompt := "y"
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:            "bad-role",
		Role:            models.AgentRole("Bad Role!"),
		ExecutionKind:   models.AgentExecutionKindLLM,
		RoleDescription: &desc,
		SystemPrompt:    &prompt,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_RejectsSandboxWithoutCodeBackend(t *testing.T) {
	svc := newAgentSvcForTest(t)
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "bad-sandbox",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_RejectsInvalidCodeBackend(t *testing.T) {
	svc := newAgentSvcForTest(t)
	cb := models.CodeBackend("nonsense")
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "bad",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation for invalid code_backend, got %v", err)
	}
}

func TestAgentService_Create_TemperatureRange(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude"
	badTemp := 2.5
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "bad",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
		Temperature:   &badTemp,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

func TestAgentService_Create_DuplicateName(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude"
	in := CreateAgentInput{
		Name:          "dup",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
	}
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := svc.Create(context.Background(), in)
	if !errors.Is(err, ErrAgentNameAlreadyTaken) {
		t.Fatalf("expected ErrAgentNameAlreadyTaken, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — Sprint 5 review fix #4 (CodeBackend update)
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentService_Update_CodeBackendForSandbox(t *testing.T) {
	svc := newAgentSvcForTest(t)
	cb := models.CodeBackendClaudeCode
	created, _ := svc.Create(context.Background(), CreateAgentInput{
		Name:          "test-dev",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
	})
	newCB := models.CodeBackendAider
	updated, err := svc.Update(context.Background(), created.ID, UpdateAgentInput{
		CodeBackend: &newCB,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.CodeBackend == nil || *updated.CodeBackend != models.CodeBackendAider {
		t.Errorf("CodeBackend not updated; got %v", updated.CodeBackend)
	}
}

func TestAgentService_Update_CodeBackendForbiddenOnLLM(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude"
	created, _ := svc.Create(context.Background(), CreateAgentInput{
		Name:          "test-planner",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
	})
	cb := models.CodeBackendAider
	_, err := svc.Update(context.Background(), created.ID, UpdateAgentInput{
		CodeBackend: &cb,
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "sandbox-agents") {
		t.Errorf("error must explain LLM-agent restriction, got: %v", err)
	}
}

func TestAgentService_Update_ModelForbiddenOnSandbox(t *testing.T) {
	svc := newAgentSvcForTest(t)
	cb := models.CodeBackendClaudeCode
	created, _ := svc.Create(context.Background(), CreateAgentInput{
		Name:          "dev",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackend:   &cb,
	})
	model := "claude"
	_, err := svc.Update(context.Background(), created.ID, UpdateAgentInput{Model: &model})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
}

// TestAgentService_Update_OptimisticConcurrency — Sprint 5 review fix #2:
// если между чтением и записью кто-то изменил запись, Update вернёт
// ErrAgentConcurrentUpdate (а не "тихо" затрёт чужие изменения).
func TestAgentService_Update_OptimisticConcurrency(t *testing.T) {
	agentRepo := newMemAgentRepo()
	svc := NewAgentService(agentRepo, newMemSecretRepo(), makeAESEncryptor(t), newMemTxManager())
	model := "claude"
	created, _ := svc.Create(context.Background(), CreateAgentInput{
		Name:          "concurrent-target",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
	})

	// Симулируем "stale read": берём текущий updated_at из реальной записи,
	// потом мутируем её напрямую через repo (другой процесс), потом пробуем Update.
	originalUpdatedAt := agentRepo.byID[created.ID].UpdatedAt
	// Mutation by "another process":
	agentRepo.mu.Lock()
	agentRepo.byID[created.ID].UpdatedAt = originalUpdatedAt.Add(1 * time.Second)
	agentRepo.mu.Unlock()

	// Теперь сервис попробует обновить — внутри транзакции он СНОВА прочитает
	// (FOR UPDATE), увидит обновлённый updated_at, и Update пройдёт с этим
	// новым expected. Это корректное поведение FOR UPDATE моделирования.
	// Для теста РЕАЛЬНОГО Lost-Update сценария: симулируем что между
	// GetByIDForUpdate и Update произошла внешняя мутация. С нашим mu.Lock-моделью
	// это невозможно (lock держится). Поэтому тест проверяет другой инвариант:
	// Update с истёкшим updated_at напрямую через repo возвращает ErrConcurrentUpdate.

	stale := *created
	stale.UpdatedAt = originalUpdatedAt // stale — pre-мутация
	stale.IsActive = false
	err := agentRepo.Update(context.Background(), &stale, originalUpdatedAt)
	if !errors.Is(err, repository.ErrAgentConcurrentUpdate) {
		t.Fatalf("expected ErrAgentConcurrentUpdate for stale write, got: %v", err)
	}
}

// TestAgentService_SetSecret_ConcurrentSafe — Sprint 5 review fix #2: при
// параллельных SetSecret для одной (agent_id, key_name) — финально остаётся
// ровно ОДНА запись (не дубликаты, не lost write).
func TestAgentService_SetSecret_ConcurrentSafe(t *testing.T) {
	agentRepo := newMemAgentRepo()
	secretRepo := newMemSecretRepo()
	svc := NewAgentService(agentRepo, secretRepo, makeAESEncryptor(t), newMemTxManager())

	agentID := uuid.New()
	const N = 20
	var wg sync.WaitGroup
	errCount := 0
	var errMu sync.Mutex

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.SetSecret(context.Background(), SetSecretInput{
				AgentID: agentID,
				KeyName: "GITHUB_TOKEN",
				Value:   "value_iteration_" + uuid.New().String(),
			})
			if err != nil {
				errMu.Lock()
				errCount++
				errMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// Финально — РОВНО 1 запись (последний writer выигрывает; promo-cycling
	// DELETE→INSERT внутри tx + mu.Lock гарантирует, что параллельные операции
	// сериализуются).
	secrets, _ := secretRepo.ListByAgentID(context.Background(), agentID)
	if len(secrets) != 1 {
		t.Fatalf("after %d concurrent SetSecret, expected exactly 1 secret remaining; got %d (errors=%d)",
			N, len(secrets), errCount)
	}
}

func TestAgentService_Update_NotFound(t *testing.T) {
	svc := newAgentSvcForTest(t)
	_, err := svc.Update(context.Background(), uuid.New(), UpdateAgentInput{})
	if !errors.Is(err, ErrAgentNotInRegistry) {
		t.Fatalf("expected ErrAgentNotInRegistry, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SetSecret — encryption + idempotency
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentService_SetSecret_HappyPath(t *testing.T) {
	enc := makeAESEncryptor(t)
	secretRepo := newMemSecretRepo()
	svc := NewAgentService(newMemAgentRepo(), secretRepo, enc, newMemTxManager())

	out, err := svc.SetSecret(context.Background(), SetSecretInput{
		AgentID: uuid.New(),
		KeyName: "GITHUB_TOKEN",
		Value:   "ghp_real_token_value",
	})
	if err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if out.SecretID == uuid.Nil {
		t.Error("SecretID must be non-nil")
	}
	// Проверяем что blob — реальный AES-GCM (≥ MinCiphertextBlobLen), не plaintext.
	stored, _ := secretRepo.GetByName(context.Background(), out.AgentID, out.KeyName)
	if stored == nil {
		t.Fatal("secret was not persisted")
	}
	if len(stored.EncryptedValue) < crypto.MinCiphertextBlobLen {
		t.Errorf("blob too short (%d bytes), looks unencrypted", len(stored.EncryptedValue))
	}
	if strings.Contains(string(stored.EncryptedValue), "ghp_real_token_value") {
		t.Error("plaintext leaked into encrypted_value")
	}
	// Расшифровка с правильным AAD должна вернуть исходное значение.
	plaintext, err := enc.Decrypt(stored.EncryptedValue, []byte(stored.ID.String()))
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(plaintext) != "ghp_real_token_value" {
		t.Errorf("roundtrip failed: %q", plaintext)
	}
}

func TestAgentService_SetSecret_Idempotency(t *testing.T) {
	secretRepo := newMemSecretRepo()
	svc := newAgentSvcWithRepos(t, newMemAgentRepo(), secretRepo, makeAESEncryptor(t))
	agentID := uuid.New()

	in := SetSecretInput{AgentID: agentID, KeyName: "API_TOKEN", Value: "first"}
	out1, _ := svc.SetSecret(context.Background(), in)
	in.Value = "second"
	out2, err := svc.SetSecret(context.Background(), in)
	if err != nil {
		t.Fatalf("second SetSecret: %v", err)
	}
	if out1.SecretID == out2.SecretID {
		t.Error("idempotent set should generate NEW secret_id (delete-then-create pattern)")
	}
	secrets, _ := secretRepo.ListByAgentID(context.Background(), agentID)
	if len(secrets) != 1 {
		t.Errorf("only 1 secret should remain after re-set; got %d", len(secrets))
	}
}

func TestAgentService_SetSecret_InvalidKeyName(t *testing.T) {
	svc := newAgentSvcForTest(t)
	_, err := svc.SetSecret(context.Background(), SetSecretInput{
		AgentID: uuid.New(),
		KeyName: "lowercase_not_allowed",
		Value:   "x",
	})
	if !errors.Is(err, ErrAgentSecretInvalidKey) {
		t.Fatalf("expected ErrAgentSecretInvalidKey, got %v", err)
	}
}

func TestAgentService_SetSecret_NoEncryptor(t *testing.T) {
	svc := NewAgentService(newMemAgentRepo(), newMemSecretRepo(), nil, newMemTxManager())
	_, err := svc.SetSecret(context.Background(), SetSecretInput{
		AgentID: uuid.New(),
		KeyName: "TOKEN",
		Value:   "v",
	})
	if !errors.Is(err, ErrEncryptorNotConfigured) {
		t.Fatalf("expected ErrEncryptorNotConfigured, got %v", err)
	}
}

func TestAgentService_SetSecret_NoopEncryptorRejected(t *testing.T) {
	// NoopEncryptor возвращает plaintext (короче 29 байт для коротких значений).
	svc := NewAgentService(newMemAgentRepo(), newMemSecretRepo(), NoopEncryptor{}, newMemTxManager())
	_, err := svc.SetSecret(context.Background(), SetSecretInput{
		AgentID: uuid.New(),
		KeyName: "TOKEN",
		Value:   "short", // 5 байт < 29 → blob будет тоже 5 байт
	})
	if !errors.Is(err, ErrEncryptorNotConfigured) {
		t.Fatalf("expected NoopEncryptor to fail length-check, got %v", err)
	}
}

func TestAgentService_DeleteSecret(t *testing.T) {
	secretRepo := newMemSecretRepo()
	svc := newAgentSvcWithRepos(t, newMemAgentRepo(), secretRepo, makeAESEncryptor(t))
	out, _ := svc.SetSecret(context.Background(), SetSecretInput{
		AgentID: uuid.New(), KeyName: "X", Value: "v",
	})
	if err := svc.DeleteSecret(context.Background(), out.SecretID); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	secrets, _ := secretRepo.ListByAgentID(context.Background(), out.AgentID)
	if len(secrets) != 0 {
		t.Errorf("secret should be removed, got %d remaining", len(secrets))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 2: Factory methods — CreateDefaultAssistant, CreateDefaultProjectAgents
// ─────────────────────────────────────────────────────────────────────────────

type memRolePromptRepo struct {
	mu      sync.Mutex
	byRole  map[string]*models.AgentRolePrompt
}

func newMemRolePromptRepo() *memRolePromptRepo {
	return &memRolePromptRepo{byRole: map[string]*models.AgentRolePrompt{}}
}

func (r *memRolePromptRepo) seed(role, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byRole[role] = &models.AgentRolePrompt{
		ID:      uuid.New(),
		Role:    role,
		Content: content,
	}
}

func (r *memRolePromptRepo) GetByRole(_ context.Context, role string) (*models.AgentRolePrompt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byRole[role]
	if !ok {
		return nil, repository.ErrAgentRolePromptNotFound
	}
	cp := *p
	return &cp, nil
}
func (r *memRolePromptRepo) List(_ context.Context) ([]models.AgentRolePrompt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.AgentRolePrompt, 0, len(r.byRole))
	for _, p := range r.byRole {
		out = append(out, *p)
	}
	return out, nil
}
func (r *memRolePromptRepo) Upsert(_ context.Context, p *models.AgentRolePrompt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	r.byRole[p.Role] = &cp
	return nil
}

type memApiKeyRepo struct {
	mu   sync.Mutex
	keys map[uuid.UUID]*models.ApiKey
}

func newMemApiKeyRepo() *memApiKeyRepo {
	return &memApiKeyRepo{keys: map[uuid.UUID]*models.ApiKey{}}
}

func (r *memApiKeyRepo) Create(_ context.Context, k *models.ApiKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	cp := *k
	r.keys[k.ID] = &cp
	return nil
}
func (r *memApiKeyRepo) GetByKeyHash(_ context.Context, _ string) (*models.ApiKey, error) {
	return nil, repository.ErrApiKeyNotFound
}
func (r *memApiKeyRepo) GetByID(_ context.Context, id uuid.UUID) (*models.ApiKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.keys[id]
	if !ok {
		return nil, repository.ErrApiKeyNotFound
	}
	cp := *k
	return &cp, nil
}
func (r *memApiKeyRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.ApiKey
	for _, k := range r.keys {
		if k.UserID == userID {
			out = append(out, *k)
		}
	}
	return out, nil
}
func (r *memApiKeyRepo) Revoke(_ context.Context, _ uuid.UUID) error    { return nil }
func (r *memApiKeyRepo) RevokeAllForUser(_ context.Context, _ uuid.UUID) error { return nil }
func (r *memApiKeyRepo) UpdateLastUsed(_ context.Context, _ uuid.UUID) error   { return nil }
func (r *memApiKeyRepo) Delete(_ context.Context, _ uuid.UUID) error           { return nil }

func newAgentSvcWithFactories(t *testing.T) (*AgentService, *memAgentRepo, *memSecretRepo, *memApiKeyRepo) {
	t.Helper()
	agentRepo := newMemAgentRepo()
	secretRepo := newMemSecretRepo()
	apiKeyRepo := newMemApiKeyRepo()
	rolePromptRepo := newMemRolePromptRepo()
	rolePromptRepo.seed(string(models.AgentRoleAssistant), "You are the assistant.")
	rolePromptRepo.seed(string(models.AgentRoleOrchestrator), "You are the orchestrator.")
	rolePromptRepo.seed(string(models.AgentRoleRouter), "You are the router.")
	rolePromptRepo.seed(string(models.AgentRolePlanner), "You are the planner.")
	rolePromptRepo.seed(string(models.AgentRoleDecomposer), "You are the decomposer.")
	rolePromptRepo.seed(string(models.AgentRoleReviewer), "You are the reviewer.")
	rolePromptRepo.seed(string(models.AgentRoleDeveloper), "You are the developer.")
	rolePromptRepo.seed(string(models.AgentRoleTester), "You are the tester.")
	rolePromptRepo.seed(string(models.AgentRoleMerger), "You are the merger.")

	svc := NewAgentService(agentRepo, secretRepo, makeAESEncryptor(t), newMemTxManager())
	svc.WithRolePromptRepo(rolePromptRepo).WithApiKeyRepo(apiKeyRepo)
	return svc, agentRepo, secretRepo, apiKeyRepo
}

func TestAgentService_CreateDefaultAssistant_HappyPath(t *testing.T) {
	svc, agentRepo, secretRepo, apiKeyRepo := newAgentSvcWithFactories(t)
	userID := uuid.New()

	if err := svc.CreateDefaultAssistant(context.Background(), userID); err != nil {
		t.Fatalf("CreateDefaultAssistant: %v", err)
	}

	// Agent created with correct attributes.
	agents, _, _ := agentRepo.List(context.Background(), repository.AgentFilter{UserID: &userID})
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	a := agents[0]
	if a.Name != "assistant" {
		t.Errorf("name=%q, want assistant", a.Name)
	}
	if a.Role != models.AgentRoleAssistant {
		t.Errorf("role=%q, want assistant", a.Role)
	}
	if a.UserID == nil || *a.UserID != userID {
		t.Errorf("user_id mismatch")
	}
	if a.Model != nil {
		t.Errorf("model should be nil (unconfigured), got %v", *a.Model)
	}

	// Scoped MCP key created.
	keys, _ := apiKeyRepo.ListByUserID(context.Background(), userID)
	if len(keys) != 1 {
		t.Fatalf("expected 1 api key, got %d", len(keys))
	}
	if keys[0].Scopes != `"mcp"` {
		t.Errorf("scopes=%q, want %q", keys[0].Scopes, `"mcp"`)
	}

	// Encrypted secret DEVTEAM_MCP_TOKEN created.
	secrets, _ := secretRepo.ListByAgentID(context.Background(), a.ID)
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if secrets[0].KeyName != "DEVTEAM_MCP_TOKEN" {
		t.Errorf("key_name=%q, want DEVTEAM_MCP_TOKEN", secrets[0].KeyName)
	}
}

func TestAgentService_CreateDefaultAssistant_PromptCopied(t *testing.T) {
	svc, agentRepo, _, _ := newAgentSvcWithFactories(t)
	userID := uuid.New()

	if err := svc.CreateDefaultAssistant(context.Background(), userID); err != nil {
		t.Fatalf("CreateDefaultAssistant: %v", err)
	}

	a, err := agentRepo.GetByName(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if a.SystemPrompt == nil || *a.SystemPrompt != "You are the assistant." {
		t.Errorf("system_prompt not copied from role_prompts")
	}
}

func TestAgentService_CreateDefaultProjectAgents_HappyPath(t *testing.T) {
	svc, agentRepo, _, _ := newAgentSvcWithFactories(t)
	teamID := uuid.New()

	if err := svc.CreateDefaultProjectAgents(context.Background(), teamID, "development"); err != nil {
		t.Fatalf("CreateDefaultProjectAgents: %v", err)
	}

	agents, total, _ := agentRepo.List(context.Background(), repository.AgentFilter{TeamID: &teamID})
	if total != 7 {
		t.Fatalf("expected 7 agents, got %d", total)
	}

	roles := map[models.AgentRole]bool{}
	for _, a := range agents {
		roles[a.Role] = true
		if a.TeamID == nil || *a.TeamID != teamID {
			t.Errorf("agent %s: team_id mismatch", a.Name)
		}
		if a.SystemPrompt == nil || *a.SystemPrompt == "" {
			t.Errorf("agent %s: system_prompt should be set", a.Name)
		}
		// model should only be non-nil for router, planner, decomposer (reviewer is now sandbox)
		switch a.Role {
		case models.AgentRoleRouter, models.AgentRolePlanner, models.AgentRoleDecomposer:
			if a.Model == nil || *a.Model == "" {
				t.Errorf("agent %s: expected configured model, got nil", a.Name)
			}
		default:
			if a.Model != nil {
				t.Errorf("agent %s: expected unconfigured model, got %v", a.Name, *a.Model)
			}
		}
	}
	expectedRoles := []models.AgentRole{
		models.AgentRoleRouter,
		models.AgentRolePlanner,
		models.AgentRoleDecomposer,
		models.AgentRoleReviewer,
		models.AgentRoleDeveloper,
		models.AgentRoleTester,
		models.AgentRoleMerger,
	}
	for _, r := range expectedRoles {
		if !roles[r] {
			t.Errorf("role %s not created", r)
		}
	}
	// orchestrator — это Go-движок, отдельный LLM-агент не создаётся.
	if roles[models.AgentRoleOrchestrator] {
		t.Errorf("orchestrator agent must NOT be created (it is a Go engine, not an LLM)")
	}
}

func TestAgentService_CreateDefaultProjectAgents_NonDevelopmentTeam(t *testing.T) {
	svc, agentRepo, _, _ := newAgentSvcWithFactories(t)
	teamID := uuid.New()

	if err := svc.CreateDefaultProjectAgents(context.Background(), teamID, "marketing"); err != nil {
		t.Fatalf("CreateDefaultProjectAgents: %v", err)
	}

	agents, total, _ := agentRepo.List(context.Background(), repository.AgentFilter{TeamID: &teamID})
	if total != 1 {
		t.Fatalf("expected 1 agent, got %d", total)
	}

	roles := map[models.AgentRole]bool{}
	for _, a := range agents {
		roles[a.Role] = true
		if a.TeamID == nil || *a.TeamID != teamID {
			t.Errorf("agent %s: team_id mismatch", a.Name)
		}
		if a.SystemPrompt == nil || *a.SystemPrompt == "" {
			t.Errorf("agent %s: system_prompt should be set", a.Name)
		}
	}

	expectedRoles := []models.AgentRole{
		models.AgentRoleRouter,
	}
	for _, r := range expectedRoles {
		if !roles[r] {
			t.Errorf("role %s not created", r)
		}
	}

	unexpectedRoles := []models.AgentRole{
		models.AgentRoleOrchestrator,
		models.AgentRolePlanner,
		models.AgentRoleDecomposer,
		models.AgentRoleReviewer,
		models.AgentRoleDeveloper,
		models.AgentRoleTester,
		models.AgentRoleMerger,
	}
	for _, r := range unexpectedRoles {
		if roles[r] {
			t.Errorf("role %s should not be created for non-development team", r)
		}
	}
}

func TestAgentService_CreateDefaultAssistant_MissingPrompt(t *testing.T) {
	agentRepo := newMemAgentRepo()
	secretRepo := newMemSecretRepo()
	emptyPromptRepo := newMemRolePromptRepo() // no prompts seeded

	svc := NewAgentService(agentRepo, secretRepo, makeAESEncryptor(t), newMemTxManager())
	svc.WithRolePromptRepo(emptyPromptRepo).WithApiKeyRepo(newMemApiKeyRepo())

	err := svc.CreateDefaultAssistant(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if !strings.Contains(err.Error(), "default prompt for assistant") {
		t.Errorf("error should mention missing prompt, got: %v", err)
	}
}

func TestAgentService_CreateDefaultAssistant_NoRolePromptRepo(t *testing.T) {
	svc := newAgentSvcForTest(t) // no rolePromptRepo set
	err := svc.CreateDefaultAssistant(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rolePromptRepo") {
		t.Errorf("error should mention rolePromptRepo, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 4 §4.1 — InternalMCPEnabled update
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentService_Update_InternalMCPEnabled(t *testing.T) {
	svc := newAgentSvcForTest(t)
	model := "claude-sonnet-4-6"
	created, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "assistant-mcp-test",
		Role:          models.AgentRoleAssistant,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &model,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.InternalMCPEnabled {
		t.Fatal("InternalMCPEnabled should default to false")
	}

	enable := true
	updated, err := svc.Update(context.Background(), created.ID, UpdateAgentInput{
		InternalMCPEnabled: &enable,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.InternalMCPEnabled {
		t.Error("InternalMCPEnabled should be true after update")
	}

	disable := false
	updated2, err := svc.Update(context.Background(), updated.ID, UpdateAgentInput{
		InternalMCPEnabled: &disable,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated2.InternalMCPEnabled {
		t.Error("InternalMCPEnabled should be false after second update")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 4 §4.3 — Provider validation
// ─────────────────────────────────────────────────────────────────────────────

type agentMemLlmCredRepo struct {
	mu    sync.Mutex
	creds map[string]*models.UserLlmCredential // key: "userID:provider"
}

func newAgentMemLlmCredRepo() *agentMemLlmCredRepo {
	return &agentMemLlmCredRepo{creds: map[string]*models.UserLlmCredential{}}
}

func (r *agentMemLlmCredRepo) seed(userID uuid.UUID, provider models.UserLLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := userID.String() + ":" + string(provider)
	r.creds[key] = &models.UserLlmCredential{
		ID:       uuid.New(),
		UserID:   userID,
		Provider: provider,
	}
}

func (r *agentMemLlmCredRepo) GetByUserAndProvider(_ context.Context, userID uuid.UUID, provider models.UserLLMProvider) (*models.UserLlmCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := userID.String() + ":" + string(provider)
	c, ok := r.creds[key]
	if !ok {
		return nil, repository.ErrUserLlmCredentialNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *agentMemLlmCredRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]models.UserLlmCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.UserLlmCredential
	for _, c := range r.creds {
		if c.UserID == userID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *agentMemLlmCredRepo) Create(_ context.Context, _ *models.UserLlmCredential) error { return nil }
func (r *agentMemLlmCredRepo) Update(_ context.Context, _ *models.UserLlmCredential) error { return nil }
func (r *agentMemLlmCredRepo) DeleteByUserAndProvider(_ context.Context, _ uuid.UUID, _ models.UserLLMProvider) (int64, error) {
	return 0, nil
}
func (r *agentMemLlmCredRepo) CreateAudit(_ context.Context, _ *models.UserLlmCredentialAudit) error {
	return nil
}

func TestAgentService_ValidateProviderConnected_HappyPath(t *testing.T) {
	llmCredRepo := newAgentMemLlmCredRepo()
	svc := newAgentSvcForTest(t)
	svc.WithLlmCredRepo(llmCredRepo)

	userID := uuid.New()
	llmCredRepo.seed(userID, models.UserLLMProviderAnthropic)

	pk := models.AgentProviderKindAnthropic
	if err := svc.ValidateProviderConnected(context.Background(), userID, &pk); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestAgentService_ValidateProviderConnected_NotConnected(t *testing.T) {
	llmCredRepo := newAgentMemLlmCredRepo()
	svc := newAgentSvcForTest(t)
	svc.WithLlmCredRepo(llmCredRepo)

	userID := uuid.New()
	// NOT seeding any credentials

	pk := models.AgentProviderKindAnthropic
	err := svc.ValidateProviderConnected(context.Background(), userID, &pk)
	if !errors.Is(err, ErrAgentProviderNotConnected) {
		t.Fatalf("expected ErrAgentProviderNotConnected, got %v", err)
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention 'not connected', got: %v", err)
	}
}

func TestAgentService_ValidateProviderConnected_NilProvider(t *testing.T) {
	svc := newAgentSvcForTest(t)
	if err := svc.ValidateProviderConnected(context.Background(), uuid.New(), nil); err != nil {
		t.Fatalf("nil provider should be OK, got %v", err)
	}
}

func TestAgentService_ValidateProviderConnected_AnthropicOAuth(t *testing.T) {
	svc := newAgentSvcForTest(t)
	pk := models.AgentProviderKindAnthropicOAuth
	if err := svc.ValidateProviderConnected(context.Background(), uuid.New(), &pk); err != nil {
		t.Fatalf("anthropic_oauth should skip validation, got %v", err)
	}
}

func TestAgentService_ValidateProviderConnected_NilRepo(t *testing.T) {
	svc := newAgentSvcForTest(t)
	// llmCredRepo not set — should be no-op
	pk := models.AgentProviderKindAnthropic
	if err := svc.ValidateProviderConnected(context.Background(), uuid.New(), &pk); err != nil {
		t.Fatalf("nil repo should be no-op, got %v", err)
	}
}
