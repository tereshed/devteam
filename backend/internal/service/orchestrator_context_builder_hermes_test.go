package service

import (
	"context"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// Sprint 16.C — гарантирует, что ContextBuilder подключает AgentSettingsService
// и кладёт собранный bundle в ExecutionInput.AgentSettings.
//
// Без этого теста легко регрессировать: можно случайно убрать
// WithAgentSettingsServiceOption в main.go и фича 16.C опять становится мёртвым
// кодом — а текущие unit-тесты builder'ов этого не заметят.

func TestContextBuilder_Build_HermesAgent_PopulatesAgentSettings(t *testing.T) {
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	cb := &contextBuilder{
		encryptor:     NoopEncryptor{},
		agentSettings: svc,
	}

	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	hermesCB := models.CodeBackendHermes

	task := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "T",
	}
	a := &models.Agent{
		ID:                  agentID,
		Name:                "hermes-dev",
		Role:                models.AgentRoleDeveloper,
		CodeBackend:         &hermesCB,
		CodeBackendSettings: []byte(`{"hermes":{"toolsets":["file_ops"],"permission_mode":"yolo"}}`),
	}
	project := &models.Project{ID: projectID}

	input, err := cb.Build(context.Background(), task, a, project)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if input.AgentSettings == nil {
		t.Fatalf("ExecutionInput.AgentSettings is nil — pipeline сломан: bundle никогда не доедет до runner")
	}
	if len(input.AgentSettings.HermesConfigYAML) == 0 {
		t.Fatalf("HermesConfigYAML missing in input")
	}
	if input.AgentSettings.HermesEnv["DEVTEAM_HERMES_PERMISSION_MODE"] != "yolo" {
		t.Fatalf("permission_mode env lost: %v", input.AgentSettings.HermesEnv)
	}
}

func TestContextBuilder_Build_AgentWithoutCodeBackend_AgentSettingsNil(t *testing.T) {
	// orchestrator/planner: CodeBackend==nil → AgentSettings должен остаться nil
	// (нечего копировать в контейнер; runner работает legacy-путём).
	svc := NewAgentSettingsServiceWithDeps(nil, nil)
	cb := &contextBuilder{encryptor: NoopEncryptor{}, agentSettings: svc}

	a := &models.Agent{ID: uuid.New(), Name: "planner", Role: models.AgentRolePlanner}
	input, err := cb.Build(context.Background(),
		&models.Task{ID: uuid.New(), ProjectID: uuid.New()}, a, &models.Project{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if input.AgentSettings != nil {
		t.Fatalf("AgentSettings must be nil for agent without CodeBackend, got: %+v", input.AgentSettings)
	}
}
