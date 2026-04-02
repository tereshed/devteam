package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
	"github.com/wibe-flutter-gin-template/backend/pkg/httpclient"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm"
	"gorm.io/datatypes"
)

type WorkflowEngine interface {
	StartWorkflow(ctx context.Context, workflowName string, input string) (*models.Execution, error)
	GetExecution(ctx context.Context, id uuid.UUID) (*models.Execution, error)
	ListWorkflows(ctx context.Context) ([]models.Workflow, error)
	ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error)
	GetExecutionSteps(ctx context.Context, id uuid.UUID) ([]models.ExecutionStep, error)
	RunWorker(ctx context.Context)
}

type workflowEngine struct {
	repo       repository.WorkflowRepository
	llmService LLMService
	httpClient *httpclient.Client
}

func NewWorkflowEngine(repo repository.WorkflowRepository, llmService LLMService) WorkflowEngine {
	return &workflowEngine{
		repo:       repo,
		llmService: llmService,
		httpClient: httpclient.New(),
	}
}

func (e *workflowEngine) StartWorkflow(ctx context.Context, workflowName string, input string) (*models.Execution, error) {
	wf, err := e.repo.GetWorkflowByName(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	var config models.WorkflowConfig
	if err := json.Unmarshal(wf.Configuration, &config); err != nil {
		return nil, fmt.Errorf("invalid workflow config: %w", err)
	}

	if config.StartStep == "" {
		return nil, fmt.Errorf("start_step not defined in workflow config")
	}

	execution := &models.Execution{
		WorkflowID:    wf.ID,
		Status:        models.ExecutionPending,
		CurrentStepID: config.StartStep,
		InputData:     input,
		Context:       datatypes.JSON([]byte("{}")),
		StepCount:     0,
		MaxSteps:      config.MaxSteps,
	}

	if execution.MaxSteps == 0 {
		execution.MaxSteps = 20
	}

	if err := e.repo.CreateExecution(ctx, execution); err != nil {
		return nil, err
	}

	return execution, nil
}

func (e *workflowEngine) GetExecution(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	return e.repo.GetExecutionByID(ctx, id)
}

func (e *workflowEngine) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	return e.repo.ListWorkflows(ctx)
}

func (e *workflowEngine) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	return e.repo.ListExecutions(ctx, limit, offset)
}

func (e *workflowEngine) GetExecutionSteps(ctx context.Context, id uuid.UUID) ([]models.ExecutionStep, error) {
	return e.repo.GetExecutionSteps(ctx, id)
}

func (e *workflowEngine) RunWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("Workflow Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Workflow Worker stopped")
			return
		case <-ticker.C:
			if err := e.processNextTask(ctx); err != nil {
				log.Printf("Error processing task: %v", err)
			}
		}
	}
}

func (e *workflowEngine) processNextTask(ctx context.Context) error {
	exec, err := e.repo.GetNextPendingExecution(ctx)
	if err != nil {
		return nil
	}

	log.Printf("Processing execution %s step %s", exec.ID, exec.CurrentStepID)

	exec.Status = models.ExecutionRunning
	exec.UpdatedAt = time.Now()
	if err := e.repo.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}

	// Парсим конфиг воркфлоу
	var config models.WorkflowConfig
	if err := json.Unmarshal(exec.Workflow.Configuration, &config); err != nil {
		return e.failExecution(ctx, exec, fmt.Sprintf("invalid workflow config: %v", err))
	}

	// Находим текущий шаг
	stepConfig, ok := config.Steps[exec.CurrentStepID]
	if !ok {
		return e.failExecution(ctx, exec, fmt.Sprintf("step %s not found in config", exec.CurrentStepID))
	}

	// Проверяем лимит шагов
	if exec.StepCount >= exec.MaxSteps {
		return e.failExecution(ctx, exec, "max steps limit reached")
	}

	// Выполняем шаг в зависимости от типа
	var stepResult *stepExecutionResult
	switch stepConfig.Type {
	case models.StepTypeLLM, "": // Пустой тип = LLM для обратной совместимости
		stepResult, err = e.executeLLMStep(ctx, exec, stepConfig)
	case models.StepTypeAPICall:
		stepResult, err = e.executeAPICallStep(ctx, exec, stepConfig)
	case models.StepTypeLoop:
		stepResult, err = e.executeLoopStep(ctx, exec, stepConfig, &config)
	case models.StepTypeCondition:
		stepResult, err = e.executeConditionStep(ctx, exec, stepConfig)
	default:
		return e.failExecution(ctx, exec, fmt.Sprintf("unknown step type: %s", stepConfig.Type))
	}

	if err != nil {
		return e.failExecution(ctx, exec, err.Error())
	}

	// Сохраняем результат шага
	if stepResult.step != nil {
		if err := e.repo.AddExecutionStep(ctx, stepResult.step); err != nil {
			return fmt.Errorf("failed to save step: %w", err)
		}
	}

	// Обновляем Execution
	exec.StepCount++

	if stepResult.nextStepID == "" {
		// Конец воркфлоу
		exec.Status = models.ExecutionCompleted
		now := time.Now()
		exec.FinishedAt = &now
		exec.CurrentStepID = ""
		exec.OutputData = stepResult.output
	} else {
		exec.CurrentStepID = stepResult.nextStepID
		exec.InputData = stepResult.output
		exec.Context = stepResult.context
	}

	if err := e.repo.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("failed to update execution state: %w", err)
	}

	return nil
}

