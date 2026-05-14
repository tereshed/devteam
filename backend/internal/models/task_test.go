package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskStatus_IsValid(t *testing.T) {
	valid := []TaskState{
		TaskStateActive, TaskStateActive, TaskStateActive,
		TaskStateActive, TaskStateActive, TaskStateActive,
		TaskStateDone, TaskStateFailed, TaskStateCancelled, TaskStateNeedsHuman,
	}
	for _, s := range valid {
		assert.True(t, s.IsValid(), "expected valid: %s", s)
	}
	assert.False(t, TaskState("unknown").IsValid())
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

func TestTask_GetSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		desc     string
		expected string
	}{
		{
			name:     "empty",
			title:    "",
			desc:     "",
			expected: "",
		},
		{
			name:     "title only",
			title:    "Fix bug",
			desc:     "",
			expected: "Fix bug",
		},
		{
			name:     "description only",
			title:    "",
			desc:     "Implement new feature",
			expected: "Implement new feature",
		},
		{
			name:     "both",
			title:    "Task",
			desc:     "Details",
			expected: "Task Details",
		},
		{
			name:     "truncation",
			title:    "Short",
			desc:     strings.Repeat("a", 2500),
			expected: "Short " + strings.Repeat("a", 2000),
		},
		{
			name:     "utf8 truncation",
			title:    "UTF8",
			desc:     strings.Repeat("🚀", 1000), // Каждый эмодзи 4 байта, но 1 руна
			expected: "UTF8 " + strings.Repeat("🚀", 1000), // 1000 рун < 2000 лимита
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Title:       tt.title,
				Description: tt.desc,
			}
			assert.Equal(t, tt.expected, task.GetSearchQuery())
		})
	}
}

func TestTask_GetSearchQuery_UTF8Truncation(t *testing.T) {
	// Проверка жесткого лимита 2000 рун
	longDesc := ""
	for i := 0; i < 3000; i++ {
		longDesc += "あ" // 3-байтовый символ
	}
	
	task := &Task{
		Title:       "",
		Description: longDesc,
	}
	
	query := task.GetSearchQuery()
	// Проверяем количество рун
	count := 0
	for range query {
		count++
	}
	assert.Equal(t, 2000, count)
}
