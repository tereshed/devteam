package dto

import (
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestToAgentResponse_SandboxAgent_ModelFromSettings(t *testing.T) {
	agentID := uuid.New()
	agent := &models.Agent{
		ID:            agentID,
		Name:          "Sandbox Tester",
		Role:          models.AgentRoleTester,
		ExecutionKind: models.AgentExecutionKindSandbox,
		Model:         nil, // DB column must be nil
		CodeBackendSettings: datatypes.JSON([]byte(`{
			"model": "deepseek/deepseek-v4-flash",
			"hermes": {
				"toolsets": ["file_ops"]
			}
		}`)),
	}

	resp := ToAgentResponse(agent)
	require.Equal(t, agentID.String(), resp.ID)
	require.Equal(t, "Sandbox Tester", resp.Name)
	require.NotNil(t, resp.Model)
	require.Equal(t, "deepseek/deepseek-v4-flash", *resp.Model)
}

func TestToAgentResponse_LLMAgent_ModelFromColumn(t *testing.T) {
	agentID := uuid.New()
	modelName := "gpt-4o"
	agent := &models.Agent{
		ID:            agentID,
		Name:          "LLM Planner",
		Role:          models.AgentRolePlanner,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         &modelName,
	}

	resp := ToAgentResponse(agent)
	require.Equal(t, agentID.String(), resp.ID)
	require.NotNil(t, resp.Model)
	require.Equal(t, "gpt-4o", *resp.Model)
}
