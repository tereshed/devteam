package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

var (
	ErrScheduledTaskNotFound    = errors.New("scheduled task not found")
	ErrScheduledTaskInvalidName = errors.New("scheduled task name is required")
	ErrScheduledTaskInvalidCron = errors.New("invalid cron expression")
)

const scheduledTaskNameMaxLen = 500

// ScheduledTaskService — бизнес-логика регулярных (cron) задач проекта.
type ScheduledTaskService interface {
	Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateScheduledTaskRequest) (*models.ScheduledTask, error)
	List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ScheduledTask, error)
	GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) (*models.ScheduledTask, error)
	Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID, req dto.UpdateScheduledTaskRequest) (*models.ScheduledTask, error)
	Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) error
	// RunDue — вызывается leader-gated раннером: создаёт задачи по всем созревшим
	// расписаниям и пересчитывает их next_run_at. Возвращает число запущенных задач.
	RunDue(ctx context.Context, now time.Time) (int, error)
}

type scheduledTaskService struct {
	repo         repository.ScheduledTaskRepository
	taskSvc      TaskService
	projectSvc   ProjectService
	teamSvc      TeamService
	userRepo     repository.UserRepository
	orchestrator TaskOrchestrator
	logger       *slog.Logger
}

// NewScheduledTaskService создаёт сервис регулярных задач.
func NewScheduledTaskService(
	repo repository.ScheduledTaskRepository,
	taskSvc TaskService,
	projectSvc ProjectService,
	teamSvc TeamService,
	userRepo repository.UserRepository,
	orchestrator TaskOrchestrator,
	logger *slog.Logger,
) ScheduledTaskService {
	if logger == nil {
		logger = slog.Default()
	}
	return &scheduledTaskService{
		repo:         repo,
		taskSvc:      taskSvc,
		projectSvc:   projectSvc,
		teamSvc:      teamSvc,
		userRepo:     userRepo,
		orchestrator: orchestrator,
		logger:       logger,
	}
}

// cronParser — стандартный 5-польный парсер (minute hour dom month dow), как у
// большинства cron-выражений из UI-пресетов.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// parseCron валидирует cron-выражение и возвращает Schedule для вычисления next-run.
func parseCron(expr string) (cron.Schedule, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, ErrScheduledTaskInvalidCron
	}
	sched, err := cronParser.Parse(trimmed)
	if err != nil {
		return nil, ErrScheduledTaskInvalidCron
	}
	return sched, nil
}

func validateScheduledTaskName(name string) error {
	t := strings.TrimSpace(name)
	if t == "" || len(t) > scheduledTaskNameMaxLen {
		return ErrScheduledTaskInvalidName
	}
	return nil
}

// assertTeamInProject проверяет, что команда принадлежит проекту.
func (s *scheduledTaskService) assertTeamInProject(ctx context.Context, projectID, teamID uuid.UUID) error {
	teams, err := s.teamSvc.ListByProjectID(ctx, projectID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if t.ID == teamID {
			return nil
		}
	}
	return ErrTeamNotInProject
}

