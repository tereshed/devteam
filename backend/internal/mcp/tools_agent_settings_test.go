package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Sprint 15.N9 — MCP-обёртки agent_settings_get/update + skill_list уважают ownership.
// Без этих тестов любая регрессия в makeAgentSettings* / makeSkillListHandler незаметно
// сломает защиту (service-уровень покрыт, MCP — нет).

// --- mocks ---

type mockAgentSkillRepo struct{ mock.Mock }

func (m *mockAgentSkillRepo) ListByAgent(ctx context.Context, agentID uuid.UUID, onlyActive bool) ([]models.AgentSkill, error) {
	args := m.Called(ctx, agentID, onlyActive)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AgentSkill), args.Error(1)
}

func (m *mockAgentSkillRepo) ListAll(ctx context.Context, onlyActive bool) ([]models.AgentSkill, error) {
	args := m.Called(ctx, onlyActive)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AgentSkill), args.Error(1)
}

// adminCtx — actor с RoleAdmin (bypass ownership-check).
func adminCtx() (context.Context, uuid.UUID) {
	uid := uuid.New()
	ctx := context.WithValue(context.Background(), CtxKeyUserID, uid)
	ctx = context.WithValue(ctx, CtxKeyUserRole, models.RoleAdmin)
	return ctx, uid
}

// userCtx — actor с RoleUser (требует ownership-check).
func userCtx() (context.Context, uuid.UUID) {
	uid := uuid.New()
	ctx := context.WithValue(context.Background(), CtxKeyUserID, uid)
	ctx = context.WithValue(ctx, CtxKeyUserRole, models.RoleUser)
	return ctx, uid
}

// --- agent_settings_get ownership ---

func TestMCP_AgentSettingsGet_RequiresAuth(t *testing.T) {
	svc := new(mockTeamService)
	h := makeAgentSettingsGetHandler(svc)
	result, _, err := h(context.Background(), nil, &AgentSettingsGetParams{AgentID: uuid.New().String()})
	require.NoError(t, err)
	assert.True(t, result.IsError, "no auth → error")
	svc.AssertNotCalled(t, "GetAgentSettings")
}

