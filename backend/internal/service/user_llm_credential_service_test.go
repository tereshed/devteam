package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type llmNoopTxManager struct{}

func (llmNoopTxManager) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type memSnap struct {
	rows   map[uuid.UUID][]models.UserLlmCredential
	audits []models.UserLlmCredentialAudit
}

type memLlmCredRepo struct {
	mu              sync.Mutex
	rows            map[uuid.UUID][]models.UserLlmCredential
	audits          []models.UserLlmCredentialAudit
	failAuditAfterN int // при >0: ошибка на N-й успешной попытке записи аудита (1-based)
}

func newMemLlmCredRepo() *memLlmCredRepo {
	return &memLlmCredRepo{rows: make(map[uuid.UUID][]models.UserLlmCredential)}
}

func (m *memLlmCredRepo) snapshot() memSnap {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := memSnap{rows: make(map[uuid.UUID][]models.UserLlmCredential), audits: append([]models.UserLlmCredentialAudit(nil), m.audits...)}
	for k, v := range m.rows {
		snap.rows[k] = append([]models.UserLlmCredential(nil), v...)
	}
	return snap
}

func (m *memLlmCredRepo) restore(s memSnap) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = make(map[uuid.UUID][]models.UserLlmCredential)
	for k, v := range s.rows {
		m.rows[k] = append([]models.UserLlmCredential(nil), v...)
	}
	m.audits = append([]models.UserLlmCredentialAudit(nil), s.audits...)
}

type snapshotRollbackTx struct {
	repo *memLlmCredRepo
}

func (s snapshotRollbackTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	snap := s.repo.snapshot()
	err := fn(ctx)
	if err != nil {
		s.repo.restore(snap)
	}
	return err
}

func (m *memLlmCredRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.UserLlmCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.UserLlmCredential(nil), m.rows[userID]...), nil
}

