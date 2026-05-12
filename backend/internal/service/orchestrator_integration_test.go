//go:build integration

// E2E backend test (Sprint 14.1):
// создать проект → отправить запрос (создать задачу) → Orchestrator проводит её
// через Planner → Developer → Reviewer → Tester → Completed.
//
// Использует реальные репозитории и сервисы поверх YugabyteDB (как остальные
// integration-тесты в этом пакете); LLM/Sandbox executors замокированы — мы
// проверяем сквозную работу оркестрации статусов в БД, а не сам LLM/Docker.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// orchestratorE2ESetup собирает реальный стек репозиториев/сервисов и тестовые сущности.
type orchestratorE2ESetup struct {
	db           *gorm.DB
	user         *models.User
	project      *models.Project
	team         *models.Team
	agents       map[models.AgentRole]*models.Agent
	taskRepo     repository.TaskRepository
	taskMsgRepo  repository.TaskMessageRepository
	teamRepo     repository.TeamRepository
	workflowRepo repository.WorkflowRepository
	txManager    repository.TransactionManager
	taskService  TaskService
	projectSvc   ProjectService
	orch         OrchestratorService
}

func orchestratorIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "connect to test database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	return db
}

// cleanupOrchestratorE2E удаляет сущности теста (изоляция между прогонами).
func cleanupOrchestratorE2E(t *testing.T, db *gorm.DB, userID, projectID uuid.UUID) {
	t.Helper()
	// Порядок важен: дочерние перед родительскими (FK).
	require.NoError(t, db.Exec(`DELETE FROM task_messages WHERE task_id IN (SELECT id FROM tasks WHERE project_id = ?)`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM tasks WHERE project_id = ?`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM agent_tool_bindings WHERE agent_id IN (SELECT id FROM agents WHERE team_id IN (SELECT id FROM teams WHERE project_id = ?))`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM agents WHERE team_id IN (SELECT id FROM teams WHERE project_id = ?)`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM teams WHERE project_id = ?`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM projects WHERE id = ?`, projectID).Error)
	require.NoError(t, db.Exec(`DELETE FROM users WHERE id = ?`, userID).Error)
}

// scriptedAgentExecutor — фейковый исполнитель, который смотрит на роль из
// ExecutionInput и возвращает заранее заготовленный для этой роли результат.
// Дополнительно переводит assigned_agent_id на следующего по pipeline агента
// прямо в БД, чтобы Transition оркестратора отображал реальное «передаём
// задачу следующему агенту» — в текущем коде opts.AssignedAgentID оркестратор
// не выставляет, см. orchestrator_service.go:handleExecutionResult.
type scriptedAgentExecutor struct {
	db        *gorm.DB
	agents    map[models.AgentRole]*models.Agent
	callOrder *[]string
	calls     *atomic.Int32
}

func (e *scriptedAgentExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	e.calls.Add(1)
	*e.callOrder = append(*e.callOrder, in.Role)

	var (
		nextRole  models.AgentRole
		artifacts json.RawMessage
		output    string
	)
	switch models.AgentRole(in.Role) {
	case models.AgentRoleOrchestrator:
		nextRole = models.AgentRolePlanner
		artifacts = json.RawMessage(`{"plan":"decompose"}`)
		output = "orchestrator: analysed user request"
	case models.AgentRolePlanner:
		nextRole = models.AgentRoleDeveloper
		artifacts = json.RawMessage(`{"steps":["impl","tests"]}`)
		output = "planner: decomposed task into subtasks"
	case models.AgentRoleDeveloper:
		nextRole = models.AgentRoleReviewer
		artifacts = json.RawMessage(`{"diff":"+ new line"}`)
		output = "developer: implemented feature"
	case models.AgentRoleReviewer:
		nextRole = models.AgentRoleTester
		artifacts = json.RawMessage(`{"decision":"approved"}`)
		output = "reviewer: approved change"
	case models.AgentRoleTester:
		nextRole = "" // terminal
		artifacts = json.RawMessage(`{"decision":"passed"}`)
		output = "tester: tests green"
	default:
		return nil, errors.New("scripted executor: unknown role " + in.Role)
	}

	// Перенаправляем задачу на следующего агента до того, как Orchestrator
	// сделает Transition. Если nextRole пуст — это терминальный шаг.
	if nextRole != "" {
		nextAgent, ok := e.agents[nextRole]
		if !ok {
			return nil, errors.New("scripted executor: no agent for role " + string(nextRole))
		}
		taskID, err := uuid.Parse(in.TaskID)
		if err != nil {
			return nil, err
		}
		// Прямое обновление в обход repo, чтобы не конкурировать с optimistic
		// locking (Transition сделает свой GetByID после нашего апдейта).
		err = e.db.WithContext(ctx).
			Model(&models.Task{}).
			Where("id = ?", taskID).
			Updates(map[string]interface{}{
				"assigned_agent_id": nextAgent.ID,
				"updated_at":        time.Now().UTC(),
			}).Error
		if err != nil {
			return nil, err
		}
	}

	return &agent.ExecutionResult{
		Success:       true,
		Output:        output,
		ArtifactsJSON: artifacts,
	}, nil
}

// noopCodeIndexer — оркестратор может вызывать SearchContext; нам не нужен Weaviate.
type noopCodeIndexer struct{}

func (noopCodeIndexer) IndexProject(ctx context.Context, req interface{}) error { return nil }

// noopSandboxStopper — задача не отменяется, остановка не нужна.
type noopSandboxStopper struct{}

func (noopSandboxStopper) StopTask(ctx context.Context, taskID string) error { return nil }

func setupOrchestratorE2E(t *testing.T) *orchestratorE2ESetup {
	t.Helper()
	ctx := context.Background()
	db := orchestratorIntegrationDB(t)

	// 1. Пользователь
	userRepo := repository.NewUserRepository(db)
	user := &models.User{
		Email:        "orch-e2e-" + uuid.NewString() + "@example.com",
		PasswordHash: "hashed",
		Role:         models.RoleUser,
	}
	require.NoError(t, userRepo.Create(ctx, user))

	// 2. Проект (создаём напрямую — без git clone/index хуков ProjectService.Create)
	projectRepo := repository.NewProjectRepository(db)
	project := &models.Project{
		Name:        "orch-e2e-proj-" + uuid.NewString()[:8],
		Description: "E2E orchestrator integration test",
		GitProvider: models.GitProviderLocal,
		UserID:      user.ID,
		Status:      models.ProjectStatusActive,
	}
	require.NoError(t, projectRepo.Create(ctx, project))

	// 3. Команда
	teamRepo := repository.NewTeamRepository(db)
	team := &models.Team{
		Name:      "team-orch-e2e",
		ProjectID: project.ID,
		Type:      models.TeamTypeDevelopment,
	}
	require.NoError(t, teamRepo.Create(ctx, team))

	// 4. Агенты на все pipeline-роли
	claudeBackend := models.CodeBackendClaudeCode
	roles := []models.AgentRole{
		models.AgentRoleOrchestrator,
		models.AgentRolePlanner,
		models.AgentRoleDeveloper,
		models.AgentRoleReviewer,
		models.AgentRoleTester,
	}
	agents := make(map[models.AgentRole]*models.Agent, len(roles))
	for _, role := range roles {
		a := &models.Agent{
			Name:     "agent-" + string(role) + "-" + uuid.NewString()[:6],
			Role:     role,
			TeamID:   &team.ID,
			Skills:   datatypes.JSON([]byte("[]")),
			Settings: datatypes.JSON([]byte("{}")),
			IsActive: true,
		}
		if role == models.AgentRoleDeveloper || role == models.AgentRoleTester {
			a.CodeBackend = &claudeBackend
		}
		require.NoError(t, db.WithContext(ctx).Create(a).Error)
		agents[role] = a
	}

	// 5. Репозитории + сервисы оркестратора
	taskRepo := repository.NewTaskRepository(db)
	taskMsgRepo := repository.NewTaskMessageRepository(db)
	workflowRepo := repository.NewWorkflowRepository(db)
	gitCredRepo := repository.NewGitCredentialRepository(db)
	toolDefRepo := repository.NewToolDefinitionRepository(db)
	txManager := repository.NewTransactionManager(db)
	eventBus := events.NewInMemoryBus(nil, nil)

	projectSvc := NewProjectService(projectRepo, teamRepo, gitCredRepo, txManager, nil, NoopEncryptor{}, eventBus, nil, "")
	teamSvc := NewTeamService(teamRepo, toolDefRepo)
	taskSvc := NewTaskService(taskRepo, taskMsgRepo, projectSvc, teamSvc, txManager, eventBus, nil, slog.Default())

	pipe := NewPipelineEngine(5)
	ctxBuilder := NewContextBuilder(NoopEncryptor{}, nil, nil)

	return &orchestratorE2ESetup{
		db:           db,
		user:         user,
		project:      project,
		team:         team,
		agents:       agents,
		taskRepo:     taskRepo,
		taskMsgRepo:  taskMsgRepo,
		teamRepo:     teamRepo,
		workflowRepo: workflowRepo,
		txManager:    txManager,
		taskService:  taskSvc,
		projectSvc:   projectSvc,
		orch: NewOrchestratorService(
			taskRepo, taskMsgRepo, workflowRepo, projectSvc, txManager,
			nil, nil, // executors подменим ниже
			taskSvc, pipe, ctxBuilder, nil,
			noopSandboxStopper{}, nil,
			WithStepPollInterval(0),
		),
	}
}

// TestOrchestratorE2E_FullPipelineToCompleted — основной E2E-сценарий 14.1.
// Запускается через `go test -tags=integration` (см. Makefile: make test-integration).
func TestOrchestratorE2E_FullPipelineToCompleted(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Пользователь «отправляет запрос» — создаём задачу через TaskService
	//    (имитируем то, что делает ConversationService при новом сообщении).
	orchAgentID := s.agents[models.AgentRoleOrchestrator].ID
	createReq := dto.CreateTaskRequest{
		Title:           "Add hello-world endpoint",
		Description:     "Реализовать новый эндпоинт GET /hello, возвращающий 200 OK.",
		AssignedAgentID: &orchAgentID,
	}
	task, err := s.taskService.Create(ctx, s.user.ID, models.RoleUser, s.project.ID, createReq)
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusPending, task.Status)
	require.NotNil(t, task.AssignedAgentID)

	// 2. Подключаем сфабрикованного исполнителя для LLM- и Sandbox-агентов.
	//    Один и тот же экземпляр — реальная маршрутизация по role внутри pipeline.
	callOrder := make([]string, 0, 5)
	var calls atomic.Int32
	executor := &scriptedAgentExecutor{
		db:        s.db,
		agents:    s.agents,
		callOrder: &callOrder,
		calls:     &calls,
	}

	pipe := NewPipelineEngine(5)
	ctxBuilder := NewContextBuilder(NoopEncryptor{}, nil, nil)
	txManager := repository.NewTransactionManager(s.db)
	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, txManager,
		executor, executor,
		s.taskService, pipe, ctxBuilder, nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
	)

	// 3. Запускаем оркестратор — он должен последовательно прогнать pipeline.
	err = orch.ProcessTask(ctx, task.ID)
	require.NoError(t, err, "ProcessTask should run pipeline to terminal status")

	// 4. Проверяем все 5 шагов pipeline в правильном порядке.
	require.Equal(t, int32(5), calls.Load(), "expected exactly 5 agent invocations (orchestrator→planner→developer→reviewer→tester)")
	require.Equal(t, []string{
		string(models.AgentRoleOrchestrator),
		string(models.AgentRolePlanner),
		string(models.AgentRoleDeveloper),
		string(models.AgentRoleReviewer),
		string(models.AgentRoleTester),
	}, callOrder)

	// 5. Проверяем итоговое состояние задачи в БД.
	final, err := s.taskRepo.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusCompleted, final.Status, "task should be completed after full pipeline")
	require.NotNil(t, final.CompletedAt, "completed_at must be set on terminal status")
	require.NotNil(t, final.Result, "result must be persisted on final transition")
	require.NotEmpty(t, *final.Result)

	// 6. Каждый шаг pipeline должен оставить сообщение от агента в task_messages.
	msgs, _, err := s.taskMsgRepo.ListByTaskID(context.Background(), task.ID, repository.TaskMessageFilter{Limit: 50})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(msgs), 5, "expected at least one task_message per pipeline step")
	agentMsgs := 0
	for _, m := range msgs {
		if m.SenderType == models.SenderTypeAgent {
			agentMsgs++
		}
	}
	require.Equal(t, 5, agentMsgs, "exactly one agent message per pipeline step")
}

// TestOrchestratorE2E_ReviewerApprovesAdvancesToTester проверяет, что одобрение
// ревьюера ведёт ровно в Testing (не в ChangesRequested) и далее в Completed.
// Это эксплицитная гарантия пункта «Reviewer одобряет → Completed» из 14.1.
func TestOrchestratorE2E_ReviewerApprovesAdvancesToTester(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Стартуем сразу с review-статуса, чтобы изолировать пункт «Reviewer → ...».
	revAgentID := s.agents[models.AgentRoleReviewer].ID
	branch := "feature/orch-e2e"
	task := &models.Task{
		ProjectID:       s.project.ID,
		Title:           "Review only",
		Description:     "Уже разработано — нужен только review и tests.",
		Status:          models.TaskStatusReview,
		Priority:        models.TaskPriorityMedium,
		AssignedAgentID: &revAgentID,
		CreatedByType:   models.CreatedByUser,
		CreatedByID:     s.user.ID,
		Context:         datatypes.JSON([]byte("{}")),
		Artifacts:       datatypes.JSON([]byte("{}")),
		BranchName:      &branch,
	}
	require.NoError(t, s.taskRepo.Create(context.Background(), task))

	callOrder := make([]string, 0, 2)
	var calls atomic.Int32
	executor := &scriptedAgentExecutor{
		db:        s.db,
		agents:    s.agents,
		callOrder: &callOrder,
		calls:     &calls,
	}

	pipe := NewPipelineEngine(5)
	ctxBuilder := NewContextBuilder(NoopEncryptor{}, nil, nil)
	txManager := repository.NewTransactionManager(s.db)
	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, txManager,
		executor, executor,
		s.taskService, pipe, ctxBuilder, nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
	)

	require.NoError(t, orch.ProcessTask(ctx, task.ID))
	require.Equal(t, []string{
		string(models.AgentRoleReviewer),
		string(models.AgentRoleTester),
	}, callOrder)

	final, err := s.taskRepo.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusCompleted, final.Status)
}

// passiveAgentExecutor — простой исполнитель: возвращает заготовленный по роли
// результат и НЕ трогает assigned_agent_id в БД. Используется для проверки,
// что переключение агента действительно делает оркестратор (WithTeamRepository).
type passiveAgentExecutor struct {
	callOrder *[]string
	calls     *atomic.Int32
}

func (e *passiveAgentExecutor) Execute(_ context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	e.calls.Add(1)
	*e.callOrder = append(*e.callOrder, in.Role)
	var artifacts json.RawMessage
	output := in.Role + ": ok"
	switch models.AgentRole(in.Role) {
	case models.AgentRoleOrchestrator:
		artifacts = json.RawMessage(`{"plan":"decompose"}`)
	case models.AgentRolePlanner:
		artifacts = json.RawMessage(`{"steps":["impl","tests"]}`)
	case models.AgentRoleDeveloper:
		artifacts = json.RawMessage(`{"diff":"+ new line","branch_name":"feature/x"}`)
	case models.AgentRoleReviewer:
		artifacts = json.RawMessage(`{"decision":"approved"}`)
	case models.AgentRoleTester:
		artifacts = json.RawMessage(`{"decision":"passed"}`)
	default:
		return nil, errors.New("passive executor: unknown role " + in.Role)
	}
	return &agent.ExecutionResult{Success: true, Output: output, ArtifactsJSON: artifacts}, nil
}

// TestOrchestratorE2E_WithTeamRepository_AutoAdvancesAgent — оркестратор сам
// (через WithTeamRepository → resolveNextAgentID) проставляет AssignedAgentID на
// агента следующей по pipeline роли. Без scripted-executor'а pipeline должен
// проходить только за счёт этого механизма.
func TestOrchestratorE2E_WithTeamRepository_AutoAdvancesAgent(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	orchAgentID := s.agents[models.AgentRoleOrchestrator].ID
	task, err := s.taskService.Create(ctx, s.user.ID, models.RoleUser, s.project.ID, dto.CreateTaskRequest{
		Title:           "Auto-advance smoke",
		Description:     "Verify orchestrator switches assigned_agent_id by role at each transition.",
		AssignedAgentID: &orchAgentID,
	})
	require.NoError(t, err)

	callOrder := make([]string, 0, 5)
	var calls atomic.Int32
	exec := &passiveAgentExecutor{callOrder: &callOrder, calls: &calls}

	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, s.txManager,
		exec, exec,
		s.taskService, NewPipelineEngine(5), NewContextBuilder(NoopEncryptor{}, nil, nil), nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
		WithTeamRepository(s.teamRepo), // ← главный объект теста
	)

	require.NoError(t, orch.ProcessTask(ctx, task.ID))
	require.Equal(t, []string{
		string(models.AgentRoleOrchestrator),
		string(models.AgentRolePlanner),
		string(models.AgentRoleDeveloper),
		string(models.AgentRoleReviewer),
		string(models.AgentRoleTester),
	}, callOrder)

	final, err := s.taskRepo.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusCompleted, final.Status)
	require.NotNil(t, final.AssignedAgentID)
	require.Equal(t, s.agents[models.AgentRoleTester].ID, *final.AssignedAgentID,
		"after testing→completed, assigned_agent_id should be tester (последний шаг pipeline)")
}

// TestOrchestratorE2E_ContextBuilder_InjectsArtifactsForNextAgent — проверяем,
// что ContextBuilder.appendPipelineHandoff подмешивает в PromptUser следующего
// агента артефакты предыдущего шага (например, diff developer'а для reviewer'а).
func TestOrchestratorE2E_ContextBuilder_InjectsArtifactsForNextAgent(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx := context.Background()

	// Готовим задачу в статусе review с artifacts от developer и одним
	// сообщением-результатом developer'а в task_messages.
	devAgent := s.agents[models.AgentRoleDeveloper]
	revAgent := s.agents[models.AgentRoleReviewer]
	branch := "feature/ctx-handoff"
	diffArtifacts := datatypes.JSON([]byte(`{"diff":"diff --git a/HELLO.md b/HELLO.md\nnew file","commit_hash":"abc1234","branch_name":"feature/ctx-handoff"}`))

	task := &models.Task{
		ProjectID:       s.project.ID,
		Title:           "Ctx handoff",
		Description:     "Reviewer must see developer's diff in prompt.",
		Status:          models.TaskStatusReview,
		Priority:        models.TaskPriorityMedium,
		AssignedAgentID: &revAgent.ID,
		CreatedByType:   models.CreatedByUser,
		CreatedByID:     s.user.ID,
		Context:         datatypes.JSON([]byte("{}")),
		Artifacts:       diffArtifacts,
		BranchName:      &branch,
	}
	require.NoError(t, s.taskRepo.Create(ctx, task))

	devMsg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeAgent,
		SenderID:    devAgent.ID,
		Content:     "developer: implemented HELLO.md with two lines",
		MessageType: models.MessageTypeResult,
		Metadata:    diffArtifacts,
	}
	require.NoError(t, s.taskMsgRepo.Create(ctx, devMsg))

	cb := NewContextBuilderFull(NoopEncryptor{}, nil, nil, nil, s.taskMsgRepo)
	input, err := cb.Build(ctx, task, revAgent, s.project)
	require.NoError(t, err)

	require.Contains(t, input.PromptUser, "<previous_step_artifacts",
		"reviewer prompt must include developer's artifacts block")
	require.Contains(t, input.PromptUser, "HELLO.md",
		"reviewer prompt must contain developer's diff content")
	require.Contains(t, input.PromptUser, "<previous_steps>",
		"reviewer prompt must include prior task_messages history")
	require.Contains(t, input.PromptUser, "developer: implemented HELLO.md",
		"reviewer prompt must contain developer's message content")
}

// recordingPRPublisher — запоминает вызов Publish для проверки в тесте.
type recordingPRPublisher struct {
	called    atomic.Int32
	lastTask  *models.Task
	lastProj  *models.Project
	donePR    chan struct{}
}

func newRecordingPRPublisher() *recordingPRPublisher {
	return &recordingPRPublisher{donePR: make(chan struct{}, 1)}
}

func (p *recordingPRPublisher) Publish(_ context.Context, task *models.Task, project *models.Project) (*gitprovider.PullRequest, error) {
	p.called.Add(1)
	p.lastTask = task
	p.lastProj = project
	select {
	case p.donePR <- struct{}{}:
	default:
	}
	return &gitprovider.PullRequest{Number: 42, HTMLURL: "https://example.com/pr/42"}, nil
}

// TestOrchestratorE2E_PullRequestPublisher_FiresOnCompleted — после перехода
// задачи в completed оркестратор должен вызвать PullRequestPublisher.Publish
// ровно один раз с этой задачей и её проектом.
func TestOrchestratorE2E_PullRequestPublisher_FiresOnCompleted(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	orchAgentID := s.agents[models.AgentRoleOrchestrator].ID
	task, err := s.taskService.Create(ctx, s.user.ID, models.RoleUser, s.project.ID, dto.CreateTaskRequest{
		Title:           "PR after completed",
		Description:     "Publisher must be invoked once on terminal completed transition.",
		AssignedAgentID: &orchAgentID,
	})
	require.NoError(t, err)

	callOrder := make([]string, 0, 5)
	var calls atomic.Int32
	exec := &scriptedAgentExecutor{db: s.db, agents: s.agents, callOrder: &callOrder, calls: &calls}

	publisher := newRecordingPRPublisher()
	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, s.txManager,
		exec, exec,
		s.taskService, NewPipelineEngine(5), NewContextBuilder(NoopEncryptor{}, nil, nil), nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
		WithPullRequestPublisher(publisher),
	)

	require.NoError(t, orch.ProcessTask(ctx, task.ID))

	// Publisher вызывается в отдельной горутине — ждём её.
	select {
	case <-publisher.donePR:
	case <-time.After(3 * time.Second):
		t.Fatal("PR publisher was not invoked within 3s after completed")
	}
	require.Equal(t, int32(1), publisher.called.Load(),
		"publisher must be called exactly once per completed pipeline")
	require.NotNil(t, publisher.lastTask)
	require.Equal(t, task.ID, publisher.lastTask.ID)
	require.NotNil(t, publisher.lastProj)
	require.Equal(t, s.project.ID, publisher.lastProj.ID)
}

// changesRequestedThenApproveExecutor — reviewer первый раз отвечает
// "changes_requested", после повторного прогона developer'а — "approved".
// НЕ трогает assigned_agent_id (это делает оркестратор через WithTeamRepository).
type changesRequestedThenApproveExecutor struct {
	callOrder   *[]string
	reviewCount atomic.Int32
}

func (e *changesRequestedThenApproveExecutor) Execute(_ context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	*e.callOrder = append(*e.callOrder, in.Role)
	var (
		artifacts json.RawMessage
		output    string
	)
	switch models.AgentRole(in.Role) {
	case models.AgentRoleDeveloper:
		artifacts = json.RawMessage(`{"diff":"+ work"}`)
		output = "developer: changes"
	case models.AgentRoleReviewer:
		if e.reviewCount.Add(1) == 1 {
			artifacts = json.RawMessage(`{"decision":"changes_requested"}`)
			output = "reviewer: please fix X"
		} else {
			artifacts = json.RawMessage(`{"decision":"approved"}`)
			output = "reviewer: ok"
		}
	case models.AgentRoleTester:
		artifacts = json.RawMessage(`{"decision":"passed"}`)
		output = "tester: green"
	default:
		return nil, errors.New("unexpected role " + in.Role)
	}
	return &agent.ExecutionResult{Success: true, Output: output, ArtifactsJSON: artifacts}, nil
}

// TestOrchestratorE2E_ChangesRequested_LoopThenApprove — проверяем полную петлю
// review→changes_requested→in_progress→review→testing→completed и инкремент
// iteration_count в task.Context.
func TestOrchestratorE2E_ChangesRequested_LoopThenApprove(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	devAgent := s.agents[models.AgentRoleDeveloper]
	branch := "feature/loop"
	task := &models.Task{
		ProjectID:       s.project.ID,
		Title:           "Loop test",
		Description:     "Reviewer first says changes_requested, then approves on retry.",
		Status:          models.TaskStatusInProgress,
		Priority:        models.TaskPriorityMedium,
		AssignedAgentID: &devAgent.ID,
		CreatedByType:   models.CreatedByUser,
		CreatedByID:     s.user.ID,
		Context:         datatypes.JSON([]byte("{}")),
		Artifacts:       datatypes.JSON([]byte("{}")),
		BranchName:      &branch,
	}
	require.NoError(t, s.taskRepo.Create(context.Background(), task))

	callOrder := make([]string, 0, 6)
	exec := &changesRequestedThenApproveExecutor{callOrder: &callOrder}

	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, s.txManager,
		exec, exec,
		s.taskService, NewPipelineEngine(5), NewContextBuilder(NoopEncryptor{}, nil, nil), nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
		WithTeamRepository(s.teamRepo), // оркестратор сам выбирает агента по роли nextStatus
	)

	require.NoError(t, orch.ProcessTask(ctx, task.ID))
	// Pipeline зовёт executor на каждом не-терминальном статусе. Для одной петли
	// changes_requested это: in_progress→review, review→changes_requested,
	// changes_requested→in_progress (developer-ack), in_progress→review,
	// review→testing, testing→completed.
	require.Equal(t, []string{
		string(models.AgentRoleDeveloper), // in_progress → review
		string(models.AgentRoleReviewer),  // review → changes_requested
		string(models.AgentRoleDeveloper), // changes_requested → in_progress
		string(models.AgentRoleDeveloper), // in_progress → review (retry)
		string(models.AgentRoleReviewer),  // review → testing (approve)
		string(models.AgentRoleTester),    // testing → completed
	}, callOrder)

	final, err := s.taskRepo.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, models.TaskStatusCompleted, final.Status)

	// iteration_count должен быть инкрементирован хотя бы один раз
	// (попадание в changes_requested обновляет счётчик).
	require.Regexp(t, `"iteration_count"\s*:\s*1`, string(final.Context),
		"iteration_count must be incremented after a changes_requested loop")
}

// alwaysChangesRequestedExecutor — reviewer бесконечно требует правок;
// агента переключает оркестратор через WithTeamRepository.
type alwaysChangesRequestedExecutor struct {
	callOrder *[]string
}

func (e *alwaysChangesRequestedExecutor) Execute(_ context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	*e.callOrder = append(*e.callOrder, in.Role)
	var artifacts json.RawMessage
	switch models.AgentRole(in.Role) {
	case models.AgentRoleDeveloper:
		artifacts = json.RawMessage(`{"diff":"+ work"}`)
	case models.AgentRoleReviewer:
		artifacts = json.RawMessage(`{"decision":"changes_requested"}`)
	default:
		return nil, errors.New("unexpected role " + in.Role)
	}
	return &agent.ExecutionResult{Success: true, Output: in.Role + ": loop", ArtifactsJSON: artifacts}, nil
}

// TestOrchestratorE2E_IterationLimit_FailsAfterMax — если reviewer всегда
// отвечает changes_requested, после maxIterations задача должна перейти в failed
// с ErrOrchestratorIterationLimitReached, а не крутиться вечно.
func TestOrchestratorE2E_IterationLimit_FailsAfterMax(t *testing.T) {
	s := setupOrchestratorE2E(t)
	defer cleanupOrchestratorE2E(t, s.db, s.user.ID, s.project.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	devAgent := s.agents[models.AgentRoleDeveloper]
	branch := "feature/limit"
	task := &models.Task{
		ProjectID:       s.project.ID,
		Title:           "Limit test",
		Description:     "Reviewer keeps requesting changes; pipeline must give up after max.",
		Status:          models.TaskStatusInProgress,
		Priority:        models.TaskPriorityMedium,
		AssignedAgentID: &devAgent.ID,
		CreatedByType:   models.CreatedByUser,
		CreatedByID:     s.user.ID,
		Context:         datatypes.JSON([]byte("{}")),
		Artifacts:       datatypes.JSON([]byte("{}")),
		BranchName:      &branch,
	}
	require.NoError(t, s.taskRepo.Create(context.Background(), task))

	callOrder := make([]string, 0, 8)
	exec := &alwaysChangesRequestedExecutor{callOrder: &callOrder}

	const maxIter = 2
	orch := NewOrchestratorService(
		s.taskRepo, s.taskMsgRepo, s.workflowRepo, s.projectSvc, s.txManager,
		exec, exec,
		s.taskService, NewPipelineEngine(maxIter), NewContextBuilder(NoopEncryptor{}, nil, nil), nil,
		noopSandboxStopper{}, nil,
		WithStepPollInterval(0),
		WithTeamRepository(s.teamRepo),
	)

	err := orch.ProcessTask(ctx, task.ID)
	require.ErrorIs(t, err, ErrOrchestratorIterationLimitReached)

	final, e2 := s.taskRepo.GetByID(context.Background(), task.ID)
	require.NoError(t, e2)
	require.Equal(t, models.TaskStatusFailed, final.Status,
		"task must terminate as failed once iteration limit is reached")
}
