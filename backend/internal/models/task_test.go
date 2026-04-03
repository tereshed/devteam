package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskStatus_IsValid(t *testing.T) {
	valid := []TaskStatus{
		TaskStatusPending, TaskStatusPlanning, TaskStatusInProgress,
		TaskStatusReview, TaskStatusChangesRequested, TaskStatusTesting,
		TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, TaskStatusPaused,
	}
	for _, s := range valid {
		assert.True(t, s.IsValid(), "expected valid: %s", s)
	}
	assert.False(t, TaskStatus("unknown").IsValid())
}

func TestTaskPriority_IsValid(t *testing.T) {
	for _, p := range []TaskPriority{
		TaskPriorityCritical, TaskPriorityHigh, TaskPriorityMedium, TaskPriorityLow,
	} {
		assert.True(t, p.IsValid(), "expected valid: %s", p)
	}
	assert.False(t, TaskPriority("urgent").IsValid())
}

func TestCreatedByType_IsValid(t *testing.T) {
	assert.True(t, CreatedByUser.IsValid())
	assert.True(t, CreatedByAgent.IsValid())
	assert.False(t, CreatedByType("system").IsValid())
}
