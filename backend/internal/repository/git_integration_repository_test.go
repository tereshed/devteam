package repository

import (
	"errors"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
)

// TestGitIntegrationCredentialAAD_DependsOnID — гарантия: AAD = id записи.
// Защищает от cross-row substitution: одинаковый encryptor + одинаковый plaintext, но разный AAD → разный blob.
func TestGitIntegrationCredentialAAD_DependsOnID(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	a1 := GitIntegrationCredentialAAD(id1)
	a2 := GitIntegrationCredentialAAD(id2)
	if string(a1) == string(a2) {
		t.Fatalf("AAD must differ for different ids: %s vs %s", a1, a2)
	}
}

func TestGitIntegrationCredentialAAD_CrossRowSubstitutionFails(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	enc, err := crypto.NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	id1, id2 := uuid.New(), uuid.New()
	blob1, err := enc.Encrypt([]byte("ghp_token_for_row_1"), GitIntegrationCredentialAAD(id1))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Расшифровка blob1 под AAD id1 — ок.
	if _, err := enc.Decrypt(blob1, GitIntegrationCredentialAAD(id1)); err != nil {
		t.Fatalf("decrypt with correct AAD: %v", err)
	}
	// Попытка подставить blob1 в строку с id2 → GCM-tag-mismatch.
	if _, err := enc.Decrypt(blob1, GitIntegrationCredentialAAD(id2)); err == nil {
		t.Fatal("expected GCM-tag-mismatch when decrypting with foreign AAD")
	}
}

// Sanity: Upsert обязан отвергать обе аномалии: nil user_id и plaintext-blob.
func TestGitIntegrationCredentialRepository_RejectsShortBlobs(t *testing.T) {
	// Без БД: проверяем только пред-валидацию (контракт: длина блоба ≥ MinCiphertextBlobLen).
	repo := &gitIntegrationCredentialRepository{db: nil}
	cred := &models.GitIntegrationCredential{
		UserID:         uuid.New(),
		Provider:       models.GitIntegrationProviderGitHub,
		AccessTokenEnc: []byte("plain"), // явно слишком короткий
	}
	if err := repo.Upsert(nil, cred); err == nil {
		t.Fatal("expected refusal for plaintext-looking access_token_enc")
	}
}

func TestGitIntegrationCredentialRepository_RejectsInvalidInput(t *testing.T) {
	repo := &gitIntegrationCredentialRepository{db: nil}

	// nil cred.
	if err := repo.Upsert(nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil, got %v", err)
	}
	// empty user.
	if err := repo.Upsert(nil, &models.GitIntegrationCredential{Provider: models.GitIntegrationProviderGitHub}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty user, got %v", err)
	}
	// invalid provider.
	if err := repo.Upsert(nil, &models.GitIntegrationCredential{UserID: uuid.New(), Provider: "bitbucket"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid provider, got %v", err)
	}

	// nil UUID in Get.
	if _, err := repo.GetByUserAndProvider(nil, uuid.Nil, models.GitIntegrationProviderGitHub); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on nil UUID Get, got %v", err)
	}
	if err := repo.DeleteByUserAndProvider(nil, uuid.Nil, models.GitIntegrationProviderGitHub); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput on nil UUID Delete, got %v", err)
	}
}
