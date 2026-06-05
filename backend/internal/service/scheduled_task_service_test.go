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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeScheduledTaskRepo struct {
	items   map[uuid.UUID]*models.ScheduledTask
	createErr error
}

func newFakeScheduledTaskRepo() *fakeScheduledTaskRepo {
	return &fakeScheduledTaskRepo{items: map[uuid.UUID]*models.ScheduledTask{}}
}

func (f *fakeScheduledTaskRepo) Create(ctx context.Context, st *models.ScheduledTask) error {
	if f.createErr != nil {
		return f.createErr
	}
	cp := *st
	f.items[st.ID] = &cp
	return nil
}

func (f *fakeScheduledTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledTask, error) {
	st, ok := f.items[id]
	if !ok {
		return nil, repository.ErrScheduledTaskNotFound
	}
	cp := *st
	return &cp, nil
}

func (f *fakeScheduledTaskRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.ScheduledTask, error) {
	var out []models.ScheduledTask
	for _, st := range f.items {
		if st.ProjectID == projectID {
			out = append(out, *st)
		}
	}
	return out, nil
}

func (f *fakeScheduledTaskRepo) Update(ctx context.Context, st *models.ScheduledTask) error {
	if _, ok := f.items[st.ID]; !ok {
		return repository.ErrScheduledTaskNotFound
	}
	cp := *st
	f.items[st.ID] = &cp
	return nil
}

func (f *fakeScheduledTaskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return repository.ErrScheduledTaskNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeScheduledTaskRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]models.ScheduledTask, error) {
	var out []models.ScheduledTask
	for _, st := range f.items {
		if st.IsActive && st.NextRunAt != nil && !st.NextRunAt.After(now) {
			out = append(out, *st)
		}
	}
	return out, nil
}

type fakeUserRepo struct {
	user *models.User
	err  error
}

func (f *fakeUserRepo) Create(ctx context.Context, user *models.User) error { return nil }
func (f *fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}
func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeUserRepo) Update(ctx context.Context, user *models.User) error { return nil }
func (f *fakeUserRepo) Delete(ctx context.Context, id uuid.UUID) error      { return nil }

// --- helpers ---

// mockProjectSvc.GetByID — no-op stub, возвращает (nil,nil) → доступ к проекту разрешён.
func okProjectSvc() *mockProjectSvc { return new(mockProjectSvc) }

// --- pure helper tests ---

func TestParseCron(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"daily 9am", "0 9 * * *", false},
		{"weekdays", "0 9 * * 1-5", false},
		{"every 15m", "*/15 * * * *", false},
		{"empty", "", true},
		{"garbage", "not a cron", true},
		{"too many fields", "0 9 * * * *", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sched, err := parseCron(tc.expr)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrScheduledTaskInvalidCron)
				return
			}
			require.NoError(t, err)
			next := sched.Next(time.Now())
			require.True(t, next.After(time.Now()), "next run must be in the future")
		})
	}
}

func TestValidateScheduledTaskName(t *testing.T) {
	require.ErrorIs(t, validateScheduledTaskName(""), ErrScheduledTaskInvalidName)
	require.ErrorIs(t, validateScheduledTaskName("   "), ErrScheduledTaskInvalidName)
	require.NoError(t, validateScheduledTaskName("Nightly refactor"))
}

// --- Create tests ---

func TestScheduledTaskService_Create_HappyPath(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()
	repo := newFakeScheduledTaskRepo()
	projSvc := okProjectSvc()

	svc := NewScheduledTaskService(repo, nil, projSvc, nil, nil, nil, nil)

	st, err := svc.Create(context.Background(), userID, models.RoleUser, projectID, dto.CreateScheduledTaskRequest{
		Name:           "Nightly refactor",
		Description:    "do the thing",
		CronExpression: "0 3 * * *",
	})
	require.NoError(t, err)
	require.Equal(t, "Nightly refactor", st.Name)
	require.Equal(t, models.TaskPriorityMedium, st.Priority)
	require.True(t, st.IsActive)
	require.Equal(t, userID, st.CreatedBy)
	require.NotNil(t, st.NextRunAt, "active schedule must have next_run_at")
	require.True(t, st.NextRunAt.After(time.Now()))
}

