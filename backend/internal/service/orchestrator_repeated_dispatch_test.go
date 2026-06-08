package service

import (
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/lib/pq"
)

// orchestrator_repeated_dispatch_test.go — контракт на детект зацикливания Router'а:
// repeatedDispatchRun (backstop в Orchestrator.Step) и рендер `# Recent routing history`
// в user-prompt'е. Контекст: инцидент SupportAggent — Router на каждом шаге считал задачу
// «только созданной» и заново вызывал того же агента, потому что не видел собственных прошлых
// решений (см. orchestrator_v2.go §6.7 и router_service.go buildUserPrompt).

func decision(stepNo int, agents ...string) models.RouterDecision {
	return models.RouterDecision{
		StepNo:       stepNo,
		ChosenAgents: pq.StringArray(agents),
	}
}

func TestRepeatedDispatchRun(t *testing.T) {
	tests := []struct {
		name      string
		decisions []models.RouterDecision // хронологический порядок (ASC)
		wantRun   int
		wantLabel string
	}{
		{
			name:      "empty history",
			decisions: nil,
			wantRun:   0,
		},
		{
			name:      "single dispatch",
			decisions: []models.RouterDecision{decision(0, "SupportAggent")},
			wantRun:   1,
			wantLabel: "SupportAggent",
		},
		{
			name: "support loop — same agent four times",
			decisions: []models.RouterDecision{
				decision(0, "SupportAggent"),
				decision(1, "SupportAggent"),
				decision(2, "SupportAggent"),
				decision(3, "SupportAggent"),
			},
			wantRun:   4,
			wantLabel: "SupportAggent",
		},
		{
			name: "alternating developer/reviewer — no run",
			decisions: []models.RouterDecision{
				decision(0, "developer"),
				decision(1, "reviewer"),
				decision(2, "developer"),
				decision(3, "reviewer"),
			},
			wantRun:   1,
			wantLabel: "reviewer",
		},
		{
			name: "run counts only the trailing tail",
			decisions: []models.RouterDecision{
				decision(0, "planner"),
				decision(1, "developer"),
				decision(2, "developer"),
			},
			wantRun:   2,
			wantLabel: "developer",
		},
		{
			name: "set equality ignores order",
			decisions: []models.RouterDecision{
				decision(0, "developer", "reviewer"),
				decision(1, "reviewer", "developer"),
			},
			wantRun:   2,
			wantLabel: "reviewer, developer",
		},
		{
			name: "empty (waiting) decision breaks the run",
			decisions: []models.RouterDecision{
				decision(0, "developer"),
				decision(1), // waiting — no agents
				decision(2, "developer"),
			},
			wantRun:   1,
			wantLabel: "developer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRun, gotLabel := repeatedDispatchRun(tc.decisions)
			if gotRun != tc.wantRun {
				t.Errorf("run = %d, want %d", gotRun, tc.wantRun)
			}
			if tc.wantRun > 0 && gotLabel != tc.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tc.wantLabel)
			}
		})
	}
}

// TestBuildUserPrompt_RendersRecentHistory — Router должен видеть собственные прошлые решения,
// иначе считает задачу «только созданной» и зацикливается (инцидент SupportAggent).
func TestBuildUserPrompt_RendersRecentHistory(t *testing.T) {
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("Скажи привет", "Поприветствуй пользователя"),
		Agents: []*models.Agent{helperAgent("SupportAggent", "support", "обрабатывает обращения", models.AgentExecutionKindSandbox)},
		RecentDecisions: []models.RouterDecision{
			{StepNo: 0, ChosenAgents: pq.StringArray{"SupportAggent"}, Reason: "задача только создана"},
		},
	}
	out := r.buildUserPrompt(state, "")

	if !strings.Contains(out, "# Recent routing history") {
		t.Fatalf("ожидали секцию истории решений в prompt, не нашли. Output:\n%s", out)
	}
	if !strings.Contains(out, "step 0") || !strings.Contains(out, "SupportAggent") {
		t.Fatalf("ожидали запись о прошлом решении (step 0 / SupportAggent), не нашли")
	}
	if !strings.Contains(out, "ALREADY RAN") {
		t.Fatalf("ожидали предупреждение о повторном запуске уже отработавшего агента")
	}
}

// TestBuildUserPrompt_NoHistorySectionWhenEmpty — на первом шаге истории нет, секцию не рендерим.
func TestBuildUserPrompt_NoHistorySectionWhenEmpty(t *testing.T) {
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("Add JWT auth", "Implement JWT."),
		Agents: []*models.Agent{helperAgent("planner", "planner", "creates plans", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, "")

	if strings.Contains(out, "# Recent routing history") {
		t.Fatalf("не ожидали секцию истории при пустом RecentDecisions. Output:\n%s", out)
	}
}
