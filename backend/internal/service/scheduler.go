package service

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
)

type Scheduler interface {
	Start(ctx context.Context) error
	Stop()
}

type scheduler struct {
	cron         *cron.Cron
	repo         repository.WorkflowRepository
	engine       WorkflowEngine
	modelService ModelCatalogService // Опционально, может быть nil
}

func NewScheduler(repo repository.WorkflowRepository, engine WorkflowEngine, modelService ModelCatalogService) Scheduler {
	return &scheduler{
		cron:         cron.New(),
		repo:         repo,
		engine:       engine,
		modelService: modelService,
	}
}

func (s *scheduler) Start(ctx context.Context) error {
	log.Println("Starting Scheduler...")

	// 1. Системные задачи
	if s.modelService != nil {
		// Синхронизация моделей раз в сутки (в 3 часа ночи)
		_, err := s.cron.AddFunc("0 3 * * *", func() {
			log.Println("Running scheduled model sync...")
			ctx := context.Background()
			if err := s.modelService.SyncOpenRouterModels(ctx); err != nil {
				log.Printf("Failed to sync models: %v", err)
			}
		})
		if err != nil {
			log.Printf("Failed to schedule model sync: %v", err)
		} else {
			log.Println("Scheduled model sync (daily at 03:00)")
		}
	}

	// 2. Пользовательские задачи (Workflow Schedules)
	schedules, err := s.repo.ListActiveSchedules(ctx)
	if err != nil {
		return err
	}

	for _, schedule := range schedules {
		if err := s.addJob(schedule); err != nil {
			log.Printf("Failed to schedule job %s: %v", schedule.Name, err)
			continue
		}
		log.Printf("Scheduled job: %s (%s)", schedule.Name, schedule.CronExpression)
	}

	s.cron.Start()
	return nil
}

func (s *scheduler) Stop() {
	s.cron.Stop()
}

func (s *scheduler) addJob(schedule models.ScheduledWorkflow) error {
	_, err := s.cron.AddFunc(schedule.CronExpression, func() {
		log.Printf("Executing scheduled job: %s", schedule.Name)

		ctx := context.Background()

		input := schedule.InputTemplate

		execution, err := s.engine.StartWorkflow(ctx, schedule.WorkflowName, input)
		if err != nil {
			log.Printf("Failed to start scheduled workflow %s: %v", schedule.WorkflowName, err)
			return
		}

		now := time.Now()
		toUpdate := schedule
		toUpdate.LastRunAt = &now

		if err := s.repo.UpdateSchedule(ctx, &toUpdate); err != nil {
			log.Printf("Failed to update schedule last run time: %v", err)
		}

		log.Printf("Started execution %s for job %s", execution.ID, schedule.Name)
	})
	return err
}
