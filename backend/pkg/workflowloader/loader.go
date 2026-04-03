package workflowloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"gopkg.in/yaml.v3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentConfig структура для YAML агента
type AgentConfig struct {
	Name        string                 `yaml:"name"`
	Role        string                 `yaml:"role"`
	PromptName  string                 `yaml:"prompt_name"`
	ModelConfig map[string]interface{} `yaml:"model_config"`
	IsActive    bool                   `yaml:"is_active"`
}

// WorkflowConfigYAML структура для YAML воркфлоу
type WorkflowConfigYAML struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	StartStep   string                    `yaml:"start_step"`
	MaxSteps    int                       `yaml:"max_steps"`
	Steps       map[string]StepConfigYAML `yaml:"steps"`
}

// StepConfigYAML описание шага в YAML (ссылается на agent_name)
type StepConfigYAML struct {
	Type            string            `yaml:"type"`
	AgentName       string            `yaml:"agent_name,omitempty"`
	Next            *string           `yaml:"next,omitempty"`
	ConditionPrompt string            `yaml:"condition_prompt,omitempty"`
	Routes          map[string]string `yaml:"routes,omitempty"`
	Loop            *LoopConfigYAML   `yaml:"loop,omitempty"`
	APICall         *APICallYAML      `yaml:"api_call,omitempty"`
}

// LoopConfigYAML конфигурация цикла в YAML
type LoopConfigYAML struct {
	BodyStepID     string `yaml:"body_step_id"`
	MaxIterations  int    `yaml:"max_iterations"`
	ExitCondition  string `yaml:"exit_condition"`
	ExitAgentName  string `yaml:"exit_agent_name,omitempty"`
	ExitOnResponse string `yaml:"exit_on_response,omitempty"`
}

