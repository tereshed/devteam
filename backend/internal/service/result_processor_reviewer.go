package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/tidwall/gjson"
)

// ReviewerProcessor обрабатывает результаты ревьюера (LLM)
type ReviewerProcessor struct {
	cfg ResultProcessorConfig
}

// NewReviewerProcessor создаёт новый экземпляр ReviewerProcessor
func NewReviewerProcessor(cfg ResultProcessorConfig) *ReviewerProcessor {
	return &ReviewerProcessor{cfg: cfg}
}

// Process анализирует результат выполнения ревьюера
//
// Логика:
//   - Успех (success: true, review_decision: approve) → next_step к Tester, статус testing
//   - Успех (success: true, review_decision: changes_requested) → retry к Developer, статус in_progress
//   - Неудача (success: false) → fail, статус failed
//
// При changes_requested проверяется лимит итераций. Если лимит превышен → fail.
// Примечание: проверка на nil result выполняется в ResultProcessor.Process до вызова этого метода.
func (p *ReviewerProcessor) Process(
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
		errMsg := "reviewer failed"
		if result.Output != "" {
			// Маскируем секреты перед сохранением в ErrorMessage
			errMsg = fmt.Sprintf("reviewer failed: %s", MaskSecrets(result.Output))
		}
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: errMsg,
			Iterations:   iterations,
		}, nil
	}

	// Определяем решение ревьюера из артефактов
	// Требуем структурированный JSON ответ (не парсим plain text)
	decision, err := p.extractDecision(result)
	if err != nil {
		// Невалидный JSON или отсутствие decision — системная ошибка
		slog.Error("ReviewerProcessor: failed to extract decision", "error", err)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("failed to parse review decision: %v", err),
			Iterations:   iterations,
		}, err // Возвращаем ошибку для системного логирования
	}

	slog.Info("ReviewerProcessor: review decision", "decision", decision)

	switch decision {
	case "approve":
		// Код одобрен → переход к тестированию
		return ProcessResult{
			Decision:   DecisionNextStep,
			NextRole:   string(models.AgentRoleTester),
			NewStatus:  string(models.TaskStatusTesting),
			Iterations: iterations,
		}, nil

	case "changes_requested":
		// Требуются правки → возврат к разработчику
		return p.handleChangesRequested(result, iterations)

	default:
		// Неизвестное решение → fail
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("invalid review decision: %s", decision),
			Iterations:   iterations,
		}, nil
	}
}

// handleChangesRequested обрабатывает случай запроса изменений
// Проверяет лимит итераций и либо возвращает к разработчику, либо помечает как failed
func (p *ReviewerProcessor) handleChangesRequested(
	result *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	// Проверка лимита итераций
	maxIters := 0
	if p.cfg.MaxReviewIterations != nil {
		maxIters = *p.cfg.MaxReviewIterations
	}
	if iterations.ReviewIterations >= maxIters {
		slog.Warn("ReviewerProcessor: review iteration limit reached",
			"limit", maxIters,
			"iterations", iterations.ReviewIterations,
		)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: "Превышен лимит попыток исправления",
			Iterations:   iterations,
		}, ErrIterationLimitReached
	}

	// Подготовка контекста для разработчика
	var contextAdditions map[string]string
	addContext := func(k, v string) {
		if contextAdditions == nil {
			contextAdditions = make(map[string]string)
		}
		contextAdditions[k] = v
	}

	// Добавляем комментарии ревьюера (только если они структурированные)
	comments := p.extractReviewComments(result)
	if len(comments) > 0 {
		if commentsJSON, err := json.Marshal(comments); err == nil {
			addContext("review_comments", string(commentsJSON))
		} else {
			slog.Error("ReviewerProcessor: failed to marshal review comments", "error", err)
		}
	} else {
		// Если структурированных комментариев нет, передаем сырой вывод как фидбек
		// Это предотвращает дублирование данных в контексте
		if result.Output != "" {
			addContext("reviewer_feedback", MaskSecrets(result.Output))
		}
	}

	// Добавляем summary если есть
	if result.Summary != "" {
		addContext("review_summary", MaskSecrets(result.Summary))
	}

	// Инкрементируем счётчик итераций
	newIterations := IterationCounters{
		ReviewIterations: iterations.ReviewIterations + 1,
		TestIterations:   iterations.TestIterations,
	}

	slog.Info("ReviewerProcessor: changes requested, returning to developer",
		"review_iterations", newIterations.ReviewIterations,
		"max_iterations", maxIters,
	)

	return ProcessResult{
		Decision:         DecisionRetry,
		NextRole:         string(models.AgentRoleDeveloper),
		NewStatus:        string(models.TaskStatusInProgress),
		Iterations:       newIterations,
		ContextAdditions: contextAdditions,
	}, nil
}

// extractDecision извлекает решение ревьюера из результата
// Требует структурированный JSON ответ (не парсим plain text)
// Возвращает ошибку если decision не найден или JSON невалиден
func (p *ReviewerProcessor) extractDecision(result *agent.ExecutionResult) (string, error) {
	// Требуем структурированный JSON ответ
	if len(result.ArtifactsJSON) == 0 {
		return "", fmt.Errorf("no artifacts JSON provided: LLM must return structured JSON with 'decision' field")
	}

	// Проверяем поле decision
	if d := gjson.GetBytes(result.ArtifactsJSON, "decision").String(); d != "" {
		return strings.ToLower(d), nil
	}

	// Проверяем поле review_decision (альтернативное имя)
	if d := gjson.GetBytes(result.ArtifactsJSON, "review_decision").String(); d != "" {
		return strings.ToLower(d), nil
	}

	return "", fmt.Errorf("no 'decision' field found in artifacts JSON: expected 'approve' or 'changes_requested'")
}

// extractReviewComments извлекает комментарии ревьюера из результата
func (p *ReviewerProcessor) extractReviewComments(result *agent.ExecutionResult) []map[string]string {
	var comments []map[string]string

	// Пытаемся извлечь структурированные комментарии из артефактов
	if len(result.ArtifactsJSON) > 0 {
		if res := gjson.GetBytes(result.ArtifactsJSON, "comments"); res.Exists() && res.IsArray() {
			for _, c := range res.Array() {
				comment := make(map[string]string)
				if file := c.Get("file").String(); file != "" {
					comment["file"] = file
				}
				if line := c.Get("line").String(); line != "" {
					comment["line"] = line
				}
				if msg := c.Get("message").String(); msg != "" {
					comment["message"] = msg
				}
				if severity := c.Get("severity").String(); severity != "" {
					comment["severity"] = severity
				}
				if len(comment) > 0 {
					comments = append(comments, comment)
				}
			}
		}
	}

	return comments
}
