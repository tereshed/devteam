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
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// ContextBuilder собирает и фильтрует контекст для выполнения задачи агентом.
type ContextBuilder interface {
	// Build собирает ExecutionInput. targetRepo (мульти-репо) — целевой репозиторий подзадачи:
	// из него берутся GitURL/ветка/креды; соседние репо проекта монтируются read-only. nil →
	// фолбэк на git-поля проекта (одно-репо/legacy).
	Build(ctx context.Context, task *models.Task, assignedAgent *models.Agent, project *models.Project, targetRepo *models.ProjectRepository) (*agent.ExecutionInput, error)
	// WithCodeChunks добавляет найденные фрагменты кода в контекст (Задача 9.11).
	WithCodeChunks(input *agent.ExecutionInput, chunks []indexer.Chunk) error
}

// PipelinePromptComposer merges base + role system prompts from disk (task 6.8).
type PipelinePromptComposer interface {
	ComposeSystem(role string) (string, error)
	UserTemplate(role string) (string, error)
}

// previousStepMessagesLimit — сколько последних agent-сообщений из task_messages
// показывать следующему агенту pipeline (developer→reviewer и т.п.). Слишком много
// раздуют prompt и токен-бюджет; слишком мало — reviewer/tester не увидят diff.
const previousStepMessagesLimit = 6

type contextBuilder struct {
	encryptor      Encryptor
	composer       PipelinePromptComposer
	sandboxSecrets map[string]string
	taskMsgRepo    repository.TaskMessageRepository
	// Sprint 15.18: динамический резолвер аутентификации sandbox-а (OAuth/proxy/api-key).
	// Если задан, имеет приоритет над sandboxSecrets для ключей ANTHROPIC_*/CLAUDE_CODE_OAUTH_TOKEN.
	authResolver SandboxAuthEnvResolver
	// Sprint 16.C: per-agent артефакты для sandbox-runner'а (settings.json/.mcp.json/
	// permission_mode для Claude; config.yaml/mcp.json/skills/env для Hermes).
	// nil — пайплайн пропускает per-agent сборку и работает legacy-путём.
	agentSettings      AgentSettingsService
	artifactRepo       repository.ArtifactRepository
	gitIntegrationRepo repository.GitIntegrationCredentialRepository
	// tokenRefresher (опц.) рефрешит истёкшие OAuth-токены перед выдачей GIT_TOKEN в sandbox.
	tokenRefresher GitTokenRefresher
}

// WithGitTokenRefresherOption — рефрешер OAuth-токенов для GIT_TOKEN в sandbox (self-hosted GitLab TTL ~2ч).
func WithGitTokenRefresherOption(r GitTokenRefresher) ContextBuilderOption {
	return func(b *contextBuilder) { b.tokenRefresher = r }
}

// SetContextBuilderTokenRefresher — post-construction внедрение рефрешера (gitIntegrationSvc
// создаётся в app.go позже ContextBuilder).
func SetContextBuilderTokenRefresher(cb ContextBuilder, r GitTokenRefresher) {
	if b, ok := cb.(*contextBuilder); ok {
		b.tokenRefresher = r
	}
}

// NewContextBuilder создаёт сборщик контекста.
func NewContextBuilder(encryptor Encryptor, promptComposer PipelinePromptComposer) ContextBuilder {
	return &contextBuilder{
		encryptor: encryptor,
		composer:  promptComposer,
	}
}

