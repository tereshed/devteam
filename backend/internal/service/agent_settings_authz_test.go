package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Sprint 15.B (B4) regression — GetAgentSettings/UpdateAgentSettings отвергают чужого пользователя.

func TestAgentSettings_Authz_OwnerMatch(t *testing.T) {
	tr := new(mockTeamRepo)
	owner := uuid.New()
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).Return(owner, nil).Once()
	tr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID}, nil).Once()

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.GetAgentSettings(context.Background(),
		AgentSettingsActor{UserID: owner}, agentID)
	require.NoError(t, err)
}

func TestAgentSettings_Authz_DifferentOwner_Returns404Equivalent(t *testing.T) {
	tr := new(mockTeamRepo)
	owner := uuid.New()
	attacker := uuid.New()
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).Return(owner, nil)

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.GetAgentSettings(context.Background(),
		AgentSettingsActor{UserID: attacker}, agentID)
	// Сервис превращает «not your agent» в ErrTeamAgentNotFound — handler отдаёт 404
	// и не утекает существование чужого ID.
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTeamAgentNotFound),
		"foreign user must get not-found-equivalent error, got %v", err)
	// GetAgentByID / SaveAgent не должны быть вызваны.
	tr.AssertExpectations(t)
}

func TestAgentSettings_Authz_AdminBypassesOwnerCheck(t *testing.T) {
	tr := new(mockTeamRepo)
	agentID := uuid.New()
	tr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID}, nil).Once()
	// GetAgentOwnerUserID НЕ должен вызываться при admin actor.

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.GetAgentSettings(context.Background(),
		AgentSettingsActor{UserID: uuid.New(), IsAdmin: true}, agentID)
	require.NoError(t, err)
	tr.AssertExpectations(t)
}

func TestAgentSettings_Authz_Update_DifferentOwner(t *testing.T) {
	tr := new(mockTeamRepo)
	owner := uuid.New()
	attacker := uuid.New()
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).Return(owner, nil)

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.UpdateAgentSettings(context.Background(),
		AgentSettingsActor{UserID: attacker}, agentID,
		dto.UpdateAgentSettingsRequest{
			SandboxPermissions: []byte(`{"defaultMode":"bypassPermissions"}`),
		})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTeamAgentNotFound))
	tr.AssertExpectations(t) // SaveAgent не вызывался
}

func TestAgentSettings_Authz_AgentMissing_Returns404(t *testing.T) {
	tr := new(mockTeamRepo)
	agentID := uuid.New()
	tr.On("GetAgentOwnerUserID", mock.Anything, agentID).
		Return(nil, repository.ErrTeamAgentNotFound)

	svc := NewTeamService(tr, new(mockToolDefRepo))
	_, err := svc.GetAgentSettings(context.Background(),
		AgentSettingsActor{UserID: uuid.New()}, agentID)
	require.ErrorIs(t, err, ErrTeamAgentNotFound)
}
