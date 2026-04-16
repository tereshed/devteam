package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/tidwall/gjson"
)

// DeveloperProcessor обрабатывает результаты разработчика (Sandbox/LLM)
type DeveloperProcessor struct {
	cfg ResultProcessorConfig
}

// NewDeveloperProcessor создаёт новый экземпляр DeveloperProcessor
func NewDeveloperProcessor(cfg ResultProcessorConfig) *DeveloperProcessor {
	return &DeveloperProcessor{cfg: cfg}
}

// Process анализирует результат выполнения разработчика
//
// Логика:
//   - Успех (success: true) + код написан → next_step к Reviewer, статус review
//   - Неудача (success: false) → fail, статус failed
//
// Примечание: проверка на nil result выполняется в ResultProcessor.Process до вызова этого метода.
func (p *DeveloperProcessor) Process(
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
		errMsg := "developer failed"
		if result.Output != "" {
			// Маскируем секреты перед сохранением в ErrorMessage
			errMsg = fmt.Sprintf("developer failed: %s", MaskSecrets(result.Output))
		}
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: errMsg,
			Iterations:   iterations,
		}, nil
	}

	// Валидация артефактов (изменённые файлы)
	if err := p.validateArtifacts(result); err != nil {
		slog.Error("DeveloperProcessor: artifact validation failed", "error", err)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("artifact validation failed: %v", err),
			Iterations:   iterations,
		}, err // Возвращаем ошибку валидации как error для системного логирования
	}

	// Успех: код написан → следующий шаг Reviewer
	var contextAdditions map[string]string

	// Добавляем информацию о diff в контекст
	sandboxID := result.SandboxInstanceID
	if sandboxID != "" {
		contextAdditions = make(map[string]string)
		contextAdditions["sandbox_instance_id"] = sandboxID
	}

	// Добавляем информацию об изменённых файлах если есть
	files := p.extractChangedFiles(result)
	if len(files) > 0 {
		if filesJSON, err := json.Marshal(files); err == nil {
			if contextAdditions == nil {
				contextAdditions = make(map[string]string)
			}
			contextAdditions["changed_files"] = string(filesJSON)
		} else {
			slog.Error("DeveloperProcessor: failed to marshal changed files", "error", err)
		}
	}

	slog.Info("DeveloperProcessor: code developed successfully",
		"changed_files_count", len(files),
		"sandbox_instance_id", sandboxID,
	)

	return ProcessResult{
		Decision:         DecisionNextStep,
		NextRole:         string(models.AgentRoleReviewer),
		NewStatus:        string(models.TaskStatusReview),
		Iterations:       iterations,
		ContextAdditions: contextAdditions,
	}, nil
}

// validateArtifacts проверяет артефакты на валидность
// - Проверяет paths на path traversal используя WorkspaceRoot из конфига
// Использует gjson для эффективного парсинга без полной десериализации
func (p *DeveloperProcessor) validateArtifacts(result *agent.ExecutionResult) error {
	if len(result.ArtifactsJSON) == 0 {
		// Нет артефактов — это не ошибка, просто нет изменений
		return nil
	}

	// Проверяем пути файлов через gjson (zero-allocation пути)
	filesResult := gjson.GetBytes(result.ArtifactsJSON, "files")
	if filesResult.IsArray() {
		for _, file := range filesResult.Array() {
			path := file.String()
			if path != "" {
				if err := ValidateArtifactPath(path, p.cfg.WorkspaceRoot); err != nil {
					return err
				}
			}
		}
	}

	// Проверяем пути в changed_files если есть
	changedFilesResult := gjson.GetBytes(result.ArtifactsJSON, "changed_files")
	if changedFilesResult.IsArray() {
		for _, file := range changedFilesResult.Array() {
			path := file.String()
			if path != "" {
				if err := ValidateArtifactPath(path, p.cfg.WorkspaceRoot); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// extractChangedFiles извлекает список изменённых файлов из артефактов
// Использует gjson для эффективного парсинга без полной десериализации
// Дедуплицирует файлы через map
func (p *DeveloperProcessor) extractChangedFiles(result *agent.ExecutionResult) []string {
	if len(result.ArtifactsJSON) == 0 {
		return nil
	}

	uniqueFiles := make(map[string]struct{})

	// Извлекаем из поля files через gjson
	filesResult := gjson.GetBytes(result.ArtifactsJSON, "files")
	if filesResult.IsArray() {
		for _, file := range filesResult.Array() {
			if s := file.String(); s != "" {
				uniqueFiles[s] = struct{}{}
			}
		}
	}

	// Извлекаем из поля changed_files
	changedFilesResult := gjson.GetBytes(result.ArtifactsJSON, "changed_files")
	if changedFilesResult.IsArray() {
		for _, file := range changedFilesResult.Array() {
			if s := file.String(); s != "" {
				uniqueFiles[s] = struct{}{}
			}
		}
	}

	if len(uniqueFiles) == 0 {
		return nil
	}

	files := make([]string, 0, len(uniqueFiles))
	for f := range uniqueFiles {
		files = append(files, f)
	}

	return files
}