func (s *scheduledTaskService) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateScheduledTaskRequest) (*models.ScheduledTask, error) {
	if _, err := s.projectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	if err := validateScheduledTaskName(req.Name); err != nil {
		return nil, err
	}
	sched, err := parseCron(req.CronExpression)
	if err != nil {
		return nil, err
	}
	priority, err := parseTaskPriority(req.Priority)
	if err != nil {
		return nil, err
	}
	if req.TeamID != nil {
		if err := s.assertTeamInProject(ctx, projectID, *req.TeamID); err != nil {
			return nil, err
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	st := &models.ScheduledTask{
		ID:             uuid.New(),
		ProjectID:      projectID,
		TeamID:         req.TeamID,
		CreatedBy:      userID,
		Name:           strings.TrimSpace(req.Name),
		Description:    req.Description,
		CronExpression: strings.TrimSpace(req.CronExpression),
		Priority:       priority,
		IsActive:       isActive,
	}
	if isActive {
		next := sched.Next(time.Now())
		st.NextRunAt = &next
	}

	if err := s.repo.Create(ctx, st); err != nil {
		return nil, mapScheduledTaskRepoErr(err)
	}
	return st, nil
}

func (s *scheduledTaskService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ScheduledTask, error) {
	if _, err := s.projectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *scheduledTaskService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) (*models.ScheduledTask, error) {
	if _, err := s.projectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	st, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapScheduledTaskRepoErr(err)
	}
	if st.ProjectID != projectID {
		return nil, ErrScheduledTaskNotFound
	}
	return st, nil
}

func (s *scheduledTaskService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID, req dto.UpdateScheduledTaskRequest) (*models.ScheduledTask, error) {
	st, err := s.GetByID(ctx, userID, userRole, projectID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if err := validateScheduledTaskName(*req.Name); err != nil {
			return nil, err
		}
		st.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		st.Description = *req.Description
	}
	if req.Priority != nil {
		priority, err := parseTaskPriority(*req.Priority)
		if err != nil {
			return nil, err
		}
		st.Priority = priority
	}
	if req.ClearTeam {
		st.TeamID = nil
	} else if req.TeamID != nil {
		if err := s.assertTeamInProject(ctx, projectID, *req.TeamID); err != nil {
			return nil, err
		}
		st.TeamID = req.TeamID
	}

	// cron / is_active влияют на next_run_at — пересчитываем при изменении любого из них.
	cronChanged := false
	if req.CronExpression != nil {
		if _, err := parseCron(*req.CronExpression); err != nil {
			return nil, err
		}
		st.CronExpression = strings.TrimSpace(*req.CronExpression)
		cronChanged = true
	}
	if req.IsActive != nil {
		st.IsActive = *req.IsActive
		cronChanged = true
	}
	if cronChanged {
		if st.IsActive {
			sched, err := parseCron(st.CronExpression)
			if err != nil {
				return nil, err
			}
			next := sched.Next(time.Now())
			st.NextRunAt = &next
		} else {
			st.NextRunAt = nil
		}
	}

	if err := s.repo.Update(ctx, st); err != nil {
		return nil, mapScheduledTaskRepoErr(err)
	}
	return st, nil
}

func (s *scheduledTaskService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) error {
	if _, err := s.GetByID(ctx, userID, userRole, projectID, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapScheduledTaskRepoErr(err)
	}
	return nil
}

func (s *scheduledTaskService) RunDue(ctx context.Context, now time.Time) (int, error) {
	due, err := s.repo.ListDue(ctx, now, 0)
	if err != nil {
		return 0, err
	}
	fired := 0
	for i := range due {
		st := due[i]
		if s.fireOne(ctx, &st, now) {
			fired++
		}
		// next_run_at пересчитываем всегда (даже при ошибке создания задачи),
		// чтобы не зациклить раннер на «сбойном» расписании.
		s.advanceNextRun(ctx, &st, now)
	}
	return fired, nil
}

// fireOne создаёт обычную задачу из расписания. Возвращает true при успехе.
func (s *scheduledTaskService) fireOne(ctx context.Context, st *models.ScheduledTask, now time.Time) bool {
	user, err := s.userRepo.GetByID(ctx, st.CreatedBy)
	if err != nil {
		s.logger.Error("scheduled task: owner lookup failed", "scheduled_task_id", st.ID, "error", err)
		return false
	}

	task, err := s.taskSvc.Create(ctx, st.CreatedBy, user.Role, st.ProjectID, dto.CreateTaskRequest{
		Title:       st.Name,
		Description: st.Description,
		Priority:    string(st.Priority),
		TeamID:      st.TeamID,
	})
	if err != nil {
		s.logger.Error("scheduled task: create task failed", "scheduled_task_id", st.ID, "error", err)
		return false
	}

	if s.orchestrator != nil {
		if err := s.orchestrator.EnqueueInitialStep(ctx, task.ID); err != nil {
			s.logger.Error("scheduled task: enqueue initial step failed", "scheduled_task_id", st.ID, "task_id", task.ID, "error", err)
			// Задача создана — это всё ещё «выстрел». Оркестрация подхватится ретеншеном/ручным запуском.
		}
	}
	st.LastRunAt = &now
	s.logger.Info("scheduled task fired", "scheduled_task_id", st.ID, "task_id", task.ID, "project_id", st.ProjectID)
	return true
}

// advanceNextRun пересчитывает next_run_at (или гасит расписание при невалидном cron) и сохраняет.
func (s *scheduledTaskService) advanceNextRun(ctx context.Context, st *models.ScheduledTask, now time.Time) {
	sched, err := parseCron(st.CronExpression)
	if err != nil {
		// Невалидный cron в БД — деактивируем, чтобы не выбирать строку каждую минуту.
		s.logger.Error("scheduled task: invalid cron, deactivating", "scheduled_task_id", st.ID, "cron", st.CronExpression)
		st.IsActive = false
		st.NextRunAt = nil
	} else {
		next := sched.Next(now)
		st.NextRunAt = &next
	}
	if err := s.repo.Update(ctx, st); err != nil {
		s.logger.Error("scheduled task: update next_run failed", "scheduled_task_id", st.ID, "error", err)
	}
}

func mapScheduledTaskRepoErr(err error) error {
	switch {
	case errors.Is(err, repository.ErrScheduledTaskNotFound):
		return ErrScheduledTaskNotFound
	default:
		return err
	}
}

// scheduledTaskRunner — leader-gated тикер, периодически вызывающий RunDue.
type scheduledTaskRunner struct {
	svc      ScheduledTaskService
	interval time.Duration
	logger   *slog.Logger
}

// NewScheduledTaskRunner создаёт раннер регулярных задач. interval<=0 → 1 минута.
func NewScheduledTaskRunner(svc ScheduledTaskService, interval time.Duration, logger *slog.Logger) *scheduledTaskRunner {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &scheduledTaskRunner{svc: svc, interval: interval, logger: logger}
}

// Run блокируется до отмены ctx, периодически запуская созревшие расписания.
func (r *scheduledTaskRunner) Run(ctx context.Context) {
	r.logger.Info("scheduled task runner started", "interval", r.interval.String())
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	// Первый прогон сразу, не дожидаясь первого тика.
	r.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("scheduled task runner stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *scheduledTaskRunner) tick(ctx context.Context) {
	fired, err := r.svc.RunDue(ctx, time.Now())
	if err != nil {
		r.logger.Error("scheduled task runner tick failed", "error", err)
		return
	}
	if fired > 0 {
		r.logger.Info("scheduled task runner tick", "fired", fired)
	}
}