// stepExecutionResult результат выполнения шага
type stepExecutionResult struct {
	output     string
	nextStepID string
	step       *models.ExecutionStep
	context    datatypes.JSON
}

// executeLLMStep выполняет шаг с вызовом LLM
func (e *workflowEngine) executeLLMStep(ctx context.Context, exec *models.Execution, stepConfig models.StepConfig) (*stepExecutionResult, error) {
	agentID, err := uuid.Parse(stepConfig.AgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent id: %w", err)
	}

	agent, err := e.repo.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	var promptText string
	if agent.Prompt != nil {
		promptText = agent.Prompt.Template
	}

	var modelConfig struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
	}
	if len(agent.ModelConfig) > 0 {
		if err := json.Unmarshal(agent.ModelConfig, &modelConfig); err != nil {
			log.Printf("Failed to parse agent model config: %v", err)
		}
	}

	inputContent := exec.InputData
	startTime := time.Now()

	llmReq := llm.Request{
		Model:        modelConfig.Model,
		Temperature:  modelConfig.Temperature,
		SystemPrompt: promptText,
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: inputContent,
			},
		},
		Metadata: map[string]any{
			"execution_id": exec.ID.String(),
			"step_id":      exec.CurrentStepID,
			"agent_id":     agent.ID.String(),
		},
	}

	response, err := e.llmService.Generate(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("llm generation failed: %w", err)
	}
	duration := time.Since(startTime)

	step := &models.ExecutionStep{
		ExecutionID:    exec.ID,
		StepID:         exec.CurrentStepID,
		AgentID:        &agent.ID,
		PromptSnapshot: promptText,
		InputContext:   inputContent,
		OutputContent:  response.Content,
		DurationMs:     int(duration.Milliseconds()),
		TokensUsed:     response.Usage.TotalTokens,
	}

	var nextStepID string
	if stepConfig.Next != nil {
		nextStepID = *stepConfig.Next
	}

	return &stepExecutionResult{
		output:     response.Content,
		nextStepID: nextStepID,
		step:       step,
		context:    exec.Context,
	}, nil
}

// executeAPICallStep выполняет HTTP запрос к внешнему API
func (e *workflowEngine) executeAPICallStep(ctx context.Context, exec *models.Execution, stepConfig models.StepConfig) (*stepExecutionResult, error) {
	if stepConfig.APICall == nil {
		return nil, fmt.Errorf("api_call config is required for type=api_call")
	}

	apiConfig := stepConfig.APICall

	if !httpclient.ValidateMethod(apiConfig.Method) {
		return nil, fmt.Errorf("invalid HTTP method: %s", apiConfig.Method)
	}

	startTime := time.Now()

	req := httpclient.Request{
		Method:       apiConfig.Method,
		URL:          apiConfig.URL,
		Headers:      apiConfig.Headers,
		BodyTemplate: apiConfig.BodyTemplate,
		TimeoutSec:   apiConfig.TimeoutSec,
		ExtractPath:  apiConfig.ExtractPath,
	}

	response, err := e.httpClient.Execute(ctx, req, exec.InputData)
	if err != nil {
		return nil, fmt.Errorf("api call failed: %w", err)
	}
	duration := time.Since(startTime)

	// Проверяем статус код
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("api returned status %d: %s", response.StatusCode, response.Body)
	}

	step := &models.ExecutionStep{
		ExecutionID:   exec.ID,
		StepID:        exec.CurrentStepID,
		InputContext:  fmt.Sprintf("%s %s", apiConfig.Method, apiConfig.URL),
		OutputContent: response.Extracted,
		DurationMs:    int(duration.Milliseconds()),
	}

	var nextStepID string
	if stepConfig.Next != nil {
		nextStepID = *stepConfig.Next
	}

	return &stepExecutionResult{
		output:     response.Extracted,
		nextStepID: nextStepID,
		step:       step,
		context:    exec.Context,
	}, nil
}

