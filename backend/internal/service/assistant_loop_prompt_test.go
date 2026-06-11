package service

import (
	"testing"

	"github.com/devteam/backend/internal/models"
)

// TestResolveAssistantBasePrompt — приоритет per-project промпта над user-промптом
// (наследование копией role → user → project; пустой/NULL project → fallback).
func TestResolveAssistantBasePrompt(t *testing.T) {
	sp := func(s string) *string { return &s }

	cases := []struct {
		name        string
		agentPrompt *string
		project     *models.Project
		want        string
	}{
		{"нет проекта → user-промпт", sp("user prompt"), nil, "user prompt"},
		{"проект без снапшота (legacy NULL) → user-промпт", sp("user prompt"), &models.Project{}, "user prompt"},
		{"проектный снапшот замещает user-промпт", sp("user prompt"), &models.Project{AssistantPrompt: sp("project prompt")}, "project prompt"},
		{"сброшенный (пустой) проектный → user-промпт", sp("user prompt"), &models.Project{AssistantPrompt: sp("  ")}, "user prompt"},
		{"нет ничего → пусто", nil, &models.Project{}, ""},
		{"только проектный", nil, &models.Project{AssistantPrompt: sp("project only")}, "project only"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveAssistantBasePrompt(c.agentPrompt, c.project); got != c.want {
				t.Errorf("resolveAssistantBasePrompt() = %q, want %q", got, c.want)
			}
		})
	}
}
