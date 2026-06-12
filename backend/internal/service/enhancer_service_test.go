package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

// fakeEnhancerRepo — in-memory EnhancerRepository. Мьютекс обязателен:
// executeRun пишет результат прогона из горутины.
type fakeEnhancerRepo struct {
	mu        sync.Mutex
	configs   map[uuid.UUID]*models.EnhancerConfig // key: project_id
	runs      map[uuid.UUID]*models.EnhancerRun
	changes   map[uuid.UUID]*models.EnhancerChange
	overrides map[string]*models.ProjectAgentOverride // key: project_id+agent_id
}

func newFakeEnhancerRepo() *fakeEnhancerRepo {
	return &fakeEnhancerRepo{
		configs:   map[uuid.UUID]*models.EnhancerConfig{},
		runs:      map[uuid.UUID]*models.EnhancerRun{},
		changes:   map[uuid.UUID]*models.EnhancerChange{},
		overrides: map[string]*models.ProjectAgentOverride{},
	}
}

func overrideKey(projectID, agentID uuid.UUID) string {
	return projectID.String() + "/" + agentID.String()
}

func (f *fakeEnhancerRepo) GetConfigByProjectID(ctx context.Context, projectID uuid.UUID) (*models.EnhancerConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cfg, ok := f.configs[projectID]
	if !ok {
		return nil, repository.ErrEnhancerConfigNotFound
	}
	cp := *cfg
	return &cp, nil
}

