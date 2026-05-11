//go:build integration

package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testLlmEncryptionKeyHex(t *testing.T) []byte {
	t.Helper()
	k, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	require.NoError(t, err)
	require.Len(t, k, 32)
	return k
}

func TestUserLlmCredentialServiceIntegration_IsolationAB(t *testing.T) {
	db := repositorySetupTestDB(t)
	userA := repositoryCreateProjectTestUser(t, db, "llm-a-"+uuid.NewString()+"@example.com")
	userB := repositoryCreateProjectTestUser(t, db, "llm-b-"+uuid.NewString()+"@example.com")
	defer cleanupLLMIntegrationUsers(t, db, userA.ID, userB.ID)

	enc, err := crypto.NewAESEncryptor(testLlmEncryptionKeyHex(t))
	require.NoError(t, err)
	repo := repository.NewUserLlmCredentialRepository(db)
	txm := repository.NewTransactionManager(db)
	svc := NewUserLlmCredentialService(repo, txm, enc, nil)

	keyA := "12345678901234567890aaaaaaaa"
	req, err := dto.DecodePatchLlmCredentialsJSON([]byte(`{"openai_api_key":"` + keyA + `"}`))
	require.NoError(t, err)
	_, err = svc.Patch(context.Background(), userA.ID, req, "127.0.0.1", "t")
	require.NoError(t, err)

	var auditCount int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM user_llm_credential_audit WHERE user_id = ?`, userA.ID).Scan(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount)

	outB, err := svc.GetMasked(context.Background(), userB.ID)
	require.NoError(t, err)
	assert.Nil(t, outB.OpenAI.MaskedPreview)

	outA, err := svc.GetMasked(context.Background(), userA.ID)
	require.NoError(t, err)
	require.NotNil(t, outA.OpenAI.MaskedPreview)
	assert.Contains(t, *outA.OpenAI.MaskedPreview, "aaaa")
	b, err := json.Marshal(outA)
	require.NoError(t, err)
	assert.NotContains(t, string(b), keyA)
}

func TestUserLlmCredentialServiceIntegration_DecryptFailsAfterBlobSwap(t *testing.T) {
	db := repositorySetupTestDB(t)
	user := repositoryCreateProjectTestUser(t, db, "llm-swap-"+uuid.NewString()+"@example.com")
	defer cleanupLLMIntegrationUsers(t, db, user.ID)

	enc, err := crypto.NewAESEncryptor(testLlmEncryptionKeyHex(t))
	require.NoError(t, err)
	repo := repository.NewUserLlmCredentialRepository(db)
	txm := repository.NewTransactionManager(db)
	svc := NewUserLlmCredentialService(repo, txm, enc, nil)

	r1, err := dto.DecodePatchLlmCredentialsJSON([]byte(`{"openai_api_key":"12345678901234567890openai__"}`))
	require.NoError(t, err)
	_, err = svc.Patch(context.Background(), user.ID, r1, "", "")
	require.NoError(t, err)
	r2, err := dto.DecodePatchLlmCredentialsJSON([]byte(`{"anthropic_api_key":"12345678901234567890anthropic"}`))
	require.NoError(t, err)
	_, err = svc.Patch(context.Background(), user.ID, r2, "", "")
	require.NoError(t, err)

	rows, err := repo.ListByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	iOpen, iAnth := 0, 1
	if rows[0].Provider != models.UserLLMProviderOpenAI {
		iOpen, iAnth = 1, 0
	}
	tmp := append([]byte(nil), rows[iOpen].EncryptedKey...)
	rows[iOpen].EncryptedKey = append([]byte(nil), rows[iAnth].EncryptedKey...)
	rows[iAnth].EncryptedKey = tmp
	require.NoError(t, repo.Update(context.Background(), &rows[iOpen]))
	require.NoError(t, repo.Update(context.Background(), &rows[iAnth]))

	_, err = svc.GetMasked(context.Background(), user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func cleanupLLMIntegrationUsers(t *testing.T, db *gorm.DB, ids ...uuid.UUID) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, db.Exec(`DELETE FROM user_llm_credential_audit WHERE user_id = ?`, id).Error)
		require.NoError(t, db.Exec(`DELETE FROM user_llm_credentials WHERE user_id = ?`, id).Error)
		require.NoError(t, db.Exec(`DELETE FROM users WHERE id = ?`, id).Error)
	}
}

func repositorySetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	return db
}

func repositoryCreateProjectTestUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	repo := repository.NewUserRepository(db)
	u := &models.User{
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}