// executeLoopStep выполняет цикл
func (e *workflowEngine) executeLoopStep(ctx context.Context, exec *models.Execution, stepConfig models.StepConfig, config *models.WorkflowConfig) (*stepExecutionResult, error) {
	if stepConfig.Loop == nil {
		return nil, fmt.Errorf("loop config is required for type=loop")
	}

	loopConfig := stepConfig.Loop

	// Получаем состояние цикла из контекста
	var execContext map[string]any
	if err := json.Unmarshal(exec.Context, &execContext); err != nil {
		execContext = make(map[string]any)
	}

	// Ключ для хранения состояния этого цикла
	loopKey := fmt.Sprintf("loop_%s", exec.CurrentStepID)

	var loopState models.LoopState
	if loopStateRaw, ok := execContext[loopKey]; ok {
		loopStateBytes, _ := json.Marshal(loopStateRaw)
		json.Unmarshal(loopStateBytes, &loopState)
	} else {
		// Инициализируем новый цикл
		loopState = models.LoopState{
			StepID:           exec.CurrentStepID,
			CurrentIteration: 0,
			MaxIterations:    loopConfig.MaxIterations,
			ReturnToStepID:   exec.CurrentStepID,
		}
	}

	// Проверяем лимит итераций
	if loopState.CurrentIteration >= loopState.MaxIterations {
		log.Printf("Loop %s reached max iterations (%d)", exec.CurrentStepID, loopState.MaxIterations)
		// Выходим из цикла
		delete(execContext, loopKey)
		contextBytes, _ := json.Marshal(execContext)

		var nextStepID string
		if stepConfig.Next != nil {
			nextStepID = *stepConfig.Next
		}

		return &stepExecutionResult{
			output:     exec.InputData,
			nextStepID: nextStepID,
			step:       nil, // Не создаём шаг для служебного узла
			context:    datatypes.JSON(contextBytes),
		}, nil
	}

	// Проверяем условие выхода
	shouldExit, err := e.checkLoopExitCondition(ctx, exec, loopConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to check exit condition: %w", err)
	}

	if shouldExit {
		log.Printf("Loop %s exit condition met at iteration %d", exec.CurrentStepID, loopState.CurrentIteration)
		delete(execContext, loopKey)
		contextBytes, _ := json.Marshal(execContext)

		var nextStepID string
		if stepConfig.Next != nil {
			nextStepID = *stepConfig.Next
		}

		return &stepExecutionResult{
			output:     exec.InputData,
			nextStepID: nextStepID,
			step:       nil,
			context:    datatypes.JSON(contextBytes),
		}, nil
	}

	// Продолжаем цикл - переходим к телу цикла
	loopState.CurrentIteration++
	execContext[loopKey] = loopState
	contextBytes, _ := json.Marshal(execContext)

	// Проверяем, что тело цикла существует
	if _, ok := config.Steps[loopConfig.BodyStepID]; !ok {
		return nil, fmt.Errorf("loop body step %s not found", loopConfig.BodyStepID)
	}

	step := &models.ExecutionStep{
		ExecutionID:   exec.ID,
		StepID:        exec.CurrentStepID,
		InputContext:  fmt.Sprintf("Loop iteration %d/%d", loopState.CurrentIteration, loopState.MaxIterations),
		OutputContent: fmt.Sprintf("Entering loop body: %s", loopConfig.BodyStepID),
		DurationMs:    0,
	}

	return &stepExecutionResult{
		output:     exec.InputData,
		nextStepID: loopConfig.BodyStepID,
		step:       step,
		context:    datatypes.JSON(contextBytes),
	}, nil
}