func (f *fakeEnhancerRepo) CreateConfig(ctx context.Context, cfg *models.EnhancerConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *cfg
	f.configs[cfg.ProjectID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) UpdateConfig(ctx context.Context, cfg *models.EnhancerConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.configs[cfg.ProjectID]; !ok {
		return repository.ErrEnhancerConfigNotFound
	}
	cp := *cfg
	f.configs[cfg.ProjectID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) ListDueConfigs(ctx context.Context, now time.Time, limit int) ([]models.EnhancerConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.EnhancerConfig
	for _, cfg := range f.configs {
		if cfg.IsActive && cfg.NextRunAt != nil && !cfg.NextRunAt.After(now) {
			out = append(out, *cfg)
		}
	}
	return out, nil
}

func (f *fakeEnhancerRepo) CreateRun(ctx context.Context, run *models.EnhancerRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *run
	f.runs[run.ID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) GetRunByID(ctx context.Context, id uuid.UUID) (*models.EnhancerRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok {
		return nil, repository.ErrEnhancerRunNotFound
	}
	cp := *run
	return &cp, nil
}

func (f *fakeEnhancerRepo) UpdateRun(ctx context.Context, run *models.EnhancerRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.runs[run.ID]; !ok {
		return repository.ErrEnhancerRunNotFound
	}
	cp := *run
	f.runs[run.ID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) ListRunsByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]models.EnhancerRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.EnhancerRun
	for _, run := range f.runs {
		if run.ProjectID == projectID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (f *fakeEnhancerRepo) HasRunningRun(ctx context.Context, projectID uuid.UUID, staleAfter time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().Add(-staleAfter)
	for _, run := range f.runs {
		if run.ProjectID != projectID || run.Status != models.EnhancerRunStatusRunning {
			continue
		}
		if run.StartedAt.Before(cutoff) {
			run.Status = models.EnhancerRunStatusFailed
			continue
		}
		return true, nil
	}
	return false, nil
}

func (f *fakeEnhancerRepo) CreateChange(ctx context.Context, change *models.EnhancerChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *change
	f.changes[change.ID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) CountChangesByRunID(ctx context.Context, runID uuid.UUID) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, ch := range f.changes {
		if ch.RunID == runID {
			n++
		}
	}
	return n, nil
}

func (f *fakeEnhancerRepo) ListChangesByRunID(ctx context.Context, runID uuid.UUID) ([]models.EnhancerChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.EnhancerChange
	for _, ch := range f.changes {
		if ch.RunID == runID {
			out = append(out, *ch)
		}
	}
	return out, nil
}

func (f *fakeEnhancerRepo) GetChangeByID(ctx context.Context, id uuid.UUID) (*models.EnhancerChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.changes[id]
	if !ok {
		return nil, repository.ErrEnhancerChangeNotFound
	}
	cp := *ch
	return &cp, nil
}

func (f *fakeEnhancerRepo) UpdateChange(ctx context.Context, change *models.EnhancerChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.changes[change.ID]; !ok {
		return repository.ErrEnhancerChangeNotFound
	}
	cp := *change
	f.changes[change.ID] = &cp
	return nil
}

func (f *fakeEnhancerRepo) ListAppliedAgentChanges(ctx context.Context, projectID, agentID uuid.UUID) ([]models.EnhancerChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.EnhancerChange
	for _, ch := range f.changes {
		if ch.ProjectID == projectID && ch.TargetAgentID != nil && *ch.TargetAgentID == agentID &&
			ch.TargetKind == models.EnhancerChangeKindAgentOverride && ch.Status == models.EnhancerChangeStatusApplied {
			out = append(out, *ch)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aj := out[i].AppliedAt, out[j].AppliedAt
		if ai == nil || aj == nil {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return ai.Before(*aj)
	})
	return out, nil
}

func (f *fakeEnhancerRepo) GetActiveOverride(ctx context.Context, projectID, agentID uuid.UUID) (*models.ProjectAgentOverride, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.overrides[overrideKey(projectID, agentID)]
	if !ok || !o.IsActive {
		return nil, repository.ErrProjectAgentOverrideNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeEnhancerRepo) UpsertOverride(ctx context.Context, projectID, agentID uuid.UUID, promptAddendum string, updatedBy uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := overrideKey(projectID, agentID)
	if promptAddendum == "" {
		delete(f.overrides, key)
		return nil
	}
	f.overrides[key] = &models.ProjectAgentOverride{
		ID:             uuid.New(),
		ProjectID:      projectID,
		AgentID:        agentID,
		PromptAddendum: promptAddendum,
		IsActive:       true,
		UpdatedBy:      &updatedBy,
	}
	return nil
}

// fakeEnhancerTeamRepo — заглушка TeamRepository: реализован только
// GetAgentInProject, остальные методы (через embedded nil-интерфейс) паникуют.
type fakeEnhancerTeamRepo struct {
	repository.TeamRepository
	agentInProject map[uuid.UUID]bool
}

func (f *fakeEnhancerTeamRepo) GetAgentInProject(ctx context.Context, projectID, agentID uuid.UUID) (*models.Agent, error) {
	if f.agentInProject[agentID] {
		return &models.Agent{ID: agentID}, nil
	}
	return nil, repository.ErrTeamAgentNotFound
}

// stubEnhancerToolCatalog — пустой каталог read-инструментов.
type stubEnhancerToolCatalog struct{}

func (stubEnhancerToolCatalog) Catalog() []agentloop.Tool { return nil }

// stubEnhancerLLMResolver — резолвер, всегда падающий с ошибкой: прогоны в
// юнит-тестах не должны доходить до LLM.
type stubEnhancerLLMResolver struct{}

func (stubEnhancerLLMResolver) ResolveAssistantClient(ctx context.Context, agent *models.Agent, userID uuid.UUID) (llm.Client, error) {
	return nil, errors.New("llm not configured in test")
}

func newTestEnhancerService(t *testing.T, repo *fakeEnhancerRepo, userRepo repository.UserRepository) EnhancerService {
	t.Helper()
	svc, err := NewEnhancerService(EnhancerServiceDeps{
		Repo:        repo,
		ProjectSvc:  okProjectSvc(),
		TeamRepo:    &fakeEnhancerTeamRepo{},
		UserRepo:    userRepo,
		TaskRepo:    repository.NewTaskRepository(nil),
		TaskMsgRepo: repository.NewTaskMessageRepository(nil),
		AgentSvc:    &AgentService{},
		LLMResolver: stubEnhancerLLMResolver{},
		ToolCatalog: stubEnhancerToolCatalog{},
		Executor: agentloop.NewExecutor(agentloop.Config{
			MaxIterations:      3,
			MaxToolResultBytes: 1024,
			MaxHistoryBytes:    64 * 1024,
			HistoryTailKeep:    2,
			PerLLMCallTimeout:  time.Second,
		}, slog.Default()),
		Logger: slog.Default(),
	})
	require.NoError(t, err)
	return svc
}

// --- config tests ---

func TestEnhancerGetConfig_DefaultWhenMissing(t *testing.T) {
	repo := newFakeEnhancerRepo()
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{})
	projectID := uuid.New()

	cfg, err := svc.GetConfig(context.Background(), uuid.New(), models.RoleUser, projectID)
	require.NoError(t, err)
	require.False(t, cfg.IsActive)
	require.Equal(t, models.EnhancerAutonomyPropose, cfg.Autonomy)
	require.Equal(t, enhancerDefaultWindowDays, cfg.AnalysisWindowDays)
	require.Equal(t, enhancerDefaultMaxChanges, cfg.MaxChangesPerRun)
	// Дефолт не персистится.
	_, repoErr := repo.GetConfigByProjectID(context.Background(), projectID)
	require.ErrorIs(t, repoErr, repository.ErrEnhancerConfigNotFound)
}

func TestEnhancerUpdateConfig_Validation(t *testing.T) {
	svc := newTestEnhancerService(t, newFakeEnhancerRepo(), &fakeUserRepo{})
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	_, err := svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{Autonomy: strPtr("auto_apply")})
	require.ErrorIs(t, err, ErrEnhancerInvalidAutonomy)

	_, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{CronExpression: strPtr("not a cron")})
	require.ErrorIs(t, err, ErrEnhancerInvalidCron)

	_, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{AnalysisWindowDays: intPtr(0)})
	require.ErrorIs(t, err, ErrEnhancerInvalidWindow)

	_, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{AnalysisWindowDays: intPtr(91)})
	require.ErrorIs(t, err, ErrEnhancerInvalidWindow)

	_, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{MaxChangesPerRun: intPtr(0)})
	require.ErrorIs(t, err, ErrEnhancerInvalidLimit)

	_, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{MaxChangesPerRun: intPtr(21)})
	require.ErrorIs(t, err, ErrEnhancerInvalidLimit)
}

