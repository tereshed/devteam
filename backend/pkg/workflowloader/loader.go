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
)

// WorkflowConfigYAML структура для YAML воркфлоу
type WorkflowConfigYAML struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	StartStep   string                    `yaml:"start_step"`
	MaxSteps    int                       `yaml:"max_steps"`
	Steps       map[string]StepConfigYAML `yaml:"steps"`
}

// StepConfigYAML описание шага в YAML
type StepConfigYAML struct {
	Type            string            `yaml:"type"`
	AgentRole       string            `yaml:"agent_role,omitempty"`
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
	ExitAgentRole  string `yaml:"exit_agent_role,omitempty"`
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
	repo repository.WorkflowRepository
}

func New(repo repository.WorkflowRepository) *Loader {
	return &Loader{repo: repo}
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

	finalConfig := models.WorkflowConfig{
		StartStep: cfg.StartStep,
		MaxSteps:  cfg.MaxSteps,
		Steps:     make(map[string]models.StepConfig),
	}

	for stepID, stepCfg := range cfg.Steps {
		stepType := models.StepType(stepCfg.Type)
		if stepType == "" {
			stepType = models.StepTypeLLM
		}

		step := models.StepConfig{
			Type:            stepType,
			AgentRole:       stepCfg.AgentRole,
			Next:            stepCfg.Next,
			ConditionPrompt: stepCfg.ConditionPrompt,
			Routes:          stepCfg.Routes,
		}

		if stepCfg.Loop != nil {
			step.Loop = &models.LoopConfig{
				BodyStepID:     stepCfg.Loop.BodyStepID,
				MaxIterations:  stepCfg.Loop.MaxIterations,
				ExitCondition:  stepCfg.Loop.ExitCondition,
				ExitAgentRole:  stepCfg.Loop.ExitAgentRole,
				ExitOnResponse: stepCfg.Loop.ExitOnResponse,
			}
		}

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

	return l.repo.UpsertWorkflow(ctx, wf)
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

	return l.repo.UpsertScheduledWorkflow(ctx, schedule)
}
