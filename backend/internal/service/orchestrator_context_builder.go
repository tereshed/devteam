package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/agentsloader"
	"github.com/devteam/backend/pkg/llm"
)

// ContextBuilder собирает и фильтрует контекст для выполнения задачи агентом.
type ContextBuilder interface {
	Build(ctx context.Context, task *models.Task, assignedAgent *models.Agent, project *models.Project) (*agent.ExecutionInput, error)
	// WithCodeChunks добавляет найденные фрагменты кода в контекст (Задача 9.11).
	WithCodeChunks(input *agent.ExecutionInput, chunks []indexer.Chunk) error
}

// PipelinePromptComposer merges base + role system prompts from disk (task 6.8).
type PipelinePromptComposer interface {
	ComposeSystem(role string) (string, error)
	UserTemplate(role string) (string, error)
}

type contextBuilder struct {
	encryptor Encryptor
	composer  PipelinePromptComposer
	agentCfg  *agentsloader.Cache
}

// NewContextBuilder создаёт сборщик контекста. agentCfg — предзагруженный кэш backend/agents (6.9); nil в тестах.
func NewContextBuilder(encryptor Encryptor, promptComposer PipelinePromptComposer, agentCfg *agentsloader.Cache) ContextBuilder {
	return &contextBuilder{
		encryptor: encryptor,
		composer:  promptComposer,
		agentCfg:  agentCfg,
	}
}

func (b *contextBuilder) Build(ctx context.Context, task *models.Task, assignedAgent *models.Agent, project *models.Project) (*agent.ExecutionInput, error) {
	// Маскируем Title и Description как обычные строки
	scrubbedTitle := b.scrub(task.Title)
	scrubbedDescription := b.scrub(task.Description)

	// Маскируем ContextJSON как JSON структуру
	scrubbedContext := b.scrubJSON(task.Context)

	input := &agent.ExecutionInput{
		TaskID:      task.ID.String(),
		ProjectID:   task.ProjectID.String(),
		Title:       scrubbedTitle,
		Description: scrubbedDescription,
		ContextJSON: scrubbedContext,
		AgentID:     assignedAgent.ID.String(),
		AgentName:   assignedAgent.Name,
		Role:        string(assignedAgent.Role),
		EnvSecrets:  make(map[string]string),
	}

	if b.agentCfg != nil {
		if cfg := b.resolveDefaultAgentConfig(assignedAgent); cfg != nil {
			if cfg.ModelConfig.Model != "" {
				input.Model = cfg.ModelConfig.Model
			}
			input.PromptName = cfg.PromptName
			input.Temperature = llm.Float64Ptr(cfg.ModelConfig.Temperature)
			if cfg.ModelConfig.MaxTokens > 0 {
				input.MaxTokens = llm.IntPtr(cfg.ModelConfig.MaxTokens)
			}
		}
	}
	if input.Model == "" && assignedAgent.Model != nil {
		input.Model = *assignedAgent.Model
	}

	// Системный промпт агента (из БД; при наличии композера pipeline — переопределяется YAML base+role)
	if assignedAgent.Prompt != nil {
		input.PromptSystem = assignedAgent.Prompt.Template
	}
	if b.composer != nil {
		if sys, err := b.composer.ComposeSystem(string(assignedAgent.Role)); err == nil && strings.TrimSpace(sys) != "" {
			input.PromptSystem = sys
		} else if err != nil {
			slog.Warn("pipeline prompt compose failed; keeping DB system prompt", "role", assignedAgent.Role, "error", err)
		}
		if ut, err := b.composer.UserTemplate(string(assignedAgent.Role)); err == nil && strings.TrimSpace(ut) != "" {
			if input.PromptUser != "" {
				input.PromptUser = ut + "\n\n" + input.PromptUser
			} else {
				input.PromptUser = ut
			}
		}
	}

	if assignedAgent.CodeBackend != nil {
		input.CodeBackend = string(*assignedAgent.CodeBackend)
	}

	// Git информация из проекта
	if project.GitURL != "" {
		input.GitURL = project.GitURL
	}
	if project.GitDefaultBranch != "" {
		input.GitDefaultBranch = project.GitDefaultBranch
	}
	if task.BranchName != nil {
		input.BranchName = *task.BranchName
	}

	// Дешифровка Git-кредов если есть
	if project.GitCredential != nil {
		decrypted, err := b.encryptor.Decrypt(project.GitCredential.EncryptedValue, []byte(project.GitCredential.ID.String()))
		if err != nil {
			slog.Error("Failed to decrypt git credentials", "project_id", project.ID, "error", err)
			return nil, fmt.Errorf("failed to decrypt git credentials: %w", err)
		}

		// В зависимости от типа кредов кладем в EnvSecrets
		switch project.GitCredential.AuthType {
		case models.GitCredentialAuthToken:
			input.EnvSecrets["GIT_TOKEN"] = string(decrypted)
		case models.GitCredentialAuthSSHKey:
			input.EnvSecrets["GIT_SSH_KEY"] = string(decrypted)
		}
	}

	// Сбор дополнительного контекста (Vector DB, история сообщений)
	// TODO: Интеграция с VectorRepository и TaskMessageRepository

	return input, nil
}