func TestEnhancerUpdateConfig_CreateAndSchedule(t *testing.T) {
	repo := newFakeEnhancerRepo()
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{})
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	active := true
	cron := "0 9 * * *"
	cfg, err := svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{
		IsActive:       &active,
		CronExpression: &cron,
	})
	require.NoError(t, err)
	require.True(t, cfg.IsActive)
	require.NotNil(t, cfg.NextRunAt, "активный конфиг с cron обязан получить next_run_at")
	require.Equal(t, userID, cfg.CreatedBy)

	// Выключение гасит next_run_at.
	inactive := false
	cfg, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{IsActive: &inactive})
	require.NoError(t, err)
	require.Nil(t, cfg.NextRunAt)

	// Очистка расписания пустой строкой.
	active2 := true
	empty := ""
	cfg, err = svc.UpdateConfig(ctx, userID, models.RoleUser, projectID, dto.UpdateEnhancerConfigRequest{IsActive: &active2, CronExpression: &empty})
	require.NoError(t, err)
	require.Nil(t, cfg.CronExpression)
	require.Nil(t, cfg.NextRunAt)
}

// --- run tests ---

func TestEnhancerRunNow_BusyGuard(t *testing.T) {
	repo := newFakeEnhancerRepo()
	projectID := uuid.New()
	repo.runs[uuid.New()] = &models.EnhancerRun{
		ID:        uuid.New(),
		ProjectID: projectID,
		Status:    models.EnhancerRunStatusRunning,
		StartedAt: time.Now(),
	}
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{err: errors.New("no user")})

	_, err := svc.RunNow(context.Background(), uuid.New(), models.RoleUser, projectID)
	require.ErrorIs(t, err, ErrEnhancerRunInProgress)
}

