//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func taskRepoTestUserProject(t *testing.T, db *gorm.DB) (*models.User, *models.Project) {
	t.Helper()
	return teamTestProject(t, db)
}

func taskRepoTestAgent(t *testing.T, db *gorm.DB, projectID uuid.UUID) *models.Agent {
	t.Helper()
	ctx := context.Background()
	team := teamRepoCreate(t, db, projectID, "tr-"+uuid.NewString()[:8])
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	a := &models.Agent{
		Name:     "ag-" + uuid.NewString()[:8],
		Role:     models.AgentRoleDeveloper,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(a).Error)
	return a
}

func newTestTask(projectID, creatorID uuid.UUID, title string) *models.Task {
	return &models.Task{
		ProjectID:     projectID,
		Title:         title,
		Description:   "desc",
		CreatedByType: models.CreatedByUser,
		CreatedByID:   creatorID,
		Context:       datatypes.JSON([]byte("{}")),
		Artifacts:     datatypes.JSON([]byte("{}")),
	}
}

func TestTaskRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := taskRepoTestUserProject(t, db)
	agent := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	branch := "feature/x"
	res := "done"
	ctxJSON := datatypes.JSON([]byte(`{"k":1}`))
	art := datatypes.JSON([]byte(`{"a":true}`))
	started := time.Now().UTC().Truncate(time.Second)
	completed := started.Add(time.Hour)
	errMsg := "e"

	task := &models.Task{
		ProjectID:       p.ID,
		Title:           "Full task",
		Description:     "long desc",
		Status:          models.TaskStatusInProgress,
		Priority:        models.TaskPriorityHigh,
		AssignedAgentID: &agent.ID,
		CreatedByType:   models.CreatedByAgent,
		CreatedByID:     agent.ID,
		Context:         ctxJSON,
		Result:          &res,
		Artifacts:       art,
		BranchName:      &branch,
		ErrorMessage:    &errMsg,
		StartedAt:       &started,
		CompletedAt:     &completed,
	}
	require.NoError(t, repo.Create(ctx, task))
	assert.NotEqual(t, uuid.Nil, task.ID)

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "Full task", got.Title)
	assert.Equal(t, "long desc", got.Description)
	assert.Equal(t, models.TaskStatusInProgress, got.Status)
	assert.Equal(t, models.TaskPriorityHigh, got.Priority)
	assert.Equal(t, agent.ID, *got.AssignedAgentID)
	assert.Equal(t, models.CreatedByAgent, got.CreatedByType)
	assert.Equal(t, agent.ID, got.CreatedByID)
	assert.Equal(t, branch, *got.BranchName)
	assert.Equal(t, res, *got.Result)
	assert.Equal(t, errMsg, *got.ErrorMessage)
	require.NotNil(t, got.StartedAt)
	require.NotNil(t, got.CompletedAt)
}

func TestTaskRepository_Create_WithParentTask(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	parent := newTestTask(p.ID, user.ID, "parent")
	require.NoError(t, repo.Create(ctx, parent))

	child := newTestTask(p.ID, user.ID, "child")
	child.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, child))

	got, err := repo.GetByID(ctx, child.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ParentTask)
	assert.Equal(t, parent.ID, got.ParentTask.ID)
	assert.Equal(t, "parent", got.ParentTask.Title)
}

