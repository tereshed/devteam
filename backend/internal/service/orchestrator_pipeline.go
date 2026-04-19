package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/tidwall/gjson"
)

// Sentinel-ошибки pipeline для проверок errors.Is в оркестраторе и тестах.
var (
	ErrPipelineNilResult         = errors.New("pipeline: execution result is nil")
	ErrPipelineEmptyResult       = errors.New("pipeline: agent returned empty result")
	ErrPipelineEmptyDiff         = errors.New("pipeline: developer diff is empty")
	ErrPipelineInvalidTransition = errors.New("pipeline: invalid status transition")
)

// PipelineEngine управляет логикой переходов между этапами выполнения задачи.
type PipelineEngine interface {
	DetermineNextStatus(task *models.Task, result *agent.ExecutionResult) (models.TaskStatus, error)
	GetIterationCount(task *models.Task) int
}

type pipelineEngine struct {
	maxIterations int
}

func NewPipelineEngine(maxIterations int) PipelineEngine {
	if maxIterations <= 0 {
		maxIterations = 5
	}
	return &pipelineEngine{
		maxIterations: maxIterations,
	}
}

func (e *pipelineEngine) DetermineNextStatus(task *models.Task, result *agent.ExecutionResult) (models.TaskStatus, error) {
	// Проверка на nil result (защита от паники)
	if result == nil {
		return models.TaskStatusFailed, ErrPipelineNilResult
	}

	// Если агент сообщил о провале выполнения
	if !result.Success {
		return models.TaskStatusFailed, nil
	}

	// Проверка на пустой ответ от агента (early fail)
	// Проверяем что Output пустой AND (ArtifactsJSON пустой ИЛИ содержит только пустой объект/null)
	artifactsEmpty := len(result.ArtifactsJSON) == 0 ||
		string(result.ArtifactsJSON) == "{}" ||
		string(result.ArtifactsJSON) == "null" ||
		string(result.ArtifactsJSON) == ""
	if result.Output == "" && artifactsEmpty {
		return models.TaskStatusFailed, ErrPipelineEmptyResult
	}

	// Валидация путей в артефактах (Path Traversal Protection)
	if len(result.ArtifactsJSON) > 0 {
		if err := e.validateArtifactPaths(result.ArtifactsJSON); err != nil {
			return models.TaskStatusFailed, fmt.Errorf("pipeline: invalid artifact paths: %w", err)
		}
	}

	switch task.Status {
	case models.TaskStatusPending:
		// Из Pending всегда в Planning (анализ задачи)
		return models.TaskStatusPlanning, nil

	case models.TaskStatusPlanning:
		// После Planning — в разработку
		return models.TaskStatusInProgress, nil

	case models.TaskStatusInProgress:
		// После разработки — на Review; пустой diff в артефактах — явная ошибка
		if len(result.ArtifactsJSON) > 0 {
			diffNode := gjson.GetBytes(result.ArtifactsJSON, "diff")
			if diffNode.Exists() && strings.TrimSpace(diffNode.String()) == "" {
				return models.TaskStatusFailed, ErrPipelineEmptyDiff
			}
		}
		return models.TaskStatusReview, nil

	case models.TaskStatusReview:
		// Reviewer решает: Testing или ChangesRequested
		// Проверяем поле "decision" в артефактах
		decision := gjson.GetBytes(result.ArtifactsJSON, "decision").String()
		if decision == "changes_requested" {
			// Проверяем лимит итераций
			if e.GetIterationCount(task) >= e.maxIterations {
				return models.TaskStatusFailed, ErrOrchestratorIterationLimitReached
			}
			return models.TaskStatusChangesRequested, nil
		}
		return models.TaskStatusTesting, nil

	case models.TaskStatusChangesRequested:
		// Если это была итерация правок — возвращаемся в InProgress
		return models.TaskStatusInProgress, nil

	case models.TaskStatusTesting:
		// После тестов — либо Completed, либо ChangesRequested (если тесты упали)
		decision := gjson.GetBytes(result.ArtifactsJSON, "decision").String()
		if decision == "failed" {
			if e.GetIterationCount(task) >= e.maxIterations {
				return models.TaskStatusFailed, ErrOrchestratorIterationLimitReached
			}
			return models.TaskStatusChangesRequested, nil
		}
		return models.TaskStatusCompleted, nil

	default:
		return models.TaskStatusFailed, fmt.Errorf("%w: unexpected task status %s", ErrPipelineInvalidTransition, task.Status)
	}
}

func (e *pipelineEngine) GetIterationCount(task *models.Task) int {
	// Считаем количество переходов в ChangesRequested из метаданных задачи
	// В MVP используем простое поле в Context или Artifacts, которое обновляет оркестратор.
	// Для более точного подсчета нужно анализировать TaskMessage.
	count := gjson.GetBytes(task.Context, "iteration_count").Int()
	return int(count)
}

func (e *pipelineEngine) validateArtifactPaths(artifacts []byte) error {
	// Ищем все поля, похожие на пути к файлам (например, в массиве "files" или "changed_files")
	res := gjson.GetBytes(artifacts, "files")
	if res.Exists() && res.IsArray() {
		for _, file := range res.Array() {
			if err := e.isSafePath(file.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *pipelineEngine) isSafePath(path string) error {
	if path == "" {
		return nil
	}
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return fmt.Errorf("unsafe path detected: %s", path)
	}
	return nil
}