func TestEnhancerRunNow_StaleRunningRecovered(t *testing.T) {
	repo := newFakeEnhancerRepo()
	projectID := uuid.New()
	staleID := uuid.New()
	repo.runs[staleID] = &models.EnhancerRun{
		ID:        staleID,
		ProjectID: projectID,
		Status:    models.EnhancerRunStatusRunning,
		StartedAt: time.Now().Add(-2 * EnhancerRunStaleAfter),
	}
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{err: errors.New("no user")})

	run, err := svc.RunNow(context.Background(), uuid.New(), models.RoleUser, projectID)
	require.NoError(t, err, "застрявший running не должен блокировать новый прогон")
	require.Equal(t, models.EnhancerRunTriggerManual, run.TriggerKind)
}

func TestEnhancerRunNow_RunFailsCleanlyWithoutLLM(t *testing.T) {
	repo := newFakeEnhancerRepo()
	projectID := uuid.New()
	// Owner lookup падает → executeRun обязан зафиксировать failed, а не
	// оставить run висеть в running.
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{err: errors.New("no user")})

	run, err := svc.RunNow(context.Background(), uuid.New(), models.RoleUser, projectID)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		stored, err := repo.GetRunByID(context.Background(), run.ID)
		return err == nil && stored.Status == models.EnhancerRunStatusFailed && stored.FinishedAt != nil
	}, 2*time.Second, 10*time.Millisecond, "прогон обязан перейти в failed")
}

func TestEnhancerRunDue_FiresAndAdvances(t *testing.T) {
	repo := newFakeEnhancerRepo()
	projectID := uuid.New()
	past := time.Now().Add(-time.Minute)
	cron := "*/5 * * * *"
	repo.configs[projectID] = &models.EnhancerConfig{
		ID:                 uuid.New(),
		ProjectID:          projectID,
		CreatedBy:          uuid.New(),
		IsActive:           true,
		Autonomy:           models.EnhancerAutonomyPropose,
		CronExpression:     &cron,
		AnalysisWindowDays: 7,
		MaxChangesPerRun:   5,
		NextRunAt:          &past,
	}
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{err: errors.New("no user")})

	now := time.Now()
	fired, err := svc.RunDue(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 1, fired)

	cfg, err := repo.GetConfigByProjectID(context.Background(), projectID)
	require.NoError(t, err)
	require.NotNil(t, cfg.NextRunAt)
	require.True(t, cfg.NextRunAt.After(now), "next_run_at обязан уйти в будущее")
	require.NotNil(t, cfg.LastRunAt)

	runs, err := repo.ListRunsByProjectID(context.Background(), projectID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, models.EnhancerRunTriggerCron, runs[0].TriggerKind)

	// Повторный тик до следующего cron-срабатывания ничего не запускает.
	fired, err = svc.RunDue(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 0, fired)
}

// --- propose/finish tool tests ---

func proposeToolResult(t *testing.T, raw json.RawMessage) (status string, payload map[string]any) {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	status, _ = out["status"].(string)
	return status, out
}

func newProposeFixture(t *testing.T, maxChanges int, knownAgents ...uuid.UUID) (*enhancerService, *models.EnhancerRun, agentloop.ToolHandler, *fakeEnhancerRepo) {
	t.Helper()
	repo := newFakeEnhancerRepo()
	agents := map[uuid.UUID]bool{}
	for _, id := range knownAgents {
		agents[id] = true
	}
	svc := &enhancerService{deps: EnhancerServiceDeps{
		Repo:     repo,
		TeamRepo: &fakeEnhancerTeamRepo{agentInProject: agents},
		Logger:   slog.Default(),
	}}
	run := &models.EnhancerRun{ID: uuid.New(), ProjectID: uuid.New()}
	handler := svc.makeProposeChangeHandler(run, maxChanges, &enhancerRunSink{})
	return svc, run, handler, repo
}

