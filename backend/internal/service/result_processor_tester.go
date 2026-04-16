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

// TesterProcessor обрабатывает результаты тестировщика (Sandbox/LLM)
type TesterProcessor struct {
	cfg ResultProcessorConfig
}

// NewTesterProcessor создаёт новый экземпляр TesterProcessor
func NewTesterProcessor(cfg ResultProcessorConfig) *TesterProcessor {
	return &TesterProcessor{cfg: cfg}
}

// Process анализирует результат выполнения тестировщика
//
// Логика:
//   - Успех (success: true, test_result: pass) → complete, статус completed
//   - Успех (success: true, test_result: fail) → retry к Developer, статус in_progress
//   - Неудача (success: false) → fail, статус failed (инфраструктурная ошибка)
//
// При test fail проверяется лимит итераций. Если лимит превышен → fail.
// Примечание: проверка на nil result выполняется в ResultProcessor.Process до вызова этого метода.
func (p *TesterProcessor) Process(
	ctx context.Context,
	result *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	// Проверка контекста
	if err := ctx.Err(); err != nil {
		return ProcessResult{}, err
	}

	// Проверка успеха выполнения
	// Важно: success: false у тестера означает инфраструктурную ошибку, не фейл тестов!
	if !result.Success {
		errMsg := "tester infrastructure failed"
		if result.Output != "" {
			// Маскируем секреты перед сохранением в ErrorMessage
			errMsg = fmt.Sprintf("tester infrastructure failed: %s", MaskSecrets(result.Output))
		}
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: errMsg,
			Iterations:   iterations,
		}, nil
	}

	// Определяем результат тестирования
	// Требуем структурированный JSON ответ
	testResult, err := p.extractTestResult(result)
	if err != nil {
		slog.Error("TesterProcessor: failed to extract test result", "error", err)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("failed to parse test result: %v", err),
			Iterations:   iterations,
		}, err // Возвращаем ошибку для системного логирования
	}

	slog.Info("TesterProcessor: test result", "test_result", testResult)

	switch testResult {
	case "pass":
		// Все тесты пройдены → завершение пайплайна
		return p.handleTestPass(result, iterations)

	case "fail":
		// Тесты не пройдены → возврат к разработчику
		return p.handleTestFail(result, iterations)

	default:
		// Неизвестный результат → fail
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("invalid test result: %s", testResult),
			Iterations:   iterations,
		}, nil
	}
}

// handleTestPass обрабатывает успешное прохождение тестов
func (p *TesterProcessor) handleTestPass(
	result *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	var contextAdditions map[string]string
	addContext := func(k, v string) {
		if contextAdditions == nil {
			contextAdditions = make(map[string]string)
		}
		contextAdditions[k] = v
	}

	// Добавляем результаты тестов в контекст
	testReport := p.extractTestReport(result)
	if len(testReport) > 0 {
		if reportJSON, err := json.Marshal(testReport); err == nil {
			addContext("test_report", string(reportJSON))
		} else {
			slog.Error("TesterProcessor: failed to marshal test report", "error", err)
		}
	} else {
		// Добавляем базовый отчет, если структурированных метрик нет
		addContext("test_report", `{"status":"passed"}`)
	}

	if result.Summary != "" {
		addContext("test_summary", MaskSecrets(result.Summary))
	}

	slog.Info("TesterProcessor: all tests passed, pipeline completed")

	return ProcessResult{
		Decision:         DecisionComplete,
		NewStatus:        string(models.TaskStatusCompleted),
		Iterations:       iterations,
		ContextAdditions: contextAdditions,
	}, nil
}

