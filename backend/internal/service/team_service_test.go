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
func (m *mockTeamRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.Team, error) {
	args := m.Called(ctx, projectID)
	var teams []models.Team
	if v := args.Get(0); v != nil {
		teams = v.([]models.Team)
	}
	return teams, args.Error(1)
}
func (m *mockTeamRepo) GetByProjectIDAndType(ctx context.Context, projectID uuid.UUID, teamType models.TeamType) (*models.Team, error) {
	args := m.Called(ctx, projectID, teamType)
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

func (m *mockTeamRepo) CreateTeamType(ctx context.Context, tt *models.TeamTypeModel) error {
	return m.Called(ctx, tt).Error(0)
}
func (m *mockTeamRepo) GetTeamTypeByCode(ctx context.Context, code string) (*models.TeamTypeModel, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamTypeModel), args.Error(1)
}
func (m *mockTeamRepo) ListTeamTypes(ctx context.Context) ([]models.TeamTypeModel, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.TeamTypeModel), args.Error(1)
}
func (m *mockTeamRepo) DeleteTeamType(ctx context.Context, code string) error {
	return m.Called(ctx, code).Error(0)
}
func (m *mockTeamRepo) CountTeamsByType(ctx context.Context, code string) (int64, error) {
	args := m.Called(ctx, code)
	return args.Get(0).(int64), args.Error(1)
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

func TestTeamService_TeamTypes(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	t.Run("ListTeamTypes", func(t *testing.T) {
		expected := []models.TeamTypeModel{
			{Code: "dev", Name: "Development", IsSystem: true},
			{Code: "research", Name: "Research", IsSystem: false},
		}
		tr.On("ListTeamTypes", mock.Anything).Return(expected, nil).Once()
		res, err := svc.ListTeamTypes(context.Background())
		require.NoError(t, err)
		require.Equal(t, expected, res)
	})

	t.Run("CreateTeamType - Success", func(t *testing.T) {
		req := dto.CreateTeamTypeRequest{
			Code: "custom",
			Name: "Custom Team",
		}
		tr.On("GetTeamTypeByCode", mock.Anything, "custom").Return(nil, fmt.Errorf("not found")).Once()
		tr.On("CreateTeamType", mock.Anything, mock.MatchedBy(func(tt *models.TeamTypeModel) bool {
			return tt.Code == "custom" && tt.Name == "Custom Team" && !tt.IsSystem
		})).Return(nil).Once()

		res, err := svc.CreateTeamType(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, "custom", res.Code)
	})

	t.Run("CreateTeamType - Already Exists", func(t *testing.T) {
		req := dto.CreateTeamTypeRequest{
			Code: "custom",
			Name: "Custom Team",
		}
		tr.On("GetTeamTypeByCode", mock.Anything, "custom").Return(&models.TeamTypeModel{Code: "custom"}, nil).Once()

		_, err := svc.CreateTeamType(context.Background(), req)
		require.ErrorIs(t, err, ErrTeamTypeAlreadyExists)
	})

	t.Run("DeleteTeamType - System Protection", func(t *testing.T) {
		tr.On("GetTeamTypeByCode", mock.Anything, "dev").Return(&models.TeamTypeModel{Code: "dev", IsSystem: true}, nil).Once()
		err := svc.DeleteTeamType(context.Background(), "dev")
		require.ErrorIs(t, err, ErrTeamTypeCannotDeleteSystem)
	})

	t.Run("DeleteTeamType - In Use Protection", func(t *testing.T) {
		tr.On("GetTeamTypeByCode", mock.Anything, "qa").Return(&models.TeamTypeModel{Code: "qa", IsSystem: false}, nil).Once()
		tr.On("CountTeamsByType", mock.Anything, "qa").Return(int64(3), nil).Once()
		err := svc.DeleteTeamType(context.Background(), "qa")
		require.ErrorIs(t, err, ErrTeamTypeInUse)
	})

	t.Run("DeleteTeamType - Success", func(t *testing.T) {
		tr.On("GetTeamTypeByCode", mock.Anything, "qa").Return(&models.TeamTypeModel{Code: "qa", IsSystem: false}, nil).Once()
		tr.On("CountTeamsByType", mock.Anything, "qa").Return(int64(0), nil).Once()
		tr.On("DeleteTeamType", mock.Anything, "qa").Return(nil).Once()
		err := svc.DeleteTeamType(context.Background(), "qa")
		require.NoError(t, err)
	})
}

func TestTeamService_PatchAgent_SandboxAgent_ModelInSettings(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()

	agent := &models.Agent{
		ID:            aid,
		Name:          "sandbox-agent",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindSandbox,
		CodeBackendSettings: datatypes.JSON([]byte(`{"hermes":{"toolsets":["file_ops"],"permission_mode":"yolo"}}`)),
	}

	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil).Once()
	tr.On("SaveAgent", mock.Anything, mock.MatchedBy(func(a *models.Agent) bool {
		if a.Model != nil {
			return false
		}
		var settings AgentCodeBackendSettings
		if err := json.Unmarshal(a.CodeBackendSettings, &settings); err != nil {
			return false
		}
		return settings.Model == "deepseek/deepseek-v4-flash" && settings.Hermes != nil && len(settings.Hermes.Toolsets) == 1
	})).Return(nil).Once()

	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil).Once()

	var req dto.PatchAgentRequest
	raw := []byte(`{"model":"deepseek/deepseek-v4-flash"}`)
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err := svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)

	tr.AssertExpectations(t)
}

