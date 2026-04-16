package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/devteam/backend/pkg/llm"
)

var jsonArtifactsRegex = regexp.MustCompile("(?s)```json\n?(.*?)\n?```")

// LLMAgentExecutor реализует AgentExecutor через прямой вызов LLM.
// Используется для ролей Orchestrator, Planner, Reviewer.
type LLMAgentExecutor struct {
	client llm.Provider
}

// NewLLMAgentExecutor создает новый экземпляр LLMAgentExecutor.
func NewLLMAgentExecutor(client llm.Provider) *LLMAgentExecutor {
	return &LLMAgentExecutor{
		client: client,
	}
}

// Execute выполняет один шаг агента через LLM.
func (e *LLMAgentExecutor) Execute(ctx context.Context, in ExecutionInput) (*ExecutionResult, error) {
	if e.client == nil {
		return nil, ErrExecutorNotConfigured
	}

	if in.TaskID == "" {
		return nil, fmt.Errorf("%w: TaskID is required", ErrInvalidExecutionInput)
	}

	// Логируем входные данные (секреты маскируются через in.String())
	slog.Debug("LLMAgentExecutor.Execute started", "input", in.String())

	// Формируем промпт с защитой от Prompt Injection через XML-теги
	userPrompt := e.buildUserPrompt(in)

	req := llm.Request{
		Model:        in.Model,
		SystemPrompt: in.PromptSystem,
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: userPrompt,
			},
		},
		// В MVP используем значения по умолчанию для Temperature/MaxTokens или можно расширить ExecutionInput
	}

	resp, err := e.client.Generate(ctx, req)
	if err != nil {
		// Сначала проверяем отмену контекста
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrExecutionCancelled, err)
		}

		// Маппинг ошибок провайдера в Sentinel ошибки пакета agent
		// В реальном LLMClient должны быть типизированные ошибки или проверка строк
		errMsg := err.Error()
		if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
			return nil, fmt.Errorf("%w: %v", ErrRateLimit, err)
		}
		if strings.Contains(errMsg, "context_length_exceeded") || strings.Contains(errMsg, "too many tokens") {
			return nil, fmt.Errorf("%w: %v", ErrContextTooLarge, err)
		}
		return nil, fmt.Errorf("llm generate failed: %w", err)
	}

	result := &ExecutionResult{
		Success:          true,
		Output:           resp.Content,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
	}

	// Попытка извлечь структурированные данные (Artifacts) из ответа
	e.extractArtifacts(resp.Content, result)

	return result, nil
}

// buildUserPrompt формирует пользовательский промпт с XML-тегами для изоляции данных.
func (e *LLMAgentExecutor) buildUserPrompt(in ExecutionInput) string {
	var sb strings.Builder

	if in.Title != "" {
		sb.WriteString("<task_title>\n")
		sb.WriteString(in.Title)
		sb.WriteString("\n</task_title>\n\n")
	}

	if in.Description != "" {
		sb.WriteString("<task_description>\n")
		sb.WriteString(in.Description)
		sb.WriteString("\n</task_description>\n\n")
	}

	if len(in.ContextJSON) > 0 {
		sb.WriteString("<task_context>\n")
		sb.Write(NormalizeJSONForParse(in.ContextJSON))
		sb.WriteString("\n</task_context>\n\n")
	}

	if len(in.StructuredContext) > 0 {
		sb.WriteString("<role_context>\n")
		sb.Write(NormalizeJSONForParse(in.StructuredContext))
		sb.WriteString("\n</role_context>\n\n")
	}

	if in.PromptUser != "" {
		sb.WriteString("<user_instruction>\n")
		sb.WriteString(in.PromptUser)
		sb.WriteString("\n</user_instruction>\n")
	}

	return sb.String()
}

// extractArtifacts ищет JSON-блоки в ответе и пытается распарсить их в ArtifactsJSON.
func (e *LLMAgentExecutor) extractArtifacts(content string, res *ExecutionResult) {
	// Ищем блоки кода с json: ```json ... ```
	matches := jsonArtifactsRegex.FindStringSubmatch(content)

	if len(matches) > 1 {
		jsonStr := strings.TrimSpace(matches[1])
		if json.Valid([]byte(jsonStr)) {
			res.ArtifactsJSON = json.RawMessage(jsonStr)
			// Summary можно взять из первой строки или оставить пустым для заполнения оркестратором
			res.Summary = "Extracted structured artifacts from LLM response"
		} else {
			// Если JSON невалиден, помечаем успех как false согласно контракту
			res.Success = false
			res.Summary = "LLM returned invalid JSON in artifacts block"
		}
	} else {
		// Если артефактов нет, summary — просто начало ответа
		if len(content) > 100 {
			res.Summary = content[:100] + "..."
		} else {
			res.Summary = content
		}
	}
}