// handleTestFail обрабатывает провал тестов
// Проверяет лимит итераций и либо возвращает к разработчику, либо помечает как failed
func (p *TesterProcessor) handleTestFail(
	result *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	// Проверка лимита итераций
	maxIters := 0
	if p.cfg.MaxTestIterations != nil {
		maxIters = *p.cfg.MaxTestIterations
	}

	if iterations.TestIterations >= maxIters {
		slog.Warn("TesterProcessor: test iteration limit reached",
			"limit", maxIters,
			"iterations", iterations.TestIterations,
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

	// Добавляем отчёт о проваленных тестах
	testReport := p.extractTestReport(result)
	if len(testReport) > 0 {
		if reportJSON, err := json.Marshal(testReport); err == nil {
			addContext("test_failures", string(reportJSON))
		} else {
			slog.Error("TesterProcessor: failed to marshal test failure report", "error", err)
		}
	}

	// Добавляем вывод тестов (с маскированием секретов)
	// Только если нет структурированного отчета, чтобы избежать дублирования
	if len(testReport) == 0 && result.Output != "" {
		addContext("test_output", MaskSecrets(result.Output))
	}

	// Добавляем summary (с маскированием секретов)
	if result.Summary != "" {
		addContext("test_summary", MaskSecrets(result.Summary))
	}

	// Инкрементируем счётчик итераций
	newIterations := IterationCounters{
		ReviewIterations: iterations.ReviewIterations,
		TestIterations:   iterations.TestIterations + 1,
	}

	slog.Info("TesterProcessor: tests failed, returning to developer",
		"test_iterations", newIterations.TestIterations,
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

// extractTestResult извлекает результат тестирования
// Требует структурированный JSON ответ (не парсит plain text)
// Возвращает ошибку если test_result не найден или JSON невалиден
func (p *TesterProcessor) extractTestResult(result *agent.ExecutionResult) (string, error) {
	// Требуем структурированный JSON ответ
	if len(result.ArtifactsJSON) == 0 {
		return "", fmt.Errorf("no artifacts JSON provided: tester must return structured JSON with 'test_result' field")
	}

	// Проверяем поле test_result
	if r := gjson.GetBytes(result.ArtifactsJSON, "test_result").String(); r != "" {
		return strings.ToLower(r), nil
	}

	// Проверяем поле result (альтернативное имя)
	if r := gjson.GetBytes(result.ArtifactsJSON, "result").String(); r != "" {
		return strings.ToLower(r), nil
	}

	// Проверяем поле status (альтернативное имя)
	if r := gjson.GetBytes(result.ArtifactsJSON, "status").String(); r != "" {
		return strings.ToLower(r), nil
	}

	return "", fmt.Errorf("no 'test_result' field found in artifacts JSON: expected 'pass' or 'fail'")
}

// extractTestReport извлекает отчёт о тестах из результата
func (p *TesterProcessor) extractTestReport(result *agent.ExecutionResult) map[string]interface{} {
	report := make(map[string]interface{})

	if len(result.ArtifactsJSON) == 0 {
		return report
	}

	// Пытаемся извлечь структурированный отчёт
	if res := gjson.GetBytes(result.ArtifactsJSON, "test_report"); res.Exists() {
		var testReport map[string]interface{}
		if err := json.Unmarshal([]byte(res.Raw), &testReport); err == nil {
			return testReport
		}
	}

	// Пытаемся извлечь поля по отдельности
	if res := gjson.GetBytes(result.ArtifactsJSON, "passed"); res.Exists() {
		report["passed"] = res.Int()
	}
	if res := gjson.GetBytes(result.ArtifactsJSON, "failed"); res.Exists() {
		report["failed"] = res.Int()
	}
	if res := gjson.GetBytes(result.ArtifactsJSON, "total"); res.Exists() {
		report["total"] = res.Int()
	}
	if res := gjson.GetBytes(result.ArtifactsJSON, "duration_ms"); res.Exists() {
		report["duration_ms"] = res.Int()
	}
	if res := gjson.GetBytes(result.ArtifactsJSON, "coverage"); res.Exists() {
		report["coverage"] = res.Float()
	}

	return report
}
