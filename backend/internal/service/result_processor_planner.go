package service

import (
	"context"
	"log/slog"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/tidwall/gjson"
)

// PlannerProcessor обрабатывает результаты планировщика (LLM)
type PlannerProcessor struct {
	cfg ResultProcessorConfig
}

// NewPlannerProcessor создаёт новый экземпляр PlannerProcessor
func NewPlannerProcessor(cfg ResultProcessorConfig) *PlannerProcessor {
	return &PlannerProcessor{cfg: cfg}
}

// Process анализирует результат выполнения планировщика
//
// Логика:
//   - Успех (success: true) + валидный план → next_step к Developer, статус in_progress
//   - Неудача (success: false) или план невалиден → fail, статус failed
//
// Примечание: проверка на nil result выполняется в ResultProcessor.Process до вызова этого метода.
func (p *PlannerProcessor) Process(
	ctx context.Context,
	result *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	// Проверка контекста
	if err := ctx.Err(); err != nil {
		return ProcessResult{}, err
	}

	// Проверка успеха выполнения
	if !result.Success {
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: "planner failed: " + result.Output,
			Iterations:   iterations,
		}, nil
	}

	// Проверка валидности плана
	if !p.isValidPlan(result) {
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: "planner returned invalid plan",
			Iterations:   iterations,
		}, nil
	}

	// Успех: план получен и валиден → следующий шаг Developer
	var contextAdditions map[string]string
	if result.Summary != "" {
		contextAdditions = make(map[string]string)
		contextAdditions["plan_summary"] = result.Summary
	}

	slog.Info("PlannerProcessor: plan created successfully")

	return ProcessResult{
		Decision:         DecisionNextStep,
		NextRole:         string(models.AgentRoleDeveloper),
		NewStatus:        string(models.TaskStatusInProgress),
		Iterations:       iterations,
		ContextAdditions: contextAdditions,
	}, nil
}

// isValidPlan проверяет что план содержит необходимые данные
// Использует gjson для эффективного парсинга без полной десериализации
func (p *PlannerProcessor) isValidPlan(result *agent.ExecutionResult) bool {
	// План должен иметь непустой вывод или валидные артефакты

	// Если есть валидные артефакты с steps/tasks — план валиден
	// Используем gjson для эффективного доступа без полной десериализации
	if len(result.ArtifactsJSON) > 0 {
		// Проверяем наличие steps, tasks или raw_plan через gjson (zero-allocation parsing)
		if gjson.GetBytes(result.ArtifactsJSON, "steps").Exists() {
			return true
		}
		if gjson.GetBytes(result.ArtifactsJSON, "tasks").Exists() {
			return true
		}
		if gjson.GetBytes(result.ArtifactsJSON, "raw_plan").Exists() {
			return true
		}
		// Если JSON есть, но нет нужных полей — проверяем вывод
	}

	// Если нет валидных артефактов, но есть вывод — считаем валидным (plain text plan)
	return result.Output != ""
}
