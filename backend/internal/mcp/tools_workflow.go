package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/service"
)

// --- Params ---

// WorkflowListParams — параметры для workflow_list
type WorkflowListParams struct {
	Limit  *int `json:"limit,omitempty" jsonschema:"description=Макс. количество воркфлоу в ответе (1-100; по умолчанию 50)"`
	Offset *int `json:"offset,omitempty" jsonschema:"description=Сдвиг для пагинации (по умолчанию 0)"`
}

// WorkflowStartParams — параметры для workflow_start
type WorkflowStartParams struct {
	WorkflowName string `json:"workflow_name" jsonschema:"description=Имя воркфлоу для запуска,required"`
	Input        string `json:"input" jsonschema:"description=Входные данные для воркфлоу (текст),required"`
}

// ExecutionIDParams — общие параметры для workflow_status и workflow_steps
type ExecutionIDParams struct {
	ExecutionID string `json:"execution_id" jsonschema:"description=UUID выполнения воркфлоу,required"`
}

// WorkflowStepsParams — параметры для workflow_steps (расширяет ExecutionIDParams пагинацией)
type WorkflowStepsParams struct {
	ExecutionID string `json:"execution_id" jsonschema:"description=UUID выполнения воркфлоу,required"`
	Limit       *int   `json:"limit,omitempty" jsonschema:"description=Макс. количество шагов (1-200; по умолчанию 100)"`
	Offset      *int   `json:"offset,omitempty" jsonschema:"description=Сдвиг для пагинации (по умолчанию 0)"`
}

// --- Data ---

// WorkflowListData — payload для workflow_list
type WorkflowListData struct {
	Workflows []WorkflowItem `json:"workflows"`
	Count     int            `json:"count"`
}