func (b *contextBuilder) resolveDefaultAgentConfig(agent *models.Agent) *agentsloader.AgentConfig {
	if b.agentCfg == nil {
		return nil
	}
	switch agent.Role {
	case models.AgentRoleOrchestrator, models.AgentRolePlanner, models.AgentRoleDeveloper,
		models.AgentRoleReviewer, models.AgentRoleTester:
		if cfg, ok := b.agentCfg.GetByPipelineRole(string(agent.Role)); ok {
			return cfg
		}
	}
	if cfg, ok := b.agentCfg.GetByName(agent.Name); ok {
		return cfg
	}
	return nil
}

func (b *contextBuilder) scrubJSON(data []byte) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage("{}")
	}

	var m interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		// Если это не JSON, маскируем как строку (на всякий случай)
		return json.RawMessage(`"` + b.scrub(string(data)) + `"`)
	}

	scrubbed := b.scrubValue(m)
	res, _ := json.Marshal(scrubbed)
	return json.RawMessage(res)
}

func (b *contextBuilder) scrubValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return b.scrub(val)
	case map[string]interface{}:
		newMap := make(map[string]interface{})
		for k, v := range val {
			newMap[k] = b.scrubValue(v)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(val))
		for i, v := range val {
			newSlice[i] = b.scrubValue(v)
		}
		return newSlice
	default:
		return v
	}
}

func (b *contextBuilder) scrub(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			// Оставляем ключ, маскируем значение
			splitRe := regexp.MustCompile(`[\s:=]+`)
			parts := splitRe.Split(match, 2)
			if len(parts) == 2 {
				return parts[0] + ": [MASKED]"
			}
			return "[MASKED]"
		})
	}
	return s
}

// WithCodeChunks реализует добавление фрагментов кода в промпт.
// Использует быструю аппроксимацию токенов (1 токен ≈ 4 символа).
func (b *contextBuilder) WithCodeChunks(input *agent.ExecutionInput, chunks []indexer.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Лимит контекста для чанков (аппроксимация). 
	// Допустим, мы хотим выделить под чанки не более 4000 токенов (~16000 символов).
	const maxChars = 16000
	const minScore = 0.7
	
	var sb strings.Builder
	firstChunk := true

	currentChars := 0
	addedCount := 0

	for _, chunk := range chunks {
		// 1. Similarity Threshold (Порог релевантности)
		if chunk.Score < minScore {
			continue
		}

		// 2. Fast Token Approximation & Performance optimization
		// Сначала проверяем примерный размер, чтобы избежать лишних аллокаций
		approxLen := len(chunk.FilePath) + len(chunk.Symbol) + len(chunk.Content) + 150
		if currentChars+approxLen > maxChars {
			break
		}

		if firstChunk {
			sb.WriteString("\n\n--- CODE CONTEXT ---\n")
			sb.WriteString("The following code snippets are retrieved automatically via vector search. They might be incomplete or slightly outdated. Use them as a reference.\n\n")
			currentChars = sb.Len()
			firstChunk = false
		}

		// 3. XML-теги для защиты от Prompt Injection
		sb.WriteString("<code_chunk file=\"")
		sb.WriteString(chunk.FilePath)
		if chunk.Symbol != "" {
			sb.WriteString("\" symbol=\"")
			sb.WriteString(chunk.Symbol)
		}
		sb.WriteString("\" lines=\"")
		sb.WriteString(fmt.Sprintf("%d-%d", chunk.StartLine, chunk.EndLine))
		sb.WriteString("\">\n")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n</code_chunk>\n\n")
		
		currentChars = sb.Len()
		addedCount++
	}

	if addedCount > 0 {
		input.PromptUser += sb.String()
	}

	return nil
}
