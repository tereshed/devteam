package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestToTaskResponse_AllFields(t *testing.T) {
	t.Parallel()

	projID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	taskID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	parentID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	agentID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	creatorID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	started := ts.Add(time.Hour)
	completed := ts.Add(2 * time.Hour)
	result := "done"
	branch := "feature/x"
	errMsg := "oops"

	task := &models.Task{
		ID:            taskID,
		ProjectID:     projID,
		ParentTaskID:  &parentID,
		Title:         "Root task",
		Description:   "Full description",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityHigh,
		AssignedAgent: &models.Agent{ID: agentID, Name: "DevBot", Role: models.AgentRoleDeveloper},
		CreatedByType: models.CreatedByUser,
		CreatedByID:   creatorID,
		Context:       datatypes.JSON(`{"k":1}`),
		Result:        &result,
		Artifacts:     datatypes.JSON(`{"diff":"x"}`),
		BranchName:    &branch,
		ErrorMessage:  &errMsg,
		StartedAt:     &started,
		CompletedAt:   &completed,
		CreatedAt:     ts,
		UpdatedAt:     ts,
	}

	got := ToTaskResponse(task)

	assert.Equal(t, taskID.String(), got.ID)
	assert.Equal(t, projID.String(), got.ProjectID)
	require.NotNil(t, got.ParentTaskID)
	assert.Equal(t, parentID.String(), *got.ParentTaskID)
	assert.Equal(t, "Root task", got.Title)
	assert.Equal(t, "Full description", got.Description)
	assert.Equal(t, string(models.TaskStateActive), got.Status)
	assert.Equal(t, string(models.TaskPriorityHigh), got.Priority)
	require.NotNil(t, got.AssignedAgent)
	assert.Equal(t, agentID.String(), got.AssignedAgent.ID)
	assert.Equal(t, "DevBot", got.AssignedAgent.Name)
	assert.Equal(t, string(models.AgentRoleDeveloper), got.AssignedAgent.Role)
	assert.Equal(t, string(models.CreatedByUser), got.CreatedByType)
	assert.Equal(t, creatorID.String(), got.CreatedByID)
	assert.JSONEq(t, `{"k":1}`, string(got.Context))
	require.NotNil(t, got.Result)
	assert.Equal(t, "done", *got.Result)
	assert.JSONEq(t, `{"diff":"x"}`, string(got.Artifacts))
	require.NotNil(t, got.BranchName)
	assert.Equal(t, "feature/x", *got.BranchName)
	require.NotNil(t, got.ErrorMessage)
	assert.Equal(t, "oops", *got.ErrorMessage)
	assert.True(t, got.StartedAt.Equal(started))
	assert.True(t, got.CompletedAt.Equal(completed))
	assert.True(t, got.CreatedAt.Equal(ts))
	assert.True(t, got.UpdatedAt.Equal(ts))
	assert.Nil(t, got.MessageCount)
}

func TestToTaskResponse_NilAgent(t *testing.T) {
	t.Parallel()

	task := &models.Task{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Title:         "T",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   uuid.New(),
		Context:       datatypes.JSON(`{}`),
		Artifacts:     datatypes.JSON(`{}`),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	got := ToTaskResponse(task)
	assert.Nil(t, got.AssignedAgent)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(raw), `"assigned_agent"`))
}