func TestScheduledTaskService_Create_Validation(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()

	cases := []struct {
		name    string
		req     dto.CreateScheduledTaskRequest
		wantErr error
	}{
		{"empty name", dto.CreateScheduledTaskRequest{Name: "", CronExpression: "0 9 * * *"}, ErrScheduledTaskInvalidName},
		{"bad cron", dto.CreateScheduledTaskRequest{Name: "x", CronExpression: "nope"}, ErrScheduledTaskInvalidCron},
		{"bad priority", dto.CreateScheduledTaskRequest{Name: "x", CronExpression: "0 9 * * *", Priority: "urgent"}, ErrTaskInvalidPriority},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeScheduledTaskRepo()
			projSvc := okProjectSvc()
			svc := NewScheduledTaskService(repo, nil, projSvc, nil, nil, nil, nil)
			_, err := svc.Create(context.Background(), userID, models.RoleUser, projectID, tc.req)
			require.ErrorIs(t, err, tc.wantErr)
			require.Empty(t, repo.items, "nothing must be persisted on validation error")
		})
	}
}

func TestScheduledTaskService_Create_InactiveNoNextRun(t *testing.T) {
	projectID := uuid.New()
	repo := newFakeScheduledTaskRepo()
	projSvc := okProjectSvc()
	svc := NewScheduledTaskService(repo, nil, projSvc, nil, nil, nil, nil)

	inactive := false
	st, err := svc.Create(context.Background(), uuid.New(), models.RoleUser, projectID, dto.CreateScheduledTaskRequest{
		Name:           "paused one",
		CronExpression: "0 3 * * *",
		IsActive:       &inactive,
	})
	require.NoError(t, err)
	require.False(t, st.IsActive)
	require.Nil(t, st.NextRunAt, "inactive schedule must not be due")
}

// --- RunDue test ---

func TestScheduledTaskService_RunDue_FiresAndAdvances(t *testing.T) {
	projectID := uuid.New()
	ownerID := uuid.New()
	repo := newFakeScheduledTaskRepo()

	past := time.Now().Add(-time.Minute)
	stID := uuid.New()
	repo.items[stID] = &models.ScheduledTask{
		ID:             stID,
		ProjectID:      projectID,
		CreatedBy:      ownerID,
		Name:           "tick",
		Description:    "body",
		CronExpression: "*/5 * * * *",
		Priority:       models.TaskPriorityHigh,
		IsActive:       true,
		NextRunAt:      &past,
	}

	taskSvc := new(mockTaskSvc)
	createdTask := &models.Task{ID: uuid.New(), ProjectID: projectID}
	taskSvc.On("Create", mock.Anything, ownerID, models.RoleUser, projectID, mock.MatchedBy(func(req dto.CreateTaskRequest) bool {
		return req.Title == "tick" && req.Description == "body" && req.Priority == string(models.TaskPriorityHigh)
	})).Return(createdTask, nil)

	orch := new(mockOrchestratorSvc)
	// mockOrchestratorSvc.EnqueueInitialStep делегирует на ProcessTask (legacy fixtures).
	orch.On("ProcessTask", mock.Anything, createdTask.ID).Return(nil)

	userRepo := &fakeUserRepo{user: &models.User{ID: ownerID, Role: models.RoleUser}}

	svc := NewScheduledTaskService(repo, taskSvc, nil, nil, userRepo, orch, nil)

	fired, err := svc.RunDue(context.Background(), time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, fired)

	updated := repo.items[stID]
	require.NotNil(t, updated.LastRunAt)
	require.NotNil(t, updated.NextRunAt)
	require.True(t, updated.NextRunAt.After(time.Now()), "next_run_at must be advanced into the future")

	taskSvc.AssertExpectations(t)
	orch.AssertExpectations(t)
}