func cleanSecrets(secrets map[string]string) map[string]string {
	cleaned := make(map[string]string, len(secrets))
	for k, v := range secrets {
		if k == "" || v == "" {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

// NewContextBuilderWithSandboxSecrets — тот же ContextBuilder, плюс набор секретов,
// которые попадут в EnvSecrets для агентов с CodeBackend != "" (Developer/Tester в sandbox).
// Используется для проброса ANTHROPIC_API_KEY и т.п. в entrypoint sandbox-контейнера.
// Пустые значения и nil-карта игнорируются.
func NewContextBuilderWithSandboxSecrets(encryptor Encryptor, promptComposer PipelinePromptComposer, sandboxSecrets map[string]string) ContextBuilder {
	return &contextBuilder{
		encryptor:      encryptor,
		composer:       promptComposer,
		sandboxSecrets: cleanSecrets(sandboxSecrets),
	}
}

// ContextBuilderOption — функциональная опция для NewContextBuilderFull (Sprint 15.M7).
type ContextBuilderOption func(*contextBuilder)

// WithSandboxAuthResolverOption — динамический резолвер аутентификации sandbox-а (Sprint 15.18).
// Если задан, ANTHROPIC_*/CLAUDE_CODE_OAUTH_TOKEN для агента с CodeBackend != nil заполняются
// через резолвер вместо статических sandboxSecrets.
func WithSandboxAuthResolverOption(resolver SandboxAuthEnvResolver) ContextBuilderOption {
	return func(b *contextBuilder) { b.authResolver = resolver }
}

// WithAgentSettingsServiceOption — Sprint 16.C: подключает AgentSettingsService.
// Без него ContextBuilder возвращает ExecutionInput с AgentSettings == nil
// (legacy: per-agent артефакты не пробрасываются, hermes Skills/MCP не работают).
func WithAgentSettingsServiceOption(svc AgentSettingsService) ContextBuilderOption {
	return func(b *contextBuilder) { b.agentSettings = svc }
}

// WithArtifactRepositoryOption — Sprint 17: подключает ArtifactRepository для подтягивания артефактов из таблицы artifacts.
func WithArtifactRepositoryOption(repo repository.ArtifactRepository) ContextBuilderOption {
	return func(b *contextBuilder) { b.artifactRepo = repo }
}

// WithGitIntegrationRepositoryOption — подтягивает OAuth-креды для git-провайдеров.
func WithGitIntegrationRepositoryOption(repo repository.GitIntegrationCredentialRepository) ContextBuilderOption {
	return func(b *contextBuilder) { b.gitIntegrationRepo = repo }
}

// NewContextBuilderFull — полная конфигурация: секреты sandbox + репозиторий сообщений
// (для подмешивания результата предыдущего шага pipeline в prompt следующего агента).
// Если taskMsgRepo == nil, история не подтягивается (поведение как у двух предыдущих конструкторов).
func NewContextBuilderFull(
	encryptor Encryptor,
	promptComposer PipelinePromptComposer,
	sandboxSecrets map[string]string,
	taskMsgRepo repository.TaskMessageRepository,
	opts ...ContextBuilderOption,
) ContextBuilder {
	cb := &contextBuilder{
		encryptor:      encryptor,
		composer:       promptComposer,
		sandboxSecrets: cleanSecrets(sandboxSecrets),
		taskMsgRepo:    taskMsgRepo,
	}
	for _, o := range opts {
		if o != nil {
			o(cb)
		}
	}
	return cb
}

// WithSandboxAuthResolver — deprecated: используйте NewContextBuilderFull(..., WithSandboxAuthResolverOption(r)).
// Остаётся для обратной совместимости в коде, который уже собран через старую сигнатуру.
//
// Deprecated: переключиться на ContextBuilderOption-вариант.
func WithSandboxAuthResolver(builder ContextBuilder, resolver SandboxAuthEnvResolver) ContextBuilder {
	cb, ok := builder.(*contextBuilder)
	if !ok || cb == nil {
		return builder
	}
	cb.authResolver = resolver
	return cb
}

// resolveGitTokenSecrets выбирает git-креды для клонирования в sandbox.
// Приоритет: (1) выбранный OAuth-аккаунт репо/проекта (git_integration_credential_id);
// (2) PAT-credential репо/проекта (git_credentials); (3) фолбэк на первый OAuth-аккаунт
// провайдера владельца. Мульти-аккаунт: репо/проект могут указывать разные аккаунты.
func (b *contextBuilder) resolveGitTokenSecrets(ctx context.Context, project *models.Project, targetRepo *models.ProjectRepository) map[string]string {
	out := map[string]string{}
	if project == nil {
		return out
	}

	// Эффективный провайдер (репо переопределяет проект) → username для инъекции токена в
	// HTTPS-URL внутри sandbox. GitLab принимает OAuth2-токен только как "oauth2:<token>@",
	// GitHub — "x-access-token:<token>@" (дефолт в entrypoint, не передаём). Ставим заранее,
	// чтобы все ранние return-ветки ниже включали GIT_TOKEN_USER.
	provider := project.GitProvider
	if targetRepo != nil && targetRepo.GitProvider != "" {
		provider = targetRepo.GitProvider
	}
	if provider == models.GitProviderGitLab {
		out["GIT_TOKEN_USER"] = "oauth2"
	}

	// 1. Явно выбранный OAuth-аккаунт: сначала репо, затем проект.
	var integID *uuid.UUID
	if targetRepo != nil && targetRepo.GitIntegrationCredentialID != nil {
		integID = targetRepo.GitIntegrationCredentialID
	} else if project.GitIntegrationCredentialID != nil {
		integID = project.GitIntegrationCredentialID
	}
	if integID != nil && b.gitIntegrationRepo != nil {
		if cred, err := b.gitIntegrationRepo.GetByID(ctx, *integID); err == nil && cred != nil && len(cred.AccessTokenEnc) > 0 {
			if token, ok := b.integrationToken(ctx, cred); ok {
				out["GIT_TOKEN"] = token
				return out
			}
			slog.Error("Failed to resolve selected git integration credential", "project_id", project.ID, "credential_id", *integID)
		}
	}

	// 2. PAT-credential: сначала репо, затем проект.
	gc := project.GitCredential
	if targetRepo != nil && targetRepo.GitCredential != nil {
		gc = targetRepo.GitCredential
	}
	if gc != nil {
		if dec, derr := b.encryptor.Decrypt(gc.EncryptedValue, []byte(gc.ID.String())); derr == nil {
			switch gc.AuthType {
			case models.GitCredentialAuthToken:
				out["GIT_TOKEN"] = string(dec)
			case models.GitCredentialAuthSSHKey:
				out["GIT_SSH_KEY"] = string(dec)
			}
			return out
		} else {
			slog.Error("Failed to decrypt git credential", "project_id", project.ID, "error", derr)
		}
	}

	// 3. Фолбэк: первый OAuth-аккаунт провайдера владельца (эффективный провайдер — репо, иначе проект).
	if b.gitIntegrationRepo != nil && provider != models.GitProviderLocal {
		if integProvider, ok := mapGitProviderToIntegration(provider); ok {
			if cred, err := b.gitIntegrationRepo.GetByUserAndProvider(ctx, project.UserID, integProvider); err == nil && cred != nil && len(cred.AccessTokenEnc) > 0 {
				if token, tok := b.integrationToken(ctx, cred); tok {
					out["GIT_TOKEN"] = token
				} else {
					slog.Error("Failed to resolve fallback git integration credential", "project_id", project.ID)
				}
			}
		}
	}
	return out
}

// integrationToken отдаёт живой access-token интеграционного аккаунта: рефрешит истёкший OAuth
// через tokenRefresher (+ персист), иначе расшифровывает сохранённый. ok=false при ошибке.
func (b *contextBuilder) integrationToken(ctx context.Context, cred *models.GitIntegrationCredential) (string, bool) {
	if b.tokenRefresher != nil {
		if token, err := b.tokenRefresher.FreshAccessToken(ctx, cred); err == nil && token != "" {
			return token, true
		}
		// Рефреш не удался — пробуем сохранённый токен (может ещё жить).
	}
	aad := repository.GitIntegrationCredentialAAD(cred.ID)
	if dec, derr := b.encryptor.Decrypt(cred.AccessTokenEnc, aad); derr == nil {
		return string(dec), true
	}
	return "", false
}

func (b *contextBuilder) Build(ctx context.Context, task *models.Task, assignedAgent *models.Agent, project *models.Project, targetRepo *models.ProjectRepository) (*agent.ExecutionInput, error) {
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
	// Phase 5: пробрасываем agent.ProviderKind в ExecutionInput.Provider.
	// Без этого LLMAgentExecutor оставлял llm.Request.Provider="" и
	// llmService.Generate всегда уходил в defaultProvider (openai),
	// независимо от того, что в БД у агента написан provider_kind=anthropic
	// или provider_kind=openrouter. См. e2e_real seed orchestrator/planner.
	if assignedAgent.ProviderKind != nil {
		input.Provider = string(*assignedAgent.ProviderKind)
	}

	// Phase 1 §1.5: БД — единственный source of truth для model, temperature, max_tokens, system_prompt.
	if assignedAgent.Model != nil && *assignedAgent.Model != "" {
		input.Model = *assignedAgent.Model
	} else if assignedAgent.ExecutionKind == models.AgentExecutionKindSandbox && len(assignedAgent.CodeBackendSettings) > 0 {
		if settings, err := decodeCodeBackendSettings(assignedAgent.CodeBackendSettings); err == nil && settings.Model != "" {
			input.Model = settings.Model
		}
	}
	if assignedAgent.Temperature != nil {
		input.Temperature = assignedAgent.Temperature
	}
	if assignedAgent.MaxTokens != nil {
		input.MaxTokens = assignedAgent.MaxTokens
	}

	// Системный промпт: DB agent.Prompt (versioned) + DB agent.SystemPrompt (additional rules/custom prompt).
	var promptParts []string
	if assignedAgent.Prompt != nil && strings.TrimSpace(assignedAgent.Prompt.Template) != "" {
		promptParts = append(promptParts, assignedAgent.Prompt.Template)
	}
	if assignedAgent.SystemPrompt != nil && strings.TrimSpace(*assignedAgent.SystemPrompt) != "" {
		promptParts = append(promptParts, *assignedAgent.SystemPrompt)
	}
	if len(promptParts) > 0 {
		input.PromptSystem = strings.Join(promptParts, "\n\n")
	}
	if b.composer != nil {
		if sys, err := b.composer.ComposeSystem(string(assignedAgent.Role)); err == nil && strings.TrimSpace(sys) != "" {
			if input.PromptSystem == "" {
				input.PromptSystem = sys
			}
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
		// Sandbox-исполнителю (Developer/Tester c CodeBackend=claude-code) нужны
		// аутентификационные креды внутри контейнера (entrypoint.sh fast-fail'ит без них).
		// Если есть динамический резолвер (Sprint 15.18) — используем его (OAuth/per-user creds/api-key).
		// Иначе — статические sandboxSecrets из конфига (legacy: только ANTHROPIC_API_KEY).
		if b.authResolver != nil {
			for k, v := range b.authResolver.Resolve(ctx, project, assignedAgent).ToEnv() {
				input.EnvSecrets[k] = v
			}
		} else {
			for k, v := range b.sandboxSecrets {
				input.EnvSecrets[k] = v
			}
		}
	}

	// Git информация: мульти-репо — из целевого репо подзадачи, иначе из полей проекта.
	gitURL := project.GitURL
	gitBranch := project.GitDefaultBranch
	if targetRepo != nil {
		if targetRepo.GitURL != "" {
			gitURL = targetRepo.GitURL
		}
		if targetRepo.GitDefaultBranch != "" {
			gitBranch = targetRepo.GitDefaultBranch
		}
	}
	if gitURL != "" {
		input.GitURL = gitURL
	}
	if gitBranch != "" {
		input.GitDefaultBranch = gitBranch
	}
	if task.BranchName != nil {
		input.BranchName = *task.BranchName
	}

	// Git-токен: выбранный OAuth-аккаунт (repo→project) → PAT-credential → фолбэк на первый
	// аккаунт провайдера. Мульти-аккаунт: разные проекты/репо используют разные аккаунты.
	for k, v := range b.resolveGitTokenSecrets(ctx, project, targetRepo) {
		input.EnvSecrets[k] = v
	}

	// Мульти-репо: соседние репозитории (read-only) + каталог репо в промпт.
	if len(project.Repositories) > 1 {
		targetSlug := ""
		if targetRepo != nil {
			targetSlug = targetRepo.Slug
		}
		for i := range project.Repositories {
			sib := &project.Repositories[i]
			if sib.Slug == targetSlug || sib.GitURL == "" {
				continue
			}
			input.SiblingRepos = append(input.SiblingRepos, agent.SiblingRepo{
				Slug:   sib.Slug,
				GitURL: sib.GitURL,
				Branch: sib.GitDefaultBranch,
			})
		}
		if catalog := renderRepoCatalog(project.Repositories); catalog != "" {
			input.PromptUser = catalog + input.PromptUser
		}
	}

	// Подмешиваем артефакты предыдущего шага и недавнюю переписку — иначе
	// reviewer/tester получают только title+description и не видят diff/output от developer.
	b.appendPipelineHandoff(ctx, input, task, assignedAgent)

	// Sprint 16.C — per-agent артефакты для sandbox (config.yaml/mcp.json/skills,
	// permission_mode, hermes env). Без AgentSettingsService — пропускаем (legacy).
	// project передаём явно: SecretResolver использует project.UserID для поиска
	// user_llm_credentials, без него секрет-шаблоны в mcp.json не резолвятся.
	if b.agentSettings != nil && assignedAgent.CodeBackend != nil {
		bundle, err := b.agentSettings.BuildSandboxBundle(ctx, assignedAgent, project)
		if err != nil {
			// Per-agent артефакты — необязательная часть контракта: ошибка их сборки
			// не должна заблокировать LLM-only пайплайн. Логируем и продолжаем
			// с AgentSettings == nil (тот же путь, что и для агентов без CodeBackend).
			slog.Warn("ContextBuilder: BuildSandboxBundle failed; falling back to no per-agent artifacts",
				"agent_id", assignedAgent.ID, "code_backend", *assignedAgent.CodeBackend, "error", err)
		} else {
			input.AgentSettings = bundle
		}
	}

	return input, nil
}

// appendPipelineHandoff добавляет в PromptUser блок XML-tags с результатом
// предыдущего шага: task.Artifacts (последний result агента) + до N последних
// agent-сообщений из task_messages. Все строки маскируются от секретов.
//
// Для первого шага (pending → orchestrator) artifacts пустые и сообщений нет —
// функция тогда ничего не добавляет.
func (b *contextBuilder) appendPipelineHandoff(ctx context.Context, input *agent.ExecutionInput, task *models.Task, currentAgent *models.Agent) {
	var sb strings.Builder

	// 1) Артефакты последнего шага (из artifacts table, иначе legacy task.Artifacts).
	if b.artifactRepo != nil {
		arts, err := b.artifactRepo.ListByTaskID(ctx, task.ID, true)
		if err != nil {
			slog.Warn("ContextBuilder: failed to load artifacts", "task_id", task.ID, "error", err)
		} else if len(arts) > 0 {
			// ListByTaskID возвращает ASC по created_at, берём последний.
			lastArt := arts[len(arts)-1]
			artJSON := b.scrubJSON(lastArt.Content)
			sb.WriteString("\n\n<previous_step_artifacts encoding=\"json\">\n")
			sb.Write(artJSON)
			sb.WriteString("\n</previous_step_artifacts>\n")
		}
	} else if len(task.Artifacts) > 0 && string(task.Artifacts) != "{}" && string(task.Artifacts) != "null" {
		artJSON := b.scrubJSON(task.Artifacts)
		sb.WriteString("\n\n<previous_step_artifacts encoding=\"json\">\n")
		sb.Write(artJSON)
		sb.WriteString("\n</previous_step_artifacts>\n")
	}

	// 2) Последние agent-сообщения (история pipeline). Только если репозиторий есть.
	if b.taskMsgRepo != nil {
		senderAgent := models.SenderTypeAgent
		msgs, _, err := b.taskMsgRepo.ListByTaskID(ctx, task.ID, repository.TaskMessageFilter{
			SenderType: &senderAgent,
			Limit:      previousStepMessagesLimit,
		})
		if err != nil {
			slog.Warn("ContextBuilder: failed to load previous messages", "task_id", task.ID, "error", err)
		} else if len(msgs) > 0 {
			// ListByTaskID сортирует ASC; берём последние N в хронологическом порядке.
			start := 0
			if len(msgs) > previousStepMessagesLimit {
				start = len(msgs) - previousStepMessagesLimit
			}
			sb.WriteString("\n<previous_steps>\n")
			for _, m := range msgs[start:] {
				// We no longer skip messages from the agent itself to provide autobiographical memory
				// (context of previous attempts within the conversation).
				fmt.Fprintf(&sb, "<step agent_id=%q type=%q at=%q>\n",
					m.SenderID.String(), string(m.MessageType), m.CreatedAt.Format("2006-01-02T15:04:05Z"))
				sb.WriteString(b.scrub(m.Content))
				sb.WriteString("\n</step>\n")
			}
			sb.WriteString("</previous_steps>\n")
		}
	}

	if sb.Len() == 0 {
		return
	}
	if input.PromptUser != "" {
		input.PromptUser = input.PromptUser + sb.String()
	} else {
		input.PromptUser = sb.String()
	}
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