func TestMCP_AgentSettingsGet_PassesActorToService(t *testing.T) {
	ctx, uid := userCtx()
	svc := new(mockTeamService)
	agentID := uuid.New()
	wantActor := service.AgentSettingsActor{UserID: uid, IsAdmin: false}

	svc.On("GetAgentSettings", mock.Anything, wantActor, agentID).
		Return(&models.Agent{ID: agentID}, nil)

	result, _, err := makeAgentSettingsGetHandler(svc)(ctx, nil,
		&AgentSettingsGetParams{AgentID: agentID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestMCP_AgentSettingsGet_PropagatesForbiddenAsError(t *testing.T) {
	ctx, uid := userCtx()
	svc := new(mockTeamService)
	agentID := uuid.New()
	svc.On("GetAgentSettings", mock.Anything, service.AgentSettingsActor{UserID: uid}, agentID).
		Return(nil, service.ErrTeamAgentNotFound)

	result, _, err := makeAgentSettingsGetHandler(svc)(ctx, nil,
		&AgentSettingsGetParams{AgentID: agentID.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError, "foreign-agent must surface as error")
}

// --- agent_settings_update: actor propagated + invalid agent_id ---

func TestMCP_AgentSettingsUpdate_RequiresAuth(t *testing.T) {
	svc := new(mockTeamService)
	h := makeAgentSettingsUpdateHandler(svc)
	result, _, err := h(context.Background(), nil, &AgentSettingsUpdateParams{AgentID: uuid.New().String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "UpdateAgentSettings")
}

func TestMCP_AgentSettingsUpdate_PassesActor(t *testing.T) {
	ctx, uid := userCtx()
	svc := new(mockTeamService)
	agentID := uuid.New()
	expectedActor := service.AgentSettingsActor{UserID: uid, IsAdmin: false}

	svc.On("UpdateAgentSettings", mock.Anything, expectedActor, agentID, mock.Anything).
		Return(&models.Agent{ID: agentID}, nil)

	params := &AgentSettingsUpdateParams{
		AgentID:            agentID.String(),
		SandboxPermissions: json.RawMessage(`{"defaultMode":"acceptEdits"}`),
	}
	result, _, err := makeAgentSettingsUpdateHandler(svc)(ctx, nil, params)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestMCP_AgentSettingsUpdate_InvalidAgentID(t *testing.T) {
	ctx, _ := userCtx()
	svc := new(mockTeamService)
	result, _, err := makeAgentSettingsUpdateHandler(svc)(ctx, nil,
		&AgentSettingsUpdateParams{AgentID: "not-a-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "UpdateAgentSettings")
}

// --- skill_list ownership ---

func TestMCP_SkillList_PerAgent_ChecksOwnershipThroughTeamSvc(t *testing.T) {
	ctx, uid := userCtx()
	teamSvc := new(mockTeamService)
	repo := new(mockAgentSkillRepo)
	agentID := uuid.New()

	// GetAgentSettings возвращает ErrTeamAgentNotFound (актёр — не owner).
	teamSvc.On("GetAgentSettings", mock.Anything,
		service.AgentSettingsActor{UserID: uid}, agentID).
		Return(nil, service.ErrTeamAgentNotFound)

	h := makeSkillListHandler(repo, teamSvc)
	agentIDStr := agentID.String()
	result, _, err := h(ctx, nil, &SkillListParams{AgentID: &agentIDStr})
	require.NoError(t, err)
	assert.True(t, result.IsError, "foreign agent must surface as error")
	// ListByAgent НЕ должен вызываться.
	repo.AssertNotCalled(t, "ListByAgent")
}

func TestMCP_SkillList_PerAgent_OwnerAllowed(t *testing.T) {
	ctx, uid := userCtx()
	teamSvc := new(mockTeamService)
	repo := new(mockAgentSkillRepo)
	agentID := uuid.New()

	teamSvc.On("GetAgentSettings", mock.Anything,
		service.AgentSettingsActor{UserID: uid}, agentID).
		Return(&models.Agent{ID: agentID}, nil)
	repo.On("ListByAgent", mock.Anything, agentID, false).
		Return([]models.AgentSkill{}, nil)

	h := makeSkillListHandler(repo, teamSvc)
	agentIDStr := agentID.String()
	result, _, err := h(ctx, nil, &SkillListParams{AgentID: &agentIDStr})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	teamSvc.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestMCP_SkillList_NoAgentID_RequiresAdmin(t *testing.T) {
	ctx, _ := userCtx() // non-admin
	teamSvc := new(mockTeamService)
	repo := new(mockAgentSkillRepo)

	h := makeSkillListHandler(repo, teamSvc)
	result, _, err := h(ctx, nil, &SkillListParams{})
	require.NoError(t, err)
	assert.True(t, result.IsError, "non-admin without agent_id must be rejected")
	repo.AssertNotCalled(t, "ListAll")
}

func TestMCP_SkillList_NoAgentID_AdminListsAll(t *testing.T) {
	uid := uuid.New()
	ctx := context.WithValue(context.Background(), CtxKeyUserID, uid)
	ctx = context.WithValue(ctx, CtxKeyUserRole, models.RoleAdmin)

	teamSvc := new(mockTeamService)
	repo := new(mockAgentSkillRepo)
	repo.On("ListAll", mock.Anything, false).Return([]models.AgentSkill{}, nil)

	h := makeSkillListHandler(repo, teamSvc)
	result, _, err := h(ctx, nil, &SkillListParams{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	repo.AssertExpectations(t)
}

// --- admin bypass для get ---

func TestMCP_AgentSettingsGet_AdminBypassesOwnership(t *testing.T) {
	ctx, uid := adminCtx()
	svc := new(mockTeamService)
	agentID := uuid.New()

	// Admin actor — IsAdmin=true.
	svc.On("GetAgentSettings", mock.Anything,
		service.AgentSettingsActor{UserID: uid, IsAdmin: true}, agentID).
		Return(&models.Agent{ID: agentID}, nil)

	result, _, err := makeAgentSettingsGetHandler(svc)(ctx, nil,
		&AgentSettingsGetParams{AgentID: agentID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

// заглушка чтобы избежать import не использовался.
var _ = repository.AgentSkillRepository(nil)
var _ = dto.AgentSettingsResponse{}
