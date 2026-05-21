package service

import (
	"context"
	"testing"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/llm/factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAssistantLLMClientAdapter_ResolveAssistantClient(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	f := factory.New()

	t.Run("successful resolve", func(t *testing.T) {
		prov := models.AgentProviderKindOpenRouter
		agent := &models.Agent{ProviderKind: &prov}
		
		credsSvc := &assistantMockUserLlmCredentialService{
			key: "sk-or-v1-123",
			err: nil,
		}
		
		adapter := NewAssistantLLMClientAdapter(credsSvc, f, config.LLMConfig{}, nil, nil)
		client, err := adapter.ResolveAssistantClient(ctx, agent, userID)
		
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("error on missing key", func(t *testing.T) {
		prov := models.AgentProviderKindOpenRouter
		agent := &models.Agent{ProviderKind: &prov}
		
		credsSvc := &assistantMockUserLlmCredentialService{
			key: "",
			err: repository.ErrUserLlmCredentialNotFound,
		}
		
		adapter := NewAssistantLLMClientAdapter(credsSvc, f, config.LLMConfig{}, nil, nil)
		client, err := adapter.ResolveAssistantClient(ctx, agent, userID)
		
		assert.ErrorIs(t, err, ErrAssistantNotConfiguredForUser)
		assert.Nil(t, client)
	})
}