func TestToTaskResponse_WithAgent(t *testing.T) {
	t.Parallel()

	aid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	task := &models.Task{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Title:         "With agent",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityLow,
		AssignedAgent: &models.Agent{ID: aid, Name: "Planner", Role: models.AgentRolePlanner},
		CreatedByType: models.CreatedByAgent,
		CreatedByID:   uuid.New(),
		Context:       datatypes.JSON(`{}`),
		Artifacts:     datatypes.JSON(`{}`),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	got := ToTaskResponse(task)
	require.NotNil(t, got.AssignedAgent)
	assert.Equal(t, aid.String(), got.AssignedAgent.ID)
	assert.Equal(t, "Planner", got.AssignedAgent.Name)
	assert.Equal(t, string(models.AgentRolePlanner), got.AssignedAgent.Role)
}

func TestToTaskResponse_WithSubTasks(t *testing.T) {
	t.Parallel()

	st1 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	st2 := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	st3 := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	task := &models.Task{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Title:         "Parent",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   uuid.New(),
		Context:       datatypes.JSON(`{}`),
		Artifacts:     datatypes.JSON(`{}`),
		SubTasks: []models.Task{
			{ID: st1, Title: "A", State: models.TaskStateActive, Priority: models.TaskPriorityHigh},
			{ID: st2, Title: "B", State: models.TaskStateActive, Priority: models.TaskPriorityMedium},
			{ID: st3, Title: "C", State: models.TaskStateDone, Priority: models.TaskPriorityLow},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	got := ToTaskResponse(task)
	require.Len(t, got.SubTasks, 3)
	assert.Equal(t, st1.String(), got.SubTasks[0].ID)
	assert.Equal(t, "A", got.SubTasks[0].Title)
	assert.Equal(t, string(models.TaskStateActive), got.SubTasks[0].Status)
	assert.Equal(t, string(models.TaskPriorityHigh), got.SubTasks[0].Priority)
	assert.Equal(t, st3.String(), got.SubTasks[2].ID)
}

func TestToTaskResponse_NilOptionalFields(t *testing.T) {
	t.Parallel()

	task := &models.Task{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Title:         "Minimal",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   uuid.New(),
		Context:       datatypes.JSON(`{}`),
		Artifacts:     datatypes.JSON(`{}`),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	got := ToTaskResponse(task)
	assert.Nil(t, got.ParentTaskID)
	assert.Nil(t, got.Result)
	assert.Nil(t, got.BranchName)
	assert.Nil(t, got.ErrorMessage)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	s := string(raw)
	for _, key := range []string{"parent_task_id", "result", "branch_name", "error_message"} {
		assert.False(t, strings.Contains(s, `"`+key+`"`), "unexpected key %q in %s", key, s)
	}
}

func TestToTaskListItem(t *testing.T) {
	t.Parallel()

	branch := "b"
	res := "secret result"
	task := &models.Task{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Title:         "List item",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityCritical,
		CreatedByType: models.CreatedByAgent,
		CreatedByID:   uuid.New(),
		Context:       datatypes.JSON(`{"heavy":true}`),
		Result:        &res,
		Artifacts:     datatypes.JSON(`{"big":"data"}`),
		BranchName:    &branch,
		SubTasks:      []models.Task{{ID: uuid.New(), Title: "child", State: models.TaskStateActive, Priority: models.TaskPriorityLow}},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	got := ToTaskListItem(task)
	assert.Equal(t, task.ID.String(), got.ID)
	assert.Equal(t, task.ProjectID.String(), got.ProjectID)
	assert.Equal(t, "List item", got.Title)
	assert.Equal(t, string(models.TaskStateActive), got.Status)
	assert.Equal(t, string(models.TaskPriorityCritical), got.Priority)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	s := string(raw)
	for _, key := range []string{"context", "artifacts", "result", "sub_tasks", "error_message", "description"} {
		assert.False(t, strings.Contains(s, `"`+key+`"`), "list item must not expose %q", key)
	}
}

func TestToTaskListResponse(t *testing.T) {
	t.Parallel()

	t1 := models.Task{
		ID: uuid.MustParse("55555555-5555-5555-5555-555555555555"), ProjectID: uuid.New(),
		Title: "One", State: models.TaskStateActive, Priority: models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser, CreatedByID: uuid.New(),
		Context: datatypes.JSON(`{}`), Artifacts: datatypes.JSON(`{}`),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	t2 := models.Task{
		ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"), ProjectID: uuid.New(),
		Title: "Two", State: models.TaskStateDone, Priority: models.TaskPriorityLow,
		CreatedByType: models.CreatedByUser, CreatedByID: uuid.New(),
		Context: datatypes.JSON(`{}`), Artifacts: datatypes.JSON(`{}`),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	got := ToTaskListResponse([]models.Task{t1, t2}, 100, 20, 40)
	require.Len(t, got.Tasks, 2)
	assert.Equal(t, t1.ID.String(), got.Tasks[0].ID)
	assert.Equal(t, t2.ID.String(), got.Tasks[1].ID)
	assert.Equal(t, int64(100), got.Total)
	assert.Equal(t, 20, got.Limit)
	assert.Equal(t, 40, got.Offset)
}

func TestToTaskListResponse_Empty(t *testing.T) {
	t.Parallel()
	got := ToTaskListResponse(nil, 0, 10, 0)
	assert.NotNil(t, got.Tasks)
	assert.Len(t, got.Tasks, 0)
	assert.Equal(t, int64(0), got.Total)
}

func TestToAgentSummary_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ToAgentSummary(nil))
}

func TestToAgentSummary(t *testing.T) {
	t.Parallel()
	aid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	a := &models.Agent{ID: aid, Name: "Tester", Role: models.AgentRoleTester}
	got := ToAgentSummary(a)
	require.NotNil(t, got)
	assert.Equal(t, aid.String(), got.ID)
	assert.Equal(t, "Tester", got.Name)
	assert.Equal(t, string(models.AgentRoleTester), got.Role)
}

func TestToTaskMessageResponse(t *testing.T) {
	t.Parallel()
	mid := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	tid := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	sid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab")
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	m := &models.TaskMessage{
		ID: mid, TaskID: tid,
		SenderType: models.SenderTypeAgent, SenderID: sid,
		Content: "hello", MessageType: models.MessageTypeResult,
		Metadata: datatypes.JSON(`{"tokens":42}`),
		CreatedAt: ts,
	}
	got := ToTaskMessageResponse(m)
	assert.Equal(t, mid.String(), got.ID)
	assert.Equal(t, tid.String(), got.TaskID)
	assert.Equal(t, string(models.SenderTypeAgent), got.SenderType)
	assert.Equal(t, sid.String(), got.SenderID)
	assert.Equal(t, "hello", got.Content)
	assert.Equal(t, string(models.MessageTypeResult), got.MessageType)
	assert.JSONEq(t, `{"tokens":42}`, string(got.Metadata))
	assert.True(t, got.CreatedAt.Equal(ts))
}

func TestToTaskMessageListResponse(t *testing.T) {
	t.Parallel()
	base := time.Now().UTC()
	msgs := []models.TaskMessage{
		{
			ID: uuid.New(), TaskID: uuid.New(),
			SenderType: models.SenderTypeUser, SenderID: uuid.New(),
			Content: "a", MessageType: models.MessageTypeInstruction,
			Metadata: datatypes.JSON(`{}`), CreatedAt: base,
		},
		{
			ID: uuid.New(), TaskID: uuid.New(),
			SenderType: models.SenderTypeAgent, SenderID: uuid.New(),
			Content: "b", MessageType: models.MessageTypeFeedback,
			Metadata: datatypes.JSON(`{}`), CreatedAt: base,
		},
	}
	got := ToTaskMessageListResponse(msgs, 50, 25, 5)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "a", got.Messages[0].Content)
	assert.Equal(t, "b", got.Messages[1].Content)
	assert.Equal(t, int64(50), got.Total)
	assert.Equal(t, 25, got.Limit)
	assert.Equal(t, 5, got.Offset)
}