func TestTaskRepository_Create_InvalidProjectID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, _ := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	task := newTestTask(uuid.New(), user.ID, "orphan")
	err := repo.Create(ctx, task)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestTaskRepository_Create_InvalidParentTaskID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	pid := uuid.New()
	task := newTestTask(p.ID, user.ID, "bad parent")
	task.ParentTaskID = &pid
	err := repo.Create(ctx, task)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskRepository_GetByID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	agent := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	parent := newTestTask(p.ID, user.ID, "root")
	parent.AssignedAgentID = &agent.ID
	require.NoError(t, repo.Create(ctx, parent))

	c1 := newTestTask(p.ID, user.ID, "sub-a")
	c1.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, c1))
	time.Sleep(2 * time.Millisecond)
	c2 := newTestTask(p.ID, user.ID, "sub-b")
	c2.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, c2))

	got, err := repo.GetByID(ctx, parent.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AssignedAgent)
	assert.Equal(t, agent.Name, got.AssignedAgent.Name)
	require.Len(t, got.SubTasks, 2)
	assert.Equal(t, "sub-a", got.SubTasks[0].Title)
	assert.Equal(t, "sub-b", got.SubTasks[1].Title)

	childView, err := repo.GetByID(ctx, c1.ID)
	require.NoError(t, err)
	require.NotNil(t, childView.ParentTask)
	assert.Equal(t, parent.ID, childView.ParentTask.ID)
	assert.Equal(t, "root", childView.ParentTask.Title)
}

func TestTaskRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, err := NewTaskRepository(db).GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskRepository_List_ByProjectID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, pa := taskRepoTestUserProject(t, db)
	_, pb := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, newTestTask(pa.ID, user.ID, "a1")))
	require.NoError(t, repo.Create(ctx, newTestTask(pb.ID, user.ID, "b1")))

	pid := pa.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "a1", list[0].Title)
}

func TestTaskRepository_List_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	t1 := newTestTask(p.ID, user.ID, "ip")
	t1.Status = models.TaskStatusInProgress
	t2 := newTestTask(p.ID, user.ID, "pend")
	t2.Status = models.TaskStatusPending
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	st := models.TaskStatusInProgress
	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Status: &st, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, models.TaskStatusInProgress, list[0].Status)
}

func TestTaskRepository_List_FilterByStatuses(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	for _, st := range []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusPlanning, models.TaskStatusPending} {
		tk := newTestTask(p.ID, user.ID, string(st))
		tk.Status = st
		require.NoError(t, repo.Create(ctx, tk))
	}

	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{
		ProjectID: &pid,
		Statuses:  []models.TaskStatus{models.TaskStatusPending, models.TaskStatusPlanning},
		Limit:     20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

func TestTaskRepository_List_FilterByPriority(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	t1 := newTestTask(p.ID, user.ID, "crit")
	t1.Priority = models.TaskPriorityCritical
	t2 := newTestTask(p.ID, user.ID, "low")
	t2.Priority = models.TaskPriorityLow
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	pr := models.TaskPriorityCritical
	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Priority: &pr, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "crit", list[0].Title)
}

func TestTaskRepository_List_FilterByAssignedAgent(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	a1 := taskRepoTestAgent(t, db, p.ID)
	a2 := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	t1 := newTestTask(p.ID, user.ID, "to-a1")
	t1.AssignedAgentID = &a1.ID
	t2 := newTestTask(p.ID, user.ID, "to-a2")
	t2.AssignedAgentID = &a2.ID
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	pid := p.ID
	aid := a1.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, AssignedAgentID: &aid, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "to-a1", list[0].Title)
	require.NotNil(t, list[0].AssignedAgent)
	assert.Equal(t, a1.ID, list[0].AssignedAgent.ID)
}

func TestTaskRepository_List_FilterByCreatedBy(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	agent := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	tu := newTestTask(p.ID, user.ID, "by-user")
	require.NoError(t, repo.Create(ctx, tu))

	ta := newTestTask(p.ID, user.ID, "by-agent")
	ta.CreatedByType = models.CreatedByAgent
	ta.CreatedByID = agent.ID
	require.NoError(t, repo.Create(ctx, ta))

	pid := p.ID
	cbt := models.CreatedByAgent
	cbid := agent.ID
	list, total, err := repo.List(ctx, TaskFilter{
		ProjectID:     &pid,
		CreatedByType: &cbt,
		CreatedByID:   &cbid,
		Limit:         20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "by-agent", list[0].Title)
}

func TestTaskRepository_List_RootOnly(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	root := newTestTask(p.ID, user.ID, "root")
	require.NoError(t, repo.Create(ctx, root))
	sub := newTestTask(p.ID, user.ID, "sub")
	sub.ParentTaskID = &root.ID
	require.NoError(t, repo.Create(ctx, sub))

	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, RootOnly: true, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "root", list[0].Title)
}