func (m *memLlmCredRepo) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (*models.UserLlmCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.rows[userID] {
		if m.rows[userID][i].Provider == provider {
			cp := m.rows[userID][i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *memLlmCredRepo) Create(ctx context.Context, row *models.UserLlmCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows[row.UserID] {
		if r.Provider == row.Provider {
			return &pgconn.PgError{Code: "23505", ConstraintName: "uq_user_llm_credentials_user_provider"}
		}
	}
	m.rows[row.UserID] = append(m.rows[row.UserID], *row)
	return nil
}

func (m *memLlmCredRepo) Update(ctx context.Context, row *models.UserLlmCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sl := m.rows[row.UserID]
	for i := range sl {
		if sl[i].ID == row.ID {
			sl[i] = *row
			m.rows[row.UserID] = sl
			return nil
		}
	}
	return fmt.Errorf("%w", repository.ErrUserLlmCredentialNotUpdated)
}

func (m *memLlmCredRepo) DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sl := m.rows[userID]
	n := 0
	keep := sl[:0]
	for _, r := range sl {
		if r.Provider == provider {
			n++
			continue
		}
		keep = append(keep, r)
	}
	m.rows[userID] = keep
	return int64(n), nil
}

func (m *memLlmCredRepo) CreateAudit(ctx context.Context, row *models.UserLlmCredentialAudit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAuditAfterN > 0 && len(m.audits)+1 == m.failAuditAfterN {
		return errors.New("simulated audit failure")
	}
	m.audits = append(m.audits, *row)
	return nil
}

func mustLlmPatchReq(t *testing.T, raw string) *dto.PatchLlmCredentialsRequest {
	t.Helper()
	r, err := dto.DecodePatchLlmCredentialsJSON([]byte(raw))
	require.NoError(t, err)
	return r
}

func TestMaskAPIKey(t *testing.T) {
	assert.Equal(t, "****abcd", maskAPIKey("0123456789abcdefabcd"))
	assert.Equal(t, "********", maskAPIKey("abc"))
	rs := []rune("x你好世界y")
	s := string(rs)
	got := maskAPIKey(s)
	assert.Len(t, []rune(got), 8)
	assert.Contains(t, got, "****")
}

func TestUserLlmCredentialService_Patch_Conflict(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	svc := NewUserLlmCredentialService(newMemLlmCredRepo(), llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	k := "1234567890123456"
	tr := true
	_, err = svc.Patch(context.Background(), uid, &dto.PatchLlmCredentialsRequest{
		OpenAIAPIKey:   &k,
		ClearOpenAIKey: &tr,
	}, "127.0.0.1", "go-test")
	require.ErrorIs(t, err, ErrLlmCredentialsConflictClearAndSet)
}

func TestUserLlmCredentialService_Patch_KeyTooShort(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	svc := NewUserLlmCredentialService(newMemLlmCredRepo(), llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	short := "short"
	_, err = svc.Patch(context.Background(), uid, &dto.PatchLlmCredentialsRequest{OpenAIAPIKey: &short}, "", "")
	require.ErrorIs(t, err, ErrLlmCredentialsKeyTooShort)
}

func TestUserLlmCredentialService_Patch_KeyTooLong(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	svc := NewUserLlmCredentialService(newMemLlmCredRepo(), llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	longKey := strings.Repeat("а", llmCredMaxKeyRunes+1)
	_, err = svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"openai_api_key":`+string(mustJSONBytes(t, longKey))+`}`), "", "")
	require.ErrorIs(t, err, ErrLlmCredentialsKeyTooLong)
}

func mustJSONBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return b
}

func TestUserLlmCredentialService_Patch_ClearNoOp(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	svc := NewUserLlmCredentialService(repo, llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	tr := true
	out, err := svc.Patch(context.Background(), uid, &dto.PatchLlmCredentialsRequest{ClearOpenAIKey: &tr}, "1.1.1.1", "ua")
	require.NoError(t, err)
	assert.Nil(t, out.OpenAI.MaskedPreview)
	assert.Empty(t, repo.audits)
}

func TestUserLlmCredentialService_Patch_EmptyObject(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	svc := NewUserLlmCredentialService(newMemLlmCredRepo(), llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	out, err := svc.Patch(context.Background(), uid, &dto.PatchLlmCredentialsRequest{}, "", "")
	require.NoError(t, err)
	for _, p := range models.UserLLMProvidersOrdered {
		v := getMaskedField(out, p)
		assert.Nil(t, v)
	}
}

func getMaskedField(out *dto.LlmCredentialsResponse, p models.UserLLMProvider) *string {
	switch p {
	case models.UserLLMProviderOpenAI:
		return out.OpenAI.MaskedPreview
	case models.UserLLMProviderAnthropic:
		return out.Anthropic.MaskedPreview
	case models.UserLLMProviderGemini:
		return out.Gemini.MaskedPreview
	case models.UserLLMProviderDeepSeek:
		return out.DeepSeek.MaskedPreview
	case models.UserLLMProviderQwen:
		return out.Qwen.MaskedPreview
	case models.UserLLMProviderOpenRouter:
		return out.OpenRouter.MaskedPreview
	default:
		return nil
	}
}

func testKey32Llm(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

func TestUserLlmCredentialService_RoundTripSetAndGet(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	svc := NewUserLlmCredentialService(repo, llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	secret := "12345678901234567890abcdefghij"
	out, err := svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"anthropic_api_key":"`+secret+`"}`), "127.0.0.1", "t")
	require.NoError(t, err)
	require.NotNil(t, out.Anthropic.MaskedPreview)
	assert.Equal(t, "****ghij", *out.Anthropic.MaskedPreview)
	assert.NotContains(t, toJSONLlm(t, out), secret)

	out2, err := svc.GetMasked(context.Background(), uid)
	require.NoError(t, err)
	assert.Equal(t, *out.Anthropic.MaskedPreview, *out2.Anthropic.MaskedPreview)
}

func toJSONLlm(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestUserLlmCredentialService_TwoPatchesSameProvider(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	svc := NewUserLlmCredentialService(repo, llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	k1 := "12345678901234567890aaaa"
	k2 := "12345678901234567890bbbb"
	_, err = svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"deepseek_api_key":"`+k1+`"}`), "", "")
	require.NoError(t, err)
	out, err := svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"deepseek_api_key":"`+k2+`"}`), "", "")
	require.NoError(t, err)
	assert.Equal(t, "****bbbb", *out.DeepSeek.MaskedPreview)
	plain, err := key.Decrypt(repo.rows[uid][0].EncryptedKey, []byte(repo.rows[uid][0].ID.String()))
	require.NoError(t, err)
	assert.Equal(t, k2, string(plain))
}

func TestUserLlmCredentialService_Patch_TwoProvidersAuditFails_RollsBack(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	repo.failAuditAfterN = 2
	tx := snapshotRollbackTx{repo: repo}
	svc := NewUserLlmCredentialService(repo, tx, key, nil)
	uid := uuid.New()
	k1 := "12345678901234567890openai__"
	k2 := "12345678901234567890anthropic"
	_, err = svc.Patch(context.Background(), uid, mustLlmPatchReq(t,
		`{"openai_api_key":"`+k1+`","anthropic_api_key":"`+k2+`"}`), "", "")
	require.Error(t, err)
	assert.Empty(t, repo.rows[uid])
	assert.Empty(t, repo.audits)
}

func TestUserLlmCredentialService_Patch_LogsDoNotContainSecret(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	svc := NewUserLlmCredentialService(repo, llmNoopTxManager{}, key, h)
	uid := uuid.New()
	secret := "12345678901234567890LOGSECRET"
	_, err = svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"gemini_api_key":"`+secret+`"}`), "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), secret)
}

func TestUserLlmCredentialService_Patch_ConcurrentSetSameProvider(t *testing.T) {
	key, err := crypto.NewAESEncryptor(testKey32Llm(t))
	require.NoError(t, err)
	repo := newMemLlmCredRepo()
	svc := NewUserLlmCredentialService(repo, llmNoopTxManager{}, key, nil)
	uid := uuid.New()
	const n = 40
	var wg sync.WaitGroup
	var successes atomic.Int32
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			secret := fmt.Sprintf("1234567890123456%08d", i)
			_, err := svc.Patch(context.Background(), uid, mustLlmPatchReq(t, `{"openai_api_key":"`+secret+`"}`), "", "")
			if err == nil {
				successes.Add(1)
				return
			}
			if errors.Is(err, ErrLlmCredentialsConcurrentModify) {
				return
			}
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	require.Positive(t, successes.Load())
	require.Len(t, repo.rows[uid], 1)
}