func TestEnhancerProposeChange_Validation(t *testing.T) {
	agentID := uuid.New()
	_, _, handler, _ := newProposeFixture(t, 5, agentID)
	ctx := context.Background()
	auth := agentloop.AuthContext{}

	// Невалидный target_kind.
	res, err := handler(ctx, auth, json.RawMessage(`{"target_kind":"global_prompt","payload":{"new":"x"},"reason":"r","expected_effect":"e"}`))
	require.NoError(t, err)
	status, _ := proposeToolResult(t, res)
	require.Equal(t, "validation", status)

	// Пустые reason/expected_effect.
	res, err = handler(ctx, auth, json.RawMessage(`{"target_kind":"project_description","payload":{"new":"x"},"reason":" ","expected_effect":"e"}`))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "validation", status)

	// agent_override без target_agent_id.
	res, err = handler(ctx, auth, json.RawMessage(`{"target_kind":"agent_override","payload":{"prompt_addendum":"x"},"reason":"r","expected_effect":"e"}`))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "validation", status)

	// Чужой агент (не в командах проекта) — граница изоляции.
	foreign := uuid.New()
	res, err = handler(ctx, auth, json.RawMessage(`{"target_kind":"agent_override","target_agent_id":"`+foreign.String()+`","payload":{"prompt_addendum":"x"},"reason":"r","expected_effect":"e"}`))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "forbidden", status)

	// Слишком длинный prompt_addendum.
	long := strings.Repeat("я", EnhancerMaxAddendumChars+1)
	res, err = handler(ctx, auth, json.RawMessage(`{"target_kind":"agent_override","target_agent_id":"`+agentID.String()+`","payload":{"prompt_addendum":"`+long+`"},"reason":"r","expected_effect":"e"}`))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "validation", status)
}

func TestEnhancerProposeChange_LimitAndSuccess(t *testing.T) {
	agentID := uuid.New()
	_, run, handler, repo := newProposeFixture(t, 1, agentID)
	ctx := context.Background()
	auth := agentloop.AuthContext{}

	ok := `{"target_kind":"agent_override","target_agent_id":"` + agentID.String() + `","payload":{"prompt_addendum":"всегда указывай repo_slug"},"reason":"задача abc зациклилась","expected_effect":"меньше шагов роутера"}`
	res, err := handler(ctx, auth, json.RawMessage(ok))
	require.NoError(t, err)
	status, _ := proposeToolResult(t, res)
	require.Equal(t, "ok", status)

	changes, err := repo.ListChangesByRunID(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, models.EnhancerChangeStatusProposed, changes[0].Status)
	require.Equal(t, models.EnhancerChangeKindAgentOverride, changes[0].TargetKind)
	require.NotNil(t, changes[0].TargetAgentID)
	require.Equal(t, agentID, *changes[0].TargetAgentID)

	// Лимит исчерпан — второе предложение отклоняется, но петля не падает.
	res, err = handler(ctx, auth, json.RawMessage(ok))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "limit_exceeded", status)

	changes, err = repo.ListChangesByRunID(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, changes, 1)
}

func TestEnhancerFinishRun_Handler(t *testing.T) {
	sink := &enhancerRunSink{}
	handler := makeFinishRunHandler(sink)
	ctx := context.Background()
	auth := agentloop.AuthContext{}

	res, err := handler(ctx, auth, json.RawMessage(`{"report":""}`))
	require.NoError(t, err)
	status, _ := proposeToolResult(t, res)
	require.Equal(t, "validation", status)
	require.Empty(t, sink.report)

	res, err = handler(ctx, auth, json.RawMessage(`{"report":"## Итог\nдостаточно данных не было"}`))
	require.NoError(t, err)
	status, _ = proposeToolResult(t, res)
	require.Equal(t, "ok", status)
	require.Contains(t, sink.report, "Итог")
}

// --- apply / reject / rollback (фаза 2) ---

// stubEnhancerProjectSvc — ProjectService с управляемым проектом: GetByID
// отдаёт его, Update реально мутирует description/settings (для OCC-сценариев).
// Остальные методы — через embedded nil-интерфейс (паника при случайном вызове).
type stubEnhancerProjectSvc struct {
	ProjectService
	mu      sync.Mutex
	project *models.Project
	updates []dto.UpdateProjectRequest
}

func (s *stubEnhancerProjectSvc) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *s.project
	return &cp, nil
}

func (s *stubEnhancerProjectSvc) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, req)
	if req.Description != nil {
		s.project.Description = *req.Description
	}
	if req.Settings != nil {
		s.project.Settings = *req.Settings
	}
	cp := *s.project
	return &cp, nil
}