func TestTaskRepository_List_FilterByBranch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	b := "feature/auth"
	t1 := newTestTask(p.ID, user.ID, "on-branch")
	t1.BranchName = &b
	t2 := newTestTask(p.ID, user.ID, "other")
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, BranchName: &b, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "on-branch", list[0].Title)
}

func TestTaskRepository_List_Search(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	t1 := newTestTask(p.ID, user.ID, "Alpha unique token")
	t2 := newTestTask(p.ID, user.ID, "Beta other")
	t2.Description = "has unique token in body"
	require.NoError(t, repo.Create(ctx, t1))
	require.NoError(t, repo.Create(ctx, t2))

	pid := p.ID
	q := "unique token"
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Search: &q, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

func TestTaskRepository_List_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		require.NoError(t, repo.Create(ctx, newTestTask(p.ID, user.ID, string(rune('a'+i)))))
	}

	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Limit: 3, Offset: 2, OrderBy: "title", OrderDir: "ASC"})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	require.Len(t, list, 3)
	assert.Equal(t, "c", list[0].Title)
	assert.Equal(t, "d", list[1].Title)
	assert.Equal(t, "e", list[2].Title)
}

func TestTaskRepository_List_OrderBy_Whitelist(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	t1 := newTestTask(p.ID, user.ID, "first")
	require.NoError(t, repo.Create(ctx, t1))
	time.Sleep(5 * time.Millisecond)
	t2 := newTestTask(p.ID, user.ID, "second")
	require.NoError(t, repo.Create(ctx, t2))

	pid := p.ID
	list, _, err := repo.List(ctx, TaskFilter{ProjectID: &pid, OrderBy: "invalid_column;;", OrderDir: "DESC", Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 2)
	// fallback created_at DESC → newer first
	assert.Equal(t, "second", list[0].Title)
	assert.Equal(t, "first", list[1].Title)
}

func TestTaskRepository_List_OrderBy_Priority(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	order := []struct {
		title string
		pr    models.TaskPriority
	}{
		{"med", models.TaskPriorityMedium},
		{"crit", models.TaskPriorityCritical},
		{"low", models.TaskPriorityLow},
	}
	for _, o := range order {
		tk := newTestTask(p.ID, user.ID, o.title)
		tk.Priority = o.pr
		require.NoError(t, repo.Create(ctx, tk))
	}

	pid := p.ID
	list, _, err := repo.List(ctx, TaskFilter{ProjectID: &pid, OrderBy: "priority", OrderDir: "ASC", Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "crit", list[0].Title)
	assert.Equal(t, "low", list[1].Title)
	assert.Equal(t, "med", list[2].Title)
}

func TestTaskRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	agent := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	task := newTestTask(p.ID, user.ID, "up")
	require.NoError(t, repo.Create(ctx, task))

	before := task.UpdatedAt
	time.Sleep(5 * time.Millisecond)

	loaded, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	expStatus := loaded.Status
	expUpdated := loaded.UpdatedAt
	loaded.Status = models.TaskStatusReview
	loaded.AssignedAgentID = &agent.ID
	require.NoError(t, repo.Update(ctx, loaded, expStatus, expUpdated))

	again, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusReview, again.Status)
	assert.Equal(t, agent.ID, *again.AssignedAgentID)
	assert.True(t, again.UpdatedAt.After(before) || !again.UpdatedAt.Equal(before))
}

