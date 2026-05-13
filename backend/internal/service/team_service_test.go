package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

type mockTeamRepo struct{ mock.Mock }

func (m *mockTeamRepo) Create(ctx context.Context, team *models.Team) error {
	return m.Called(ctx, team).Error(0)
}
func (m *mockTeamRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}
func (m *mockTeamRepo) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}
func (m *mockTeamRepo) GetAgentInProject(ctx context.Context, projectID, agentID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, projectID, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *mockTeamRepo) GetAgentByID(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *mockTeamRepo) GetAgentOwnerUserID(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return uuid.Nil, args.Error(1)
	}
	return args.Get(0).(uuid.UUID), args.Error(1)
}
func (m *mockTeamRepo) SaveAgent(ctx context.Context, agent *models.Agent) error {
	return m.Called(ctx, agent).Error(0)
}
func (m *mockTeamRepo) SaveAgentWithToolBindings(ctx context.Context, agent *models.Agent, replace bool, ids []uuid.UUID) error {
	return m.Called(ctx, agent, replace, ids).Error(0)
}
func (m *mockTeamRepo) Update(ctx context.Context, team *models.Team) error {
	return m.Called(ctx, team).Error(0)
}
func (m *mockTeamRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type mockToolDefRepo struct{ mock.Mock }

func (m *mockToolDefRepo) ListActiveCatalog(ctx context.Context) ([]models.ToolDefinition, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ToolDefinition), args.Error(1)
}
func (m *mockToolDefRepo) CountActiveInIDs(ctx context.Context, ids []uuid.UUID) (int64, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(int64), args.Error(1)
}

func patchJSONWithToolBindings(ids []uuid.UUID) ([]byte, error) {
	type tb struct {
		ToolDefinitionID string `json:"tool_definition_id"`
	}
	arr := make([]tb, len(ids))
	for i, id := range ids {
		arr[i] = tb{ToolDefinitionID: id.String()}
	}
	return json.Marshal(map[string]any{"tool_bindings": arr})
}

func TestTeamService_PatchAgent_ToolBindings_LimitExceeded(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	agent := &models.Agent{
		ID:       aid,
		Name:     "a",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &uuid.Nil,
		Skills:   skills,
		Settings: settings,
		IsActive: true,
	}
	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil)

	ids := make([]uuid.UUID, 51)
	for i := range ids {
		ids[i] = uuid.New()
	}
	raw, err := patchJSONWithToolBindings(ids)
	require.NoError(t, err)
	var req dto.PatchAgentRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err = svc.PatchAgent(context.Background(), pid, aid, req)
	require.ErrorIs(t, err, ErrTeamAgentInvalidToolBindings)
	tr.AssertNotCalled(t, "SaveAgentWithToolBindings")
}

func TestTeamService_PatchAgent_ToolBindings_InvalidCatalog(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	tid := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	agent := &models.Agent{
		ID:       aid,
		Name:     "a",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &uuid.Nil,
		Skills:   skills,
		Settings: settings,
		IsActive: true,
	}
	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil)
	td.On("CountActiveInIDs", mock.Anything, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 1 && ids[0] == tid
	})).Return(int64(0), nil)

	raw, err := patchJSONWithToolBindings([]uuid.UUID{tid})
	require.NoError(t, err)
	var req dto.PatchAgentRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err = svc.PatchAgent(context.Background(), pid, aid, req)
	require.ErrorIs(t, err, ErrTeamAgentInvalidToolBindings)
	tr.AssertNotCalled(t, "SaveAgentWithToolBindings")
}