func newApplyFixture(t *testing.T, project *models.Project, knownAgents ...uuid.UUID) (*enhancerService, *fakeEnhancerRepo, *stubEnhancerProjectSvc) {
	t.Helper()
	repo := newFakeEnhancerRepo()
	agents := map[uuid.UUID]bool{}
	for _, id := range knownAgents {
		agents[id] = true
	}
	projSvc := &stubEnhancerProjectSvc{project: project}
	svc := &enhancerService{deps: EnhancerServiceDeps{
		Repo:       repo,
		ProjectSvc: projSvc,
		TeamRepo:   &fakeEnhancerTeamRepo{agentInProject: agents},
		Logger:     slog.Default(),
	}}
	return svc, repo, projSvc
}

func seedChange(t *testing.T, repo *fakeEnhancerRepo, projectID uuid.UUID, kind models.EnhancerChangeKind, agentID *uuid.UUID, payload string) *models.EnhancerChange {
	t.Helper()
	ch := &models.EnhancerChange{
		ID:             uuid.New(),
		RunID:          uuid.New(),
		ProjectID:      projectID,
		TargetKind:     kind,
		TargetAgentID:  agentID,
		Payload:        []byte(payload),
		Reason:         "r",
		ExpectedEffect: "e",
		Status:         models.EnhancerChangeStatusProposed,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateChange(context.Background(), ch))
	return ch
}

func TestEnhancerApplyRollback_AgentOverrideRebuild(t *testing.T) {
	projectID := uuid.New()
	agentID := uuid.New()
	userID := uuid.New()
	project := &models.Project{ID: projectID}
	svc, repo, _ := newApplyFixture(t, project, agentID)
	ctx := context.Background()

	ch1 := seedChange(t, repo, projectID, models.EnhancerChangeKindAgentOverride, &agentID, `{"prompt_addendum":"правило один"}`)
	ch2 := seedChange(t, repo, projectID, models.EnhancerChangeKindAgentOverride, &agentID, `{"prompt_addendum":"правило два"}`)

	// Применяем оба — оверрайд = свёртка обоих addendum'ов.
	applied, err := svc.ApplyChange(ctx, userID, models.RoleUser, projectID, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusApplied, applied.Status)
	require.NotNil(t, applied.AppliedAt)

	_, err = svc.ApplyChange(ctx, userID, models.RoleUser, projectID, ch2.ID)
	require.NoError(t, err)

	o, err := repo.GetActiveOverride(ctx, projectID, agentID)
	require.NoError(t, err)
	require.Contains(t, o.PromptAddendum, "правило один")
	require.Contains(t, o.PromptAddendum, "правило два")

	// Повторный apply — bad state.
	_, err = svc.ApplyChange(ctx, userID, models.RoleUser, projectID, ch1.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeBadState)

	// Откат первого — пересборка без него.
	rolled, err := svc.RollbackChange(ctx, userID, models.RoleUser, projectID, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusRolledBack, rolled.Status)
	o, err = repo.GetActiveOverride(ctx, projectID, agentID)
	require.NoError(t, err)
	require.NotContains(t, o.PromptAddendum, "правило один")
	require.Contains(t, o.PromptAddendum, "правило два")

	// Откат второго — оверрайд исчезает целиком.
	_, err = svc.RollbackChange(ctx, userID, models.RoleUser, projectID, ch2.ID)
	require.NoError(t, err)
	_, err = repo.GetActiveOverride(ctx, projectID, agentID)
	require.ErrorIs(t, err, repository.ErrProjectAgentOverrideNotFound)
}

func TestEnhancerApply_AgentLeftProject(t *testing.T) {
	projectID := uuid.New()
	foreignAgent := uuid.New()
	project := &models.Project{ID: projectID}
	svc, repo, _ := newApplyFixture(t, project) // агент НЕ в командах проекта
	ch := seedChange(t, repo, projectID, models.EnhancerChangeKindAgentOverride, &foreignAgent, `{"prompt_addendum":"x"}`)

	_, err := svc.ApplyChange(context.Background(), uuid.New(), models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeConflict)
}

