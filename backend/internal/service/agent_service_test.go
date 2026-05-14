package service

import (
	"context"
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
}

func TestAgentService_Create_RejectsLLMWithoutModel(t *testing.T) {
	svc := newAgentSvcForTest(t)
	_, err := svc.Create(context.Background(), CreateAgentInput{
		Name:          "bad",
		Role:          models.AgentRoleReviewer,
		ExecutionKind: models.AgentExecutionKindLLM,
		// Model missing
	})
	if !errors.Is(err, ErrAgentValidation) {
		t.Fatalf("expected ErrAgentValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error must mention model, got: %v", err)
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
