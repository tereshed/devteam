package service

import (
	"context"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type mockAgentLoader struct {
	agent *models.Agent
	err   error
}

func (m *mockAgentLoader) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	return m.agent, m.err
}

func (m *mockAgentLoader) GetAgentByUserRole(ctx context.Context, userID uuid.UUID, role string) (*models.Agent, error) {
	return m.agent, m.err
}

func (m *mockAgentLoader) UpdateAgentProvider(ctx context.Context, agentID uuid.UUID, providerKind models.AgentProviderKind, model string) error {
	if m.agent != nil && m.agent.ID == agentID {
		m.agent.ProviderKind = &providerKind
		m.agent.Model = &model
	}
	return m.err
}

type mockAgentCreator struct {
	calledWith []uuid.UUID
	err        error
	onCall     func(userID uuid.UUID)
}

func (m *mockAgentCreator) CreateDefaultAssistant(ctx context.Context, userID uuid.UUID) error {
	m.calledWith = append(m.calledWith, userID)
	if m.onCall != nil {
		m.onCall(userID)
	}
	return m.err
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

	t.Run("auto-provisioning missing agent", func(t *testing.T) {
		prov := models.AgentProviderKindOpenRouter
		agentToProvision := &models.Agent{
			ID:           uuid.New(),
			IsActive:     true,
			ProviderKind: &prov,
		}

		loader := &mockAgentLoader{
			agent: nil,
			err:   gorm.ErrRecordNotFound,
		}

		creator := &mockAgentCreator{
			onCall: func(u uuid.UUID) {
				loader.agent = agentToProvision
				loader.err = nil
			},
		}

		svc := &assistantService{
			deps: AssistantServiceDeps{
				AgentLoader:  loader,
				AgentCreator: creator,
				UserCreds: &assistantMockUserLlmCredentialService{
					key: "some-key",
					err: nil,
				},
			},
		}

		status, err := svc.GetStatus(ctx, userID)
		assert.NoError(t, err)
		assert.True(t, status.IsConfigured)
		assert.Equal(t, "openrouter", status.RequiredProvider)
		assert.Len(t, creator.calledWith, 1)
		assert.Equal(t, userID, creator.calledWith[0])
	})
}