// WorkflowItem — один воркфлоу в списке
type WorkflowItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// WorkflowStartData — payload для workflow_start
type WorkflowStartData struct {
	ExecutionID  string `json:"execution_id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	WorkflowID   string `json:"workflow_id"`
}

// WorkflowStatusData — payload для workflow_status
type WorkflowStatusData struct {
	ExecutionID   string  `json:"execution_id"`
	WorkflowID    string  `json:"workflow_id"`
	Status        string  `json:"status"`
	CurrentStepID string  `json:"current_step_id,omitempty"`
	StepCount     int     `json:"step_count"`
	MaxSteps      int     `json:"max_steps"`
	InputData     string  `json:"input_data,omitempty"`
	OutputData    string  `json:"output_data,omitempty"`
	ErrorMessage  string  `json:"error_message,omitempty"`
	CreatedAt     string  `json:"created_at"`
	FinishedAt    *string `json:"finished_at,omitempty"`
}

// WorkflowStepsData — payload для workflow_steps
type WorkflowStepsData struct {
	ExecutionID string             `json:"execution_id"`
	Steps       []WorkflowStepItem `json:"steps"`
	Count       int                `json:"count"`
}

// WorkflowStepItem — один шаг выполнения
type WorkflowStepItem struct {
	ID            string  `json:"id"`
	StepID        string  `json:"step_id"`
	AgentID       *string `json:"agent_id,omitempty"`
	InputContext  string  `json:"input_context,omitempty"`
	OutputContent string  `json:"output_content,omitempty"`
	TokensUsed    int     `json:"tokens_used"`
	DurationMs    int     `json:"duration_ms"`
	CreatedAt     string  `json:"created_at"`
}

// --- Registration ---

// RegisterWorkflowTools регистрирует MCP-инструменты для работы с воркфлоу
func RegisterWorkflowTools(server *mcp.Server, engine service.WorkflowEngine, cfg config.MCPConfig) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow_list",
		Description: "Получить список активных воркфлоу. Поддерживает пагинацию (limit/offset).",
	}, makeWorkflowListHandler(engine))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow_start",
		Description: "Запустить воркфлоу по имени с входными данными. Возвращает execution_id для отслеживания.",
	}, makeWorkflowStartHandler(engine, cfg))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow_status",
		Description: "Получить текущий статус выполнения воркфлоу по execution_id.",
	}, makeWorkflowStatusHandler(engine))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow_steps",
		Description: "Получить историю шагов выполнения воркфлоу по execution_id. Поддерживает пагинацию (limit/offset).",
	}, makeWorkflowStepsHandler(engine))
}

// --- Handlers ---

func makeWorkflowListHandler(engine service.WorkflowEngine) func(ctx context.Context, req *mcp.CallToolRequest, params *WorkflowListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *WorkflowListParams) (*mcp.CallToolResult, any, error) {
		workflows, err := engine.ListWorkflows(ctx)
		if err != nil {
			return Err("failed to list workflows", err)
		}

		// Фильтруем только активные
		items := make([]WorkflowItem, 0, len(workflows))
		for _, w := range workflows {
			if !w.IsActive {
				continue
			}
			items = append(items, WorkflowItem{
				ID:          w.ID.String(),
				Name:        w.Name,
				Description: w.Description,
			})
		}

		// Пагинация (in-memory; TODO: перенести limit/offset в WorkflowEngine.ListWorkflows когда интерфейс будет расширен)
		limit, offset := PaginateDefaults(params.limitVal(), params.offsetVal(), 50, 100)
		total := len(items)
		items = Paginate(items, limit, offset)

		return OK(
			fmt.Sprintf("found %d active workflows (showing %d, offset %d)", total, len(items), offset),
			&WorkflowListData{
				Workflows: items,
				Count:     total,
			},
		)
	}
}

func makeWorkflowStartHandler(engine service.WorkflowEngine, cfg config.MCPConfig) func(ctx context.Context, req *mcp.CallToolRequest, params *WorkflowStartParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *WorkflowStartParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("parameters are required (workflow_name, input)")
		}

		name := strings.TrimSpace(params.WorkflowName)
		if name == "" {
			return ValidationErr("workflow_name is required")
		}

		input := strings.TrimSpace(params.Input)
		if input == "" {
			return ValidationErr("input is required")
		}
		inputRunes := utf8.RuneCountInString(input)
		if inputRunes > cfg.MaxInputRunes {
			return ValidationErr(fmt.Sprintf(
				"input too long: %d runes (max %d)", inputRunes, cfg.MaxInputRunes))
		}

		execution, err := engine.StartWorkflow(ctx, name, input)
		if err != nil {
			return Err(fmt.Sprintf("failed to start workflow %q", name), err)
		}

		return OK(
			fmt.Sprintf("workflow %q started, execution_id=%s", name, execution.ID.String()),
			&WorkflowStartData{
				ExecutionID:  execution.ID.String(),
				WorkflowName: name,
				Status:       string(execution.Status),
				WorkflowID:   execution.WorkflowID.String(),
			},
		)
	}
}

func makeWorkflowStatusHandler(engine service.WorkflowEngine) func(ctx context.Context, req *mcp.CallToolRequest, params *ExecutionIDParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *ExecutionIDParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("parameters are required (execution_id)")
		}

		execID, err := uuid.Parse(strings.TrimSpace(params.ExecutionID))
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid execution_id: %q", params.ExecutionID))
		}

		execution, err := engine.GetExecution(ctx, execID)
		if err != nil {
			return Err(fmt.Sprintf("failed to get execution %s", execID.String()), err)
		}

		data := &WorkflowStatusData{
			ExecutionID:   execution.ID.String(),
			WorkflowID:    execution.WorkflowID.String(),
			Status:        string(execution.Status),
			CurrentStepID: execution.CurrentStepID,
			StepCount:     execution.StepCount,
			MaxSteps:      execution.MaxSteps,
			InputData:     Truncate(execution.InputData, TruncateDefault),
			OutputData:    Truncate(execution.OutputData, TruncateDefault),
			ErrorMessage:  execution.ErrorMessage,
			CreatedAt:     execution.CreatedAt.Format(time.RFC3339),
		}
		if execution.FinishedAt != nil {
			t := execution.FinishedAt.Format(time.RFC3339)
			data.FinishedAt = &t
		}

		return OK(
			fmt.Sprintf("execution %s: %s (step %d/%d)",
				execution.ID.String(), execution.Status, execution.StepCount, execution.MaxSteps),
			data,
		)
	}
}

func makeWorkflowStepsHandler(engine service.WorkflowEngine) func(ctx context.Context, req *mcp.CallToolRequest, params *WorkflowStepsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *WorkflowStepsParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("parameters are required (execution_id)")
		}

		execID, err := uuid.Parse(strings.TrimSpace(params.ExecutionID))
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid execution_id: %q", params.ExecutionID))
		}

		steps, err := engine.GetExecutionSteps(ctx, execID)
		if err != nil {
			return Err(fmt.Sprintf("failed to get steps for execution %s", execID.String()), err)
		}

		allItems := make([]WorkflowStepItem, 0, len(steps))
		for _, s := range steps {
			item := WorkflowStepItem{
				ID:            s.ID.String(),
				StepID:        s.StepID,
				InputContext:  Truncate(s.InputContext, TruncateShort),
				OutputContent: Truncate(s.OutputContent, TruncateShort),
				TokensUsed:    s.TokensUsed,
				DurationMs:    s.DurationMs,
				CreatedAt:     s.CreatedAt.Format(time.RFC3339),
			}
			if s.AgentID != nil {
				id := s.AgentID.String()
				item.AgentID = &id
			}
			allItems = append(allItems, item)
		}

		// Пагинация (in-memory; TODO: перенести limit/offset в WorkflowEngine.GetExecutionSteps когда интерфейс будет расширен)
		limit, offset := PaginateDefaults(params.limitVal(), params.offsetVal(), 100, 200)
		total := len(allItems)
		items := Paginate(allItems, limit, offset)

		return OK(
			fmt.Sprintf("found %d steps for execution %s (showing %d, offset %d)",
				total, execID.String(), len(items), offset),
			&WorkflowStepsData{
				ExecutionID: execID.String(),
				Steps:       items,
				Count:       total,
			},
		)
	}
}

// --- Pagination helpers ---

// limitVal / offsetVal — безопасные getter'ы для nullable полей
func (p *WorkflowListParams) limitVal() *int {
	if p == nil {
		return nil
	}
	return p.Limit
}
func (p *WorkflowListParams) offsetVal() *int {
	if p == nil {
		return nil
	}
	return p.Offset
}
func (p *WorkflowStepsParams) limitVal() *int {
	if p == nil {
		return nil
	}
	return p.Limit
}
func (p *WorkflowStepsParams) offsetVal() *int {
	if p == nil {
		return nil
	}
	return p.Offset
}
