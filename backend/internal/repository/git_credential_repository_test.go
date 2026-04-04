//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitCredentialRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-create@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	enc := []byte{0x01, 0x02, 0xab}
	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: enc,
		Label:          "work",
	}
	require.NoError(t, repo.Create(ctx, cred))
	assert.NotEqual(t, uuid.Nil, cred.ID)

	got, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.UserID)
	assert.Equal(t, models.GitCredentialProviderGitHub, got.Provider)
	assert.Equal(t, models.GitCredentialAuthToken, got.AuthType)
	assert.Equal(t, enc, got.EncryptedValue)
	assert.Equal(t, "work", got.Label)
}

func TestGitCredentialRepository_Create_InvalidProvider(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-badprov@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProvider("azure"),
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("x"),
		Label:          "bad",
	}
	err := repo.Create(ctx, cred)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23514", pgErr.Code)
}

func TestGitCredentialRepository_Create_InvalidAuthType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-badauth@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthType("basic"),
		EncryptedValue: []byte("x"),
		Label:          "bad",
	}
	err := repo.Create(ctx, cred)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23514", pgErr.Code)
}

func TestGitCredentialRepository_Create_InvalidUserID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         uuid.New(),
		Provider:       models.GitCredentialProviderGitLab,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("x"),
		Label:          "orphan",
	}
	err := repo.Create(ctx, cred)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23503", pgErr.Code)
}

func TestGitCredentialRepository_GetByID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-get@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	secret := []byte("cipher-blob")
	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderBitbucket,
		AuthType:       models.GitCredentialAuthSSHKey,
		EncryptedValue: secret,
		Label:          "bb",
	}
	require.NoError(t, repo.Create(ctx, cred))

	got, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	assert.Equal(t, secret, got.EncryptedValue)
}

func TestGitCredentialRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, err := NewGitCredentialRepository(db).GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrGitCredentialNotFound)
}

func TestGitCredentialRepository_ListByUserID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-list3@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	for i, prov := range []models.GitCredentialProvider{
		models.GitCredentialProviderGitHub,
		models.GitCredentialProviderGitLab,
		models.GitCredentialProviderBitbucket,
	} {
		c := &models.GitCredential{
			UserID:         user.ID,
			Provider:       prov,
			AuthType:       models.GitCredentialAuthToken,
			EncryptedValue: []byte{byte(i)},
			Label:          string(rune('a' + i)),
		}
		require.NoError(t, repo.Create(ctx, c))
	}

	list, err := repo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, list, 3)
}

func TestGitCredentialRepository_ListByUserID_NoEncryptedValue(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-nosec@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("super-secret"),
		Label:          "a",
	}))

	list, err := repo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].EncryptedValue)
}

func TestGitCredentialRepository_ListByUserID_Isolation(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	u1 := createProjectTestUser(t, db, "gc-u1@"+uuid.NewString()+".example.com")
	u2 := createProjectTestUser(t, db, "gc-u2@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	for _, u := range []*models.User{u1, u2} {
		require.NoError(t, repo.Create(ctx, &models.GitCredential{
			UserID:         u.ID,
			Provider:       models.GitCredentialProviderGitHub,
			AuthType:       models.GitCredentialAuthToken,
			EncryptedValue: []byte("x"),
			Label:          u.Email,
		}))
	}

	list1, err := repo.ListByUserID(ctx, u1.ID)
	require.NoError(t, err)
	require.Len(t, list1, 1)
	assert.Equal(t, u1.ID, list1[0].UserID)

	list2, err := repo.ListByUserID(ctx, u2.ID)
	require.NoError(t, err)
	require.Len(t, list2, 1)
	assert.Equal(t, u2.ID, list2[0].UserID)
}

func TestGitCredentialRepository_ListByUserIDAndProvider(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-filter@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("1"),
		Label:          "gh1",
	}))
	require.NoError(t, repo.Create(ctx, &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthOAuth,
		EncryptedValue: []byte("2"),
		Label:          "gh2",
	}))
	require.NoError(t, repo.Create(ctx, &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitLab,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("3"),
		Label:          "gl",
	}))

	gh, err := repo.ListByUserIDAndProvider(ctx, user.ID, models.GitCredentialProviderGitHub)
	require.NoError(t, err)
	require.Len(t, gh, 2)
	for _, c := range gh {
		assert.Equal(t, models.GitCredentialProviderGitHub, c.Provider)
		assert.Empty(t, c.EncryptedValue)
	}
}

func TestGitCredentialRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-upd@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("old"),
		Label:          "before",
	}
	require.NoError(t, repo.Create(ctx, cred))

	full, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	before := full.UpdatedAt
	time.Sleep(5 * time.Millisecond)

	full.Label = "after"
	require.NoError(t, repo.Update(ctx, full))

	got, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	assert.Equal(t, "after", got.Label)
	assert.True(t, got.UpdatedAt.After(before) || !got.UpdatedAt.Equal(before))
}

func TestGitCredentialRepository_Update_RotateToken(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-rot@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("token-v1"),
		Label:          "rot",
	}
	require.NoError(t, repo.Create(ctx, cred))

	full, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	full.EncryptedValue = []byte("token-v2")
	require.NoError(t, repo.Update(ctx, full))

	got, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	assert.Equal(t, []byte("token-v2"), got.EncryptedValue)
}

func TestGitCredentialRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-del@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("x"),
		Label:          "gone",
	}
	require.NoError(t, repo.Create(ctx, cred))

	require.NoError(t, repo.Delete(ctx, cred.ID))
	_, err := repo.GetByID(ctx, cred.ID)
	assert.ErrorIs(t, err, ErrGitCredentialNotFound)
}

func TestGitCredentialRepository_Delete_ProjectSetNull(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-null@"+uuid.NewString()+".example.com")
	gcRepo := NewGitCredentialRepository(db)
	projRepo := NewProjectRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("k"),
		Label:          "proj-cred",
	}
	require.NoError(t, gcRepo.Create(ctx, cred))

	p := &models.Project{
		Name:             "with-git-cred",
		GitProvider:      models.GitProviderGitHub,
		UserID:           user.ID,
		Status:           models.ProjectStatusActive,
		GitCredentialsID: &cred.ID,
	}
	require.NoError(t, projRepo.Create(ctx, p))

	require.NoError(t, gcRepo.Delete(ctx, cred.ID))

	var row models.Project
	require.NoError(t, db.WithContext(ctx).Where("id = ?", p.ID).First(&row).Error)
	assert.Nil(t, row.GitCredentialsID)
}

func TestGitCredentialRepository_Cascade_UserDelete(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "gc-cascade@"+uuid.NewString()+".example.com")
	repo := NewGitCredentialRepository(db)
	ctx := context.Background()

	cred := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("x"),
		Label:          "cascade",
	}
	require.NoError(t, repo.Create(ctx, cred))
	id := cred.ID

	// Физическое удаление: у User soft-delete, иначе CASCADE из миграции не сработает.
	require.NoError(t, db.Unscoped().WithContext(ctx).Delete(&models.User{}, "id = ?", user.ID).Error)

	_, err := repo.GetByID(ctx, id)
	assert.ErrorIs(t, err, ErrGitCredentialNotFound)
}
