package service

import (
	"context"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockAgentLoader struct {
	agent *models.Agent
	err   error
}

func (m *mockAgentLoader) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	return m.agent, m.err
}

type assistantMockUserLlmCredentialService struct {
	key string
	err error
}

func (m *assistantMockUserLlmCredentialService) GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error) {
	return m.key, m.err
}
func (m *assistantMockUserLlmCredentialService) GetMasked(ctx context.Context, userID uuid.UUID) (*dto.LlmCredentialsResponse, error) {
	return nil, nil
}
func (m *assistantMockUserLlmCredentialService) Patch(ctx context.Context, userID uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, userAgent string) (*dto.LlmCredentialsResponse, error) {
	return nil, nil
}

func TestAssistantService_GetStatus(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("key exists", func(t *testing.T) {
		prov := models.AgentProviderKindAnthropic
		svc := &assistantService{
			deps: AssistantServiceDeps{
				AgentLoader: &mockAgentLoader{
					agent: &models.Agent{IsActive: true, ProviderKind: &prov},
				},
				UserCreds: &assistantMockUserLlmCredentialService{
					key: "sk-ant-123",
					err: nil,
				},
			},
		}
		status, err := svc.GetStatus(ctx, userID)
		assert.NoError(t, err)
		assert.True(t, status.IsConfigured)
		assert.Equal(t, "anthropic", status.RequiredProvider)
	})

	t.Run("key missing", func(t *testing.T) {
		prov := models.AgentProviderKindOpenRouter
		svc := &assistantService{
			deps: AssistantServiceDeps{
				AgentLoader: &mockAgentLoader{
					agent: &models.Agent{IsActive: true, ProviderKind: &prov},
				},
				UserCreds: &assistantMockUserLlmCredentialService{
					key: "",
					err: repository.ErrUserLlmCredentialNotFound,
				},
			},
		}
		status, err := svc.GetStatus(ctx, userID)
		assert.NoError(t, err)
		assert.False(t, status.IsConfigured)
		assert.Equal(t, "openrouter", status.RequiredProvider)
	})

	t.Run("provider does not support per-user keys", func(t *testing.T) {
		prov := models.AgentProviderKindAnthropicOAuth
		svc := &assistantService{
			deps: AssistantServiceDeps{
				AgentLoader: &mockAgentLoader{
					agent: &models.Agent{IsActive: true, ProviderKind: &prov},
				},
				UserCreds: &assistantMockUserLlmCredentialService{},
			},
		}
		status, err := svc.GetStatus(ctx, userID)
		assert.NoError(t, err)
		assert.False(t, status.IsConfigured)
		assert.Equal(t, "admin_setup_required", status.RequiredProvider)
	})
}