func TestTaskRepository_Update_StaleOptimisticLock(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	agent := taskRepoTestAgent(t, db, p.ID)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	task := newTestTask(p.ID, user.ID, "stale-lock")
	require.NoError(t, repo.Create(ctx, task))

	a, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	staleStatus := a.Status
	staleUpdated := a.UpdatedAt

	b, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	expBStatus := b.Status
	expBUpdated := b.UpdatedAt
	b.Status = models.TaskStatusPlanning
	require.NoError(t, repo.Update(ctx, b, expBStatus, expBUpdated))

	c, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusPlanning, c.Status)

	c.Status = models.TaskStatusReview
	c.AssignedAgentID = &agent.ID
	err = repo.Update(ctx, c, staleStatus, staleUpdated)
	assert.ErrorIs(t, err, ErrTaskConcurrentUpdate)
}

func TestTaskRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	task := newTestTask(p.ID, user.ID, "del")
	require.NoError(t, repo.Create(ctx, task))
	require.NoError(t, repo.Delete(ctx, task.ID))

	_, err := repo.GetByID(ctx, task.ID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskRepository_Delete_CascadeMessages(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	msgRepo := NewTaskMessageRepository(db)
	ctx := context.Background()

	task := newTestTask(p.ID, user.ID, "with-msg")
	require.NoError(t, repo.Create(ctx, task))

	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeUser,
		SenderID:    user.ID,
		Content:     "hi",
		MessageType: models.MessageTypeInstruction,
		Metadata:    datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, msgRepo.Create(ctx, msg))

	require.NoError(t, repo.Delete(ctx, task.ID))

	var cnt int64
	require.NoError(t, db.WithContext(ctx).Model(&models.TaskMessage{}).Where("task_id = ?", task.ID).Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt)
}

func TestTaskRepository_Delete_SubTasksSetNull(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	parent := newTestTask(p.ID, user.ID, "par")
	require.NoError(t, repo.Create(ctx, parent))
	child := newTestTask(p.ID, user.ID, "ch")
	child.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, child))

	require.NoError(t, repo.Delete(ctx, parent.ID))

	var row models.Task
	require.NoError(t, db.WithContext(ctx).Where("id = ?", child.ID).First(&row).Error)
	assert.Nil(t, row.ParentTaskID)
}

func TestTaskRepository_CountByProjectID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	const n = 4
	for i := 0; i < n; i++ {
		require.NoError(t, repo.Create(ctx, newTestTask(p.ID, user.ID, uuid.NewString()[:8])))
	}

	cnt, err := repo.CountByProjectID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(n), cnt)
}

func TestTaskRepository_ListByParentID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	parent := newTestTask(p.ID, user.ID, "p")
	require.NoError(t, repo.Create(ctx, parent))

	c1 := newTestTask(p.ID, user.ID, "s1")
	c1.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, c1))
	time.Sleep(2 * time.Millisecond)
	c2 := newTestTask(p.ID, user.ID, "s2")
	c2.ParentTaskID = &parent.ID
	require.NoError(t, repo.Create(ctx, c2))

	subs, err := repo.ListByParentID(ctx, parent.ID)
	require.NoError(t, err)
	require.Len(t, subs, 2)
	assert.Equal(t, "s1", subs[0].Title)
	assert.Equal(t, "s2", subs[1].Title)
}

func TestTaskRepository_List_Search_EscapesWildcards(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	tWild := newTestTask(p.ID, user.ID, `100%_match`)
	require.NoError(t, repo.Create(ctx, tWild))
	tPlain := newTestTask(p.ID, user.ID, "plain")
	require.NoError(t, repo.Create(ctx, tPlain))

	pid := p.ID
	pct := "%"
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Search: &pct, Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, `100%_match`, list[0].Title)
}

func TestTaskRepository_List_LimitDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user, p := taskRepoTestUserProject(t, db)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	for i := 0; i < 60; i++ {
		require.NoError(t, repo.Create(ctx, newTestTask(p.ID, user.ID, uuid.NewString())))
	}

	pid := p.ID
	list, total, err := repo.List(ctx, TaskFilter{ProjectID: &pid, Limit: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(60), total)
	assert.Len(t, list, 50)
}
