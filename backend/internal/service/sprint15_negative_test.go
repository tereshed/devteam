package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Sprint 15.m6 — negative paths, которых не хватало в Sprint 15:
//   - OAuth: invalid_grant при refresh инвалидирует сценарий
//   - LLMProvider: empty credential для auth=api_key → ErrLLMProviderInvalid
//   - LLMProvider: дубликат имени → ErrLLMProviderNameExists через мок-репо
//   - Agent settings: foreign user получает 404-equivalent (уже покрыто authz-тестами;
//     здесь добавлен сценарий UpdateAgentSettings с невалидным CodeBackend).

// --- OAuth invalid_grant ---

func TestClaudeCodeAuth_RefreshOne_InvalidGrant(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	uid := uuid.New()
	soon := time.Now().Add(2 * time.Minute)
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return &ClaudeCodeOAuthToken{
				AccessToken: "a", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &soon,
			}, nil
		},
		refreshFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			return nil, ErrOAuthInvalidGrant
		},
	}
	svc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")
	_, err := svc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	sub, err := repo.GetByUserID(context.Background(), uid)
	require.NoError(t, err)
	err = svc.RefreshOne(context.Background(), sub)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOAuthInvalidGrant), "expected invalid_grant from provider to bubble up, got %v", err)
}

// --- LLMProvider: empty credential ---

func TestLLMProviderService_Create_RejectsEmptyCredential(t *testing.T) {
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})
	_, err := svc.Create(context.Background(), LLMProviderInput{
		Name:     "OR",
		Kind:     models.LLMProviderKindOpenRouter,
		AuthType: models.LLMProviderAuthAPIKey,
		// Credential intentionally empty
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLLMProviderInvalid)
}

func TestLLMProviderService_TestConnection_RejectsEmptyCredential(t *testing.T) {
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})
	err := svc.TestConnection(context.Background(), LLMProviderInput{
		Name: "OR", Kind: models.LLMProviderKindOpenRouter,
		AuthType: models.LLMProviderAuthAPIKey, Enabled: true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLLMProviderInvalid)
}

// --- LLMProvider: duplicate name ---

func TestLLMProviderService_Create_DuplicateName(t *testing.T) {
	repo := newMockLLMProviderRepo()
	svc := NewLLMProviderService(repo, NoopEncryptor{})
	mk := func() LLMProviderInput {
		return LLMProviderInput{
			Name:       "OpenRouter prod",
			Kind:       models.LLMProviderKindOpenRouter,
			AuthType:   models.LLMProviderAuthAPIKey,
			Credential: "k",
			Enabled:    true,
		}
	}
	_, err := svc.Create(context.Background(), mk())
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), mk())
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLLMProviderNameExists)
}

// --- Agent settings: invalid code_backend ---

func TestAgentSettings_Update_InvalidCodeBackend(t *testing.T) {
	tr := new(mockTeamRepo)
	owner := uuid.New()
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).Return(owner, nil)
	tr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID}, nil)

	svc := NewTeamService(tr, new(mockToolDefRepo))
	bogus := "not-a-real-backend"
	_, err := svc.UpdateAgentSettings(context.Background(),
		AgentSettingsActor{UserID: owner}, agentID,
		dto.UpdateAgentSettingsRequest{CodeBackend: &bogus})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTeamAgentInvalidCodeBackend)
}

// --- Agent settings: invalid sandbox_permissions JSON ---

func TestAgentSettings_Update_RejectsInvalidPermissionsJSON(t *testing.T) {
	tr := new(mockTeamRepo)
	owner := uuid.New()
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).Return(owner, nil)
	tr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID}, nil)

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.UpdateAgentSettings(context.Background(),
		AgentSettingsActor{UserID: owner}, agentID,
		dto.UpdateAgentSettingsRequest{
			SandboxPermissions: []byte(`{"allow":["Bash(rm -rf /:*)"],"defaultMode":"acceptEdits"}`),
		})
	require.Error(t, err, "Bash(rm -rf /:*) с / в подкоманде должно отвергаться (Sprint 15.M1)")
}
