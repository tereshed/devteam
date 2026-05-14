//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Sprint 16.C-2 — happy-path DatabaseSecretResolver на настоящем Postgres.
// Юнит-уровень покрывает только guard'ы (см. secret_resolver_test.go); тут —
// что Resolve реально находит ключ по (project.UserID, provider) и
// расшифровывает его тем же AAD, что и UserLlmCredentialService.

func TestDatabaseSecretResolverIntegration_HappyPath(t *testing.T) {
	db := repositorySetupTestDB(t)
	user := repositoryCreateProjectTestUser(t, db, "secret-resolver-"+uuid.NewString()+"@example.com")
	defer cleanupLLMIntegrationUsers(t, db, user.ID)

	enc, err := crypto.NewAESEncryptor(testLlmEncryptionKeyHex(t))
	require.NoError(t, err)

	// 1) Сохраняем openrouter-ключ через сервис (как обычно делает пользователь
	//    через PATCH /me/llm-credentials).
	repo := repository.NewUserLlmCredentialRepository(db)
	txm := repository.NewTransactionManager(db)
	svc := NewUserLlmCredentialService(repo, txm, enc, nil)
	patch, err := dto.DecodePatchLlmCredentialsJSON([]byte(`{"openrouter_api_key":"sk-or-test-key-1234567890"}`))
	require.NoError(t, err)
	_, err = svc.Patch(context.Background(), user.ID, patch, "127.0.0.1", "test")
	require.NoError(t, err)

	// 2) Резолвер должен достать тот же plaintext через project.UserID.
	resolver := NewDatabaseSecretResolver(db, enc)
	project := &models.Project{UserID: user.ID}
	got, err := resolver.Resolve(context.Background(), project, "openrouter")
	require.NoError(t, err)
	require.Equal(t, "sk-or-test-key-1234567890", got)

	// 3) Изоляция владельца: чужой project.UserID — ErrSecretNotFound.
	otherUser := repositoryCreateProjectTestUser(t, db, "secret-other-"+uuid.NewString()+"@example.com")
	defer cleanupLLMIntegrationUsers(t, db, otherUser.ID)
	_, err = resolver.Resolve(context.Background(), &models.Project{UserID: otherUser.ID}, "openrouter")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound for foreign user, got %v", err)
	}
}