func TestTeamService_PatchAgent_TransitionToSandbox(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()
	initialModel := "gpt-4o"

	agent := &models.Agent{
		ID:            aid,
		Name:          "transition-agent",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &initialModel,
		CodeBackend:   nil,
	}

	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil).Once()
	tr.On("SaveAgent", mock.Anything, mock.MatchedBy(func(a *models.Agent) bool {
		// check transitions
		if a.ExecutionKind != models.AgentExecutionKindSandbox {
			return false
		}
		if a.CodeBackend == nil || string(*a.CodeBackend) != "hermes" {
			return false
		}
		if a.Model != nil {
			return false
		}
		var settings AgentCodeBackendSettings
		if err := json.Unmarshal(a.CodeBackendSettings, &settings); err != nil {
			return false
		}
		return settings.Model == "gpt-4o"
	})).Return(nil).Once()

	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil).Once()

	var req dto.PatchAgentRequest
	raw := []byte(`{"code_backend":"hermes"}`)
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err := svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)

	tr.AssertExpectations(t)
}

func TestTeamService_PatchAgent_TransitionToLLM(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	pid := uuid.New()
	aid := uuid.New()

	agent := &models.Agent{
		ID:                  aid,
		Name:                "transition-agent",
		Role:                models.AgentRoleDeveloper,
		ExecutionKind:       models.AgentExecutionKindSandbox,
		Model:               nil,
		CodeBackend:         cbPtr(models.CodeBackendHermes),
		CodeBackendSettings: datatypes.JSON([]byte(`{"model":"gpt-4o"}`)),
	}

	tr.On("GetAgentInProject", mock.Anything, pid, aid).Return(agent, nil).Once()
	tr.On("SaveAgent", mock.Anything, mock.MatchedBy(func(a *models.Agent) bool {
		// check transitions
		if a.ExecutionKind != models.AgentExecutionKindLLM {
			return false
		}
		if a.CodeBackend != nil {
			return false
		}
		if a.Model == nil || *a.Model != "gpt-4o" {
			return false
		}
		var settings AgentCodeBackendSettings
		if err := json.Unmarshal(a.CodeBackendSettings, &settings); err != nil {
			return false
		}
		return settings.Model == ""
	})).Return(nil).Once()

	teamOut := &models.Team{ID: uuid.New(), ProjectID: pid}
	tr.On("GetByProjectID", mock.Anything, pid).Return(teamOut, nil).Once()

	var req dto.PatchAgentRequest
	raw := []byte(`{"code_backend":""}`)
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err := svc.PatchAgent(context.Background(), pid, aid, req)
	require.NoError(t, err)

	tr.AssertExpectations(t)
}

func TestTeamService_UpdateAgentSettings_TransitionToSandbox(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	aid := uuid.New()
	userID := uuid.New()
	initialModel := "gpt-4o"

	agent := &models.Agent{
		ID:            aid,
		Name:          "transition-agent",
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &initialModel,
		CodeBackend:   nil,
	}

	tr.On("GetAgentByID", mock.Anything, aid).Return(agent, nil).Once()
	tr.On("GetAgentOwnerUserID", mock.Anything, aid).Return(userID, nil).Once() // Owner matching actor ID
	tr.On("SaveAgent", mock.Anything, mock.MatchedBy(func(a *models.Agent) bool {
		return a.ExecutionKind == models.AgentExecutionKindSandbox &&
			a.Model == nil &&
			a.CodeBackend != nil && string(*a.CodeBackend) == "hermes"
	})).Return(nil).Once()

	req := dto.UpdateAgentSettingsRequest{
		CodeBackend: cbPtrString("hermes"),
	}

	actor := AgentSettingsActor{UserID: userID, IsAdmin: false}
	_, err := svc.UpdateAgentSettings(context.Background(), actor, aid, req)
	require.NoError(t, err)

	tr.AssertExpectations(t)
}

func TestTeamService_UpdateAgentSettings_TransitionToLLM(t *testing.T) {
	tr := new(mockTeamRepo)
	td := new(mockToolDefRepo)
	svc := NewTeamService(tr, td)

	aid := uuid.New()
	userID := uuid.New()

	agent := &models.Agent{
		ID:                  aid,
		Name:                "transition-agent",
		Role:                models.AgentRoleDeveloper,
		ExecutionKind:       models.AgentExecutionKindSandbox,
		Model:               nil,
		CodeBackend:         cbPtr(models.CodeBackendHermes),
		CodeBackendSettings: datatypes.JSON([]byte(`{"model":"gpt-4o"}`)),
	}

	tr.On("GetAgentByID", mock.Anything, aid).Return(agent, nil).Once()
	tr.On("GetAgentOwnerUserID", mock.Anything, aid).Return(userID, nil).Once()
	tr.On("SaveAgent", mock.Anything, mock.MatchedBy(func(a *models.Agent) bool {
		return a.ExecutionKind == models.AgentExecutionKindLLM &&
			a.Model != nil && *a.Model == "gpt-4o" &&
			a.CodeBackend == nil
	})).Return(nil).Once()

	req := dto.UpdateAgentSettingsRequest{
		CodeBackend: cbPtrString(""),
	}

	actor := AgentSettingsActor{UserID: userID, IsAdmin: false}
	_, err := svc.UpdateAgentSettings(context.Background(), actor, aid, req)
	require.NoError(t, err)

	tr.AssertExpectations(t)
}

func cbPtrString(v string) *string                  { return &v }