// APICallYAML конфигурация API вызова в YAML
type APICallYAML struct {
	Method       string            `yaml:"method"`
	URL          string            `yaml:"url"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	BodyTemplate string            `yaml:"body_template,omitempty"`
	TimeoutSec   int               `yaml:"timeout_sec,omitempty"`
	ExtractPath  string            `yaml:"extract_path,omitempty"`
}

// ScheduleConfigYAML структура для YAML расписания
type ScheduleConfigYAML struct {
	Name           string `yaml:"name"`
	WorkflowName   string `yaml:"workflow_name"`
	CronExpression string `yaml:"cron_expression"`
	InputTemplate  string `yaml:"input_template"`
	IsActive       bool   `yaml:"is_active"`
}

type Loader struct {
	repo       repository.WorkflowRepository
	promptRepo repository.PromptRepository
	db         *gorm.DB // Нужен для транзакций или Upsert логики, если в репо ее нет
}

func New(repo repository.WorkflowRepository, promptRepo repository.PromptRepository, db *gorm.DB) *Loader {
	return &Loader{
		repo:       repo,
		promptRepo: promptRepo,
		db:         db,
	}
}

// LoadAgents загружает агентов из папки
func (l *Loader) LoadAgents(ctx context.Context, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read agents dir: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" && filepath.Ext(file.Name()) != ".yml" {
			continue
		}
		if err := l.loadAgent(ctx, filepath.Join(dirPath, file.Name())); err != nil {
			return fmt.Errorf("failed to load agent %s: %w", file.Name(), err)
		}
	}
	return nil
}

func (l *Loader) loadAgent(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Находим промпт
	prompt, err := l.promptRepo.GetByName(ctx, cfg.PromptName)
	if err != nil {
		return fmt.Errorf("prompt '%s' not found for agent '%s': %w", cfg.PromptName, cfg.Name, err)
	}

	modelConfigJSON, _ := json.Marshal(cfg.ModelConfig)

	agent := &models.Agent{
		Name:        cfg.Name,
		Role:        models.AgentRole(cfg.Role),
		PromptID:    &prompt.ID,
		ModelConfig: datatypes.JSON(modelConfigJSON),
		IsActive:    cfg.IsActive,
	}

	// Upsert Agent
	return l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "prompt_id", "model_config", "is_active", "updated_at"}),
	}).Create(agent).Error
}

// LoadWorkflows загружает воркфлоу из папки
func (l *Loader) LoadWorkflows(ctx context.Context, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read workflows dir: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" && filepath.Ext(file.Name()) != ".yml" {
			continue
		}
		if err := l.loadWorkflow(ctx, filepath.Join(dirPath, file.Name())); err != nil {
			return fmt.Errorf("failed to load workflow %s: %w", file.Name(), err)
		}
	}
	return nil
}

func (l *Loader) loadWorkflow(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg WorkflowConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Преобразуем YAML конфиг в JSON конфиг (заменяя agent_name на agent_id)
	finalConfig := models.WorkflowConfig{
		StartStep: cfg.StartStep,
		MaxSteps:  cfg.MaxSteps,
		Steps:     make(map[string]models.StepConfig),
	}

	for stepID, stepCfg := range cfg.Steps {
		stepType := models.StepType(stepCfg.Type)
		if stepType == "" {
			stepType = models.StepTypeLLM // По умолчанию LLM
		}

		step := models.StepConfig{
			Type:            stepType,
			Next:            stepCfg.Next,
			ConditionPrompt: stepCfg.ConditionPrompt,
			Routes:          stepCfg.Routes,
		}

		// Обрабатываем agent_name если указан
		if stepCfg.AgentName != "" {
			agent, err := l.repo.GetAgentByName(ctx, stepCfg.AgentName)
			if err != nil {
				return fmt.Errorf("agent '%s' not found for step '%s': %w", stepCfg.AgentName, stepID, err)
			}
			step.AgentID = agent.ID.String()
		}

		// Обрабатываем Loop конфигурацию
		if stepCfg.Loop != nil {
			loopCfg := &models.LoopConfig{
				BodyStepID:     stepCfg.Loop.BodyStepID,
				MaxIterations:  stepCfg.Loop.MaxIterations,
				ExitCondition:  stepCfg.Loop.ExitCondition,
				ExitOnResponse: stepCfg.Loop.ExitOnResponse,
			}
			// Если указан exit_agent_name, находим ID
			if stepCfg.Loop.ExitAgentName != "" {
				agent, err := l.repo.GetAgentByName(ctx, stepCfg.Loop.ExitAgentName)
				if err != nil {
					return fmt.Errorf("exit agent '%s' not found for loop step '%s': %w", stepCfg.Loop.ExitAgentName, stepID, err)
				}
				loopCfg.ExitAgentID = agent.ID.String()
			}
			step.Loop = loopCfg
		}

		// Обрабатываем API Call конфигурацию
		if stepCfg.APICall != nil {
			step.APICall = &models.APICallConfig{
				Method:       stepCfg.APICall.Method,
				URL:          stepCfg.APICall.URL,
				Headers:      stepCfg.APICall.Headers,
				BodyTemplate: stepCfg.APICall.BodyTemplate,
				TimeoutSec:   stepCfg.APICall.TimeoutSec,
				ExtractPath:  stepCfg.APICall.ExtractPath,
			}
		}

		finalConfig.Steps[stepID] = step
	}

	configJSON, err := json.Marshal(finalConfig)
	if err != nil {
		return err
	}

	wf := &models.Workflow{
		Name:          cfg.Name,
		Description:   cfg.Description,
		Configuration: datatypes.JSON(configJSON),
		IsActive:      true,
	}

	// Upsert Workflow
	return l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "configuration", "is_active", "updated_at"}),
	}).Create(wf).Error
}

// LoadSchedules загружает расписания из папки
func (l *Loader) LoadSchedules(ctx context.Context, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read schedules dir: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" && filepath.Ext(file.Name()) != ".yml" {
			continue
		}
		if err := l.loadSchedule(ctx, filepath.Join(dirPath, file.Name())); err != nil {
			return fmt.Errorf("failed to load schedule %s: %w", file.Name(), err)
		}
	}
	return nil
}

func (l *Loader) loadSchedule(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg ScheduleConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Проверяем существование воркфлоу
	if _, err := l.repo.GetWorkflowByName(ctx, cfg.WorkflowName); err != nil {
		return fmt.Errorf("workflow '%s' not found for schedule '%s': %w", cfg.WorkflowName, cfg.Name, err)
	}

	schedule := &models.ScheduledWorkflow{
		Name:           cfg.Name,
		WorkflowName:   cfg.WorkflowName,
		CronExpression: cfg.CronExpression,
		InputTemplate:  cfg.InputTemplate,
		IsActive:       cfg.IsActive,
	}

	// Upsert Schedule
	return l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"workflow_name", "cron_expression", "input_template", "is_active", "updated_at"}),
	}).Create(schedule).Error
}