// checkLoopExitCondition проверяет условие выхода из цикла
func (e *workflowEngine) checkLoopExitCondition(ctx context.Context, exec *models.Execution, loopConfig *models.LoopConfig) (bool, error) {
	if loopConfig.ExitCondition == "" {
		// Нет условия - всегда продолжаем
		return false, nil
	}

	// Используем LLM для проверки условия
	exitPrompt := loopConfig.ExitCondition
	exitOnResponse := loopConfig.ExitOnResponse
	if exitOnResponse == "" {
		exitOnResponse = "YES"
	}

	var agentID *uuid.UUID
	if loopConfig.ExitAgentID != "" {
		id, err := uuid.Parse(loopConfig.ExitAgentID)
		if err == nil {
			agentID = &id
		}
	}

	var systemPrompt string
	var modelName string
	var temperature float64 = 0.0 // Низкая для детерминированного ответа

	if agentID != nil {
		agent, err := e.repo.GetAgentByID(ctx, *agentID)
		if err == nil && agent.Prompt != nil {
			systemPrompt = agent.Prompt.Template
			var modelConfig struct {
				Model       string  `json:"model"`
				Temperature float64 `json:"temperature"`
			}
			if len(agent.ModelConfig) > 0 {
				json.Unmarshal(agent.ModelConfig, &modelConfig)
				modelName = modelConfig.Model
				temperature = modelConfig.Temperature
			}
		}
	}

	llmReq := llm.Request{
		Model:        modelName,
		Temperature:  temperature,
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: fmt.Sprintf("%s\n\nCurrent data:\n%s\n\nAnswer only %s or NO.", exitPrompt, exec.InputData, exitOnResponse),
			},
		},
	}

	response, err := e.llmService.Generate(ctx, llmReq)
	if err != nil {
		return false, err
	}

	// Проверяем ответ
	answer := strings.TrimSpace(strings.ToUpper(response.Content))
	return strings.Contains(answer, strings.ToUpper(exitOnResponse)), nil
}

// executeConditionStep выполняет условное ветвление
func (e *workflowEngine) executeConditionStep(ctx context.Context, exec *models.Execution, stepConfig models.StepConfig) (*stepExecutionResult, error) {
	if stepConfig.ConditionPrompt == "" {
		return nil, fmt.Errorf("condition_prompt is required for type=condition")
	}

	if len(stepConfig.Routes) == 0 {
		return nil, fmt.Errorf("routes are required for type=condition")
	}

	// Используем LLM для определения маршрута
	routeOptions := make([]string, 0, len(stepConfig.Routes))
	for route := range stepConfig.Routes {
		routeOptions = append(routeOptions, route)
	}

	conditionPrompt := fmt.Sprintf(`%s

Based on the following input, choose one of these options: %s

Input:
%s

Respond with ONLY one of the options listed above.`,
		stepConfig.ConditionPrompt,
		strings.Join(routeOptions, ", "),
		exec.InputData,
	)

	var agentID *uuid.UUID
	var modelName string
	if stepConfig.AgentID != "" {
		id, err := uuid.Parse(stepConfig.AgentID)
		if err == nil {
			agentID = &id
			agent, err := e.repo.GetAgentByID(ctx, id)
			if err == nil {
				var modelConfig struct {
					Model string `json:"model"`
				}
				if len(agent.ModelConfig) > 0 {
					json.Unmarshal(agent.ModelConfig, &modelConfig)
					modelName = modelConfig.Model
				}
			}
		}
	}

	startTime := time.Now()

	llmReq := llm.Request{
		Model:       modelName,
		Temperature: 0.0, // Детерминированный ответ
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: conditionPrompt,
			},
		},
	}

	response, err := e.llmService.Generate(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("condition evaluation failed: %w", err)
	}
	duration := time.Since(startTime)

	// Определяем маршрут
	answer := strings.TrimSpace(response.Content)
	nextStepID, ok := stepConfig.Routes[answer]
	if !ok {
		// Пробуем найти частичное совпадение
		for route, stepID := range stepConfig.Routes {
			if strings.Contains(strings.ToUpper(answer), strings.ToUpper(route)) {
				nextStepID = stepID
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil, fmt.Errorf("no route matched for answer: %s", answer)
	}

	step := &models.ExecutionStep{
		ExecutionID:   exec.ID,
		StepID:        exec.CurrentStepID,
		AgentID:       agentID,
		InputContext:  conditionPrompt,
		OutputContent: fmt.Sprintf("Condition result: %s -> %s", answer, nextStepID),
		DurationMs:    int(duration.Milliseconds()),
		TokensUsed:    response.Usage.TotalTokens,
	}

	return &stepExecutionResult{
		output:     exec.InputData, // Данные не меняются
		nextStepID: nextStepID,
		step:       step,
		context:    exec.Context,
	}, nil
}

func (e *workflowEngine) failExecution(ctx context.Context, exec *models.Execution, msg string) error {
	exec.Status = models.ExecutionFailed
	exec.ErrorMessage = msg
	now := time.Now()
	exec.FinishedAt = &now
	return e.repo.UpdateExecution(ctx, exec)
}