func TestTeamService_PatchAgent_ToolBindings_DedupAndSave(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	tid := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	agent := &models.Agent{
		ID:       aid,
		Name:     "a",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &uuid.Nil,
		Skills:   skills,
		Settings: settings,
		IsActive: true,
	}
	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil)
	td.On("CountActiveInIDs", mock.Anything, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 1 && ids[0] == tid
	})).Return(int64(1), nil)

	tr.On("SaveAgentWithToolBindings", mock.Anything, agent, true, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 1 && ids[0] == tid
	})).Return(nil)

	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil)

	body := map[string]any{
		"tool_bindings": []map[string]string{
			{"tool_definition_id": tid.String()},
			{"tool_definition_id": tid.String()},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	var req dto.PatchAgentRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	got, err := svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)
	require.Equal(t, teamOut, got)
	tr.AssertExpectations(t)
	td.AssertExpectations(t)
}

func TestTeamService_PatchAgent_ToolBindingsClearAll(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	agent := &models.Agent{
		ID:       aid,
		Name:     "a",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &uuid.Nil,
		Skills:   skills,
		Settings: settings,
		IsActive: true,
	}
	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil)
	tr.On("SaveAgentWithToolBindings", mock.Anything, agent, true, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 0
	})).Return(nil)
	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil)

	var req dto.PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"tool_bindings":[]}`), &req))

	got, err := svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)
	require.Equal(t, teamOut, got)
	td.AssertNotCalled(t, "CountActiveInIDs")
	tr.AssertExpectations(t)
}

func TestMapAgentPatchPostgresFK_BindingsToolDefinition(t *testing.T) {
	wrapped := fmt.Errorf("repo: %w", &pgconn.PgError{
		Code:            "23503",
		ConstraintName:  fkAgentToolBindingsToolDefinitionID,
	})
	mapped, ok := mapAgentPatchPostgresFK(wrapped)
	require.True(t, ok)
	require.ErrorIs(t, mapped, ErrTeamAgentInvalidToolBindings)
}

func TestMapAgentPatchPostgresFK_PromptFKToConflict(t *testing.T) {
	wrapped := fmt.Errorf("repo: %w", &pgconn.PgError{
		Code:             "23503",
		ConstraintName:   fkAgentsPromptID,
	})
	mapped, ok := mapAgentPatchPostgresFK(wrapped)
	require.True(t, ok)
	require.ErrorIs(t, mapped, ErrTeamAgentConflict)
}

func TestMapAgentPatchPostgresFK_UnknownConstraintNotMapped(t *testing.T) {
	wrapped := fmt.Errorf("repo: %w", &pgconn.PgError{
		Code:             "23503",
		ConstraintName:   "agent_tool_bindings_agent_id_fkey",
	})
	_, ok := mapAgentPatchPostgresFK(wrapped)
	require.False(t, ok)
}

func TestMapAgentPatchPostgresFK_NotFK(t *testing.T) {
	_, ok := mapAgentPatchPostgresFK(fmt.Errorf("other"))
	require.False(t, ok)
}

// Инвариант 13.3.1 A.12 / §5: повторный PATCH с тем же набором tool_bindings не должен быть
// no-op на уровне сервиса — репозиторий всегда вызывается (updated_at в БД строго растёт).
func TestTeamService_PatchAgent_ToolBindings_SameSetTwiceStillCallsSave(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	tid := uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	agent := &models.Agent{
		ID:       aid,
		Name:     "a",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &uuid.Nil,
		Skills:   skills,
		Settings: settings,
		IsActive: true,
	}
	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil).Twice()
	td.On("CountActiveInIDs", mock.Anything, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 1 && ids[0] == tid
	})).Return(int64(1), nil).Twice()
	tr.On("SaveAgentWithToolBindings", mock.Anything, agent, true, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 1 && ids[0] == tid
	})).Return(nil).Twice()
	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil).Twice()

	raw, err := patchJSONWithToolBindings([]uuid.UUID{tid})
	require.NoError(t, err)
	var req dto.PatchAgentRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err = svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)
	_, err = svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)
	tr.AssertNumberOfCalls(t, "SaveAgentWithToolBindings", 2)
	tr.AssertExpectations(t)
	td.AssertExpectations(t)
}