func TestEnhancerApplyRollback_ProjectDescriptionOCC(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()
	project := &models.Project{ID: projectID, Description: "старое описание"}
	svc, repo, projSvc := newApplyFixture(t, project)
	ctx := context.Background()

	ch := seedChange(t, repo, projectID, models.EnhancerChangeKindProjectDescription, nil,
		`{"old":"старое описание","new":"новое описание"}`)

	applied, err := svc.ApplyChange(ctx, userID, models.RoleUser, projectID, ch.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusApplied, applied.Status)
	require.Equal(t, "новое описание", projSvc.project.Description)
	require.Len(t, projSvc.updates, 1)

	// Пользователь правит описание после применения → откат конфликтует.
	projSvc.project.Description = "правка пользователя"
	_, err = svc.RollbackChange(ctx, userID, models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeConflict)

	// Возвращаем применённое значение — откат проходит и восстанавливает old.
	projSvc.project.Description = "новое описание"
	rolled, err := svc.RollbackChange(ctx, userID, models.RoleUser, projectID, ch.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusRolledBack, rolled.Status)
	require.Equal(t, "старое описание", projSvc.project.Description)
}

func TestEnhancerApply_DescriptionConflict(t *testing.T) {
	projectID := uuid.New()
	project := &models.Project{ID: projectID, Description: "уже другое"}
	svc, repo, _ := newApplyFixture(t, project)
	ch := seedChange(t, repo, projectID, models.EnhancerChangeKindProjectDescription, nil,
		`{"old":"старое описание","new":"новое"}`)

	_, err := svc.ApplyChange(context.Background(), uuid.New(), models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeConflict)
}

func TestEnhancerApply_ProjectSettings(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()
	project := &models.Project{ID: projectID} // settings пустые (nil ≈ {})
	svc, repo, projSvc := newApplyFixture(t, project)

	ch := seedChange(t, repo, projectID, models.EnhancerChangeKindProjectSettings, nil,
		`{"old":{},"new":{"max_parallel":2}}`)

	applied, err := svc.ApplyChange(context.Background(), userID, models.RoleUser, projectID, ch.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusApplied, applied.Status)
	require.JSONEq(t, `{"max_parallel":2}`, string(projSvc.project.Settings))
}

func TestEnhancerRejectChange(t *testing.T) {
	projectID := uuid.New()
	project := &models.Project{ID: projectID}
	svc, repo, _ := newApplyFixture(t, project)
	ctx := context.Background()
	ch := seedChange(t, repo, projectID, models.EnhancerChangeKindProjectDescription, nil, `{"old":"","new":"x"}`)

	rejected, err := svc.RejectChange(ctx, uuid.New(), models.RoleUser, projectID, ch.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerChangeStatusRejected, rejected.Status)
	require.NotNil(t, rejected.DecidedAt)

	// Отклонённое нельзя применить или откатить.
	_, err = svc.ApplyChange(ctx, uuid.New(), models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeBadState)
	_, err = svc.RollbackChange(ctx, uuid.New(), models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeBadState)
}

func TestEnhancerApply_ChangeFromOtherProject(t *testing.T) {
	projectID := uuid.New()
	project := &models.Project{ID: projectID}
	svc, repo, _ := newApplyFixture(t, project)
	ch := seedChange(t, repo, uuid.New() /* чужой проект */, models.EnhancerChangeKindProjectDescription, nil, `{"old":"","new":"x"}`)

	_, err := svc.ApplyChange(context.Background(), uuid.New(), models.RoleUser, projectID, ch.ID)
	require.ErrorIs(t, err, ErrEnhancerChangeNotFound)
}

func TestEnhancerListRunChanges_WrongProject(t *testing.T) {
	repo := newFakeEnhancerRepo()
	svc := newTestEnhancerService(t, repo, &fakeUserRepo{})
	ctx := context.Background()

	otherProject := uuid.New()
	runID := uuid.New()
	repo.runs[runID] = &models.EnhancerRun{ID: runID, ProjectID: otherProject, Status: models.EnhancerRunStatusDone}

	_, err := svc.ListRunChanges(ctx, uuid.New(), models.RoleUser, uuid.New(), runID)
	require.ErrorIs(t, err, ErrEnhancerRunNotFound, "прогон чужого проекта не должен читаться")
}
