//go:build integration

package repository

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func gitIntegrationTestKey(t *testing.T) []byte {
	t.Helper()
	k, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	require.NoError(t, err)
	require.Len(t, k, 32)
	return k
}

func gitIntegrationSetupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	return db
}

func gitIntegrationCreateUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	repo := NewUserRepository(db)
	u := &models.User{
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func cleanupGitIntegration(t *testing.T, db *gorm.DB, userIDs ...uuid.UUID) {
	t.Helper()
	for _, id := range userIDs {
		require.NoError(t, db.Exec(`DELETE FROM git_integration_credentials WHERE user_id = ?`, id).Error)
		require.NoError(t, db.Exec(`DELETE FROM users WHERE id = ?`, id).Error)
	}
}

func TestGitIntegrationCredentialRepository_RoundTrip(t *testing.T) {
	db := gitIntegrationSetupDB(t)
	user := gitIntegrationCreateUser(t, db, "git-rt-"+uuid.NewString()+"@example.com")
	defer cleanupGitIntegration(t, db, user.ID)

	enc, err := crypto.NewAESEncryptor(gitIntegrationTestKey(t))
	require.NoError(t, err)
	repo := NewGitIntegrationCredentialRepository(db)

	id := uuid.New()
	aad := GitIntegrationCredentialAAD(id)
	access, err := enc.Encrypt([]byte("ghp_secret_access_token"), aad)
	require.NoError(t, err)
	refresh, err := enc.Encrypt([]byte("ghr_secret_refresh"), aad)
	require.NoError(t, err)

	exp := time.Now().Add(8 * time.Hour).UTC()
	cred := &models.GitIntegrationCredential{
		ID:              id,
		UserID:          user.ID,
		Provider:        models.GitIntegrationProviderGitHub,
		AccessTokenEnc:  access,
		RefreshTokenEnc: refresh,
		TokenType:       "Bearer",
		Scopes:          "repo,read:user",
		AccountLogin:    "octocat",
		ExpiresAt:       &exp,
	}
	require.NoError(t, repo.Upsert(context.Background(), cred))

	got, err := repo.GetByUserAndProvider(context.Background(), user.ID, models.GitIntegrationProviderGitHub)
	require.NoError(t, err)
	assert.Equal(t, "octocat", got.AccountLogin)
	assert.Equal(t, "Bearer", got.TokenType)

	plain, err := enc.Decrypt(got.AccessTokenEnc, GitIntegrationCredentialAAD(got.ID))
	require.NoError(t, err)
	assert.Equal(t, "ghp_secret_access_token", string(plain))

	// Подменяем blob другой записи (id2) → расшифровка под AAD(id1) падает.
	id2 := uuid.New()
	otherBlob, err := enc.Encrypt([]byte("foreign"), GitIntegrationCredentialAAD(id2))
	require.NoError(t, err)
	_, err = enc.Decrypt(otherBlob, GitIntegrationCredentialAAD(got.ID))
	assert.Error(t, err, "cross-row substitution must fail with GCM tag mismatch")

	// Delete.
	require.NoError(t, repo.DeleteByUserAndProvider(context.Background(), user.ID, models.GitIntegrationProviderGitHub))
	_, err = repo.GetByUserAndProvider(context.Background(), user.ID, models.GitIntegrationProviderGitHub)
	assert.ErrorIs(t, err, ErrGitIntegrationNotFound)
}

func TestGitIntegrationCredentialRepository_UpsertUpdatesWithNewID(t *testing.T) {
	db := gitIntegrationSetupDB(t)
	user := gitIntegrationCreateUser(t, db, "git-up-"+uuid.NewString()+"@example.com")
	defer cleanupGitIntegration(t, db, user.ID)

	enc, err := crypto.NewAESEncryptor(gitIntegrationTestKey(t))
	require.NoError(t, err)
	repo := NewGitIntegrationCredentialRepository(db)

	id1 := uuid.New()
	access1, _ := enc.Encrypt([]byte("v1"), GitIntegrationCredentialAAD(id1))
	require.NoError(t, repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             id1,
		UserID:         user.ID,
		Provider:       models.GitIntegrationProviderGitLab,
		AccessTokenEnc: access1,
		AccountLogin:   "user-v1",
	}))

	// Сервис при HandleCallback каждый раз генерирует новый UUID и шифрует токены под него:
	// AAD = новый id. Repository обязан перезаписать id столбца под этот же blob — иначе
	// при последующей расшифровке AAD не сойдётся и GCM сломается. Это regression-тест
	// на review-замечание про FirstOrCreate vs OnConflict.
	id2 := uuid.New()
	access2, _ := enc.Encrypt([]byte("v2"), GitIntegrationCredentialAAD(id2))
	require.NoError(t, repo.Upsert(context.Background(), &models.GitIntegrationCredential{
		ID:             id2,
		UserID:         user.ID,
		Provider:       models.GitIntegrationProviderGitLab,
		AccessTokenEnc: access2,
		AccountLogin:   "user-v2",
	}))

	got, err := repo.GetByUserAndProvider(context.Background(), user.ID, models.GitIntegrationProviderGitLab)
	require.NoError(t, err)
	assert.Equal(t, id2, got.ID, "id must be rewritten on conflict so AAD matches new ciphertext")
	assert.Equal(t, "user-v2", got.AccountLogin)
	plain, err := enc.Decrypt(got.AccessTokenEnc, GitIntegrationCredentialAAD(got.ID))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(plain))

	// На одного пользователя/провайдера в БД ровно одна строка (UNIQUE).
	var count int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM git_integration_credentials WHERE user_id = ? AND provider = 'gitlab'`, user.ID).Scan(&count).Error)
	assert.Equal(t, int64(1), count)
}
