package agent

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/devteam/backend/internal/sandbox"
)

const stringLogTruncate = 256

// SiblingRepo — соседний репозиторий проекта, монтируемый в sandbox read-only.
type SiblingRepo struct {
	Slug   string
	GitURL string
	Branch string
}

// ExecutionInput — данные, которые оркестратор собрал до вызова исполнителя.
// Рекомендация: не передавать *gorm.DB и не передавать целые ORM-графы без необходимости;
// копируйте строки/UUID в поля ввода, чтобы юнит-тесты исполнителя не тянули БД.
//
// String реализует fmt.Stringer: значения EnvSecrets маскируются как "***" (защита от случайного логирования).
// Запрещено логировать структуру через reflect или %#v в обход String().
type ExecutionInput struct {
	TaskID      string
	ProjectID   string
	ExecutionID string

	Title       string
	Description string
	// ContextJSON — сырой JSON задачи (models.Task.Context).
	// Семантика: nil или len==0 трактуется как пустой объект "{}" при парсинге (см. NormalizeJSONForParse).
	ContextJSON json.RawMessage

	AgentID   string
	AgentName string
	Role      string

	// OwnerUserID — владелец проекта (uuid-строка). LLM-executor пробрасывает его
	// в llm.Request: ключи провайдеров из блока пользователя (user_llm_credentials)
	// приоритетнее env-ключей процесса. Пусто → поведение как раньше (env).
	OwnerUserID string

	Model string
	// Provider — kind LLM-провайдера (openai/anthropic/openrouter/...).
	// Заполняется ContextBuilder'ом из agent.ProviderKind. Пустая строка →
	// llmService.Generate упадёт на defaultProvider. Тип — string, чтобы
	// internal/agent не тащил pkg/llm import; преобразование выполняется
	// в llm_executor.go при сборке llm.Request.
	Provider     string
	PromptSystem string
	PromptUser   string
	// PromptName — идентификатор промпта (задача 6.9).
	PromptName string
	// Temperature / MaxTokens — параметры LLM из БД агента (nil = не заданы).
	Temperature *float64
	MaxTokens   *int

	// GitURL, GitDefaultBranch, BranchName — строки из БД/пользователя/LLM.
	// Реализации 6.2–6.3 обязаны валидировать формат; при вызове git и оболочки после фиксированных флагов
	// использовать разделитель "--" перед пользовательскими ref/путями (например: git checkout -- <branch>),
	// чтобы значения вида -h или --upload-pack не интерпретировались как флаги.
	// GitURL не должен содержать учётные данные в userinfo; секреты — только в EnvSecrets.
	GitURL           string
	GitDefaultBranch string
	BranchName       string

	// SiblingRepos — соседние репозитории проекта (мульти-репо), которые монтируются
	// в sandbox в режиме read-only (контракты/типы для согласования, напр. API↔UI).
	// Целевой (writable) репозиторий подзадачи передаётся через GitURL/BranchName выше.
	SiblingRepos []SiblingRepo

	// Services — эфемерные сервис-сайдкары прогона (Sprint 22): runner поднимает их
	// рядом с агент-контейнером (postgres для интеграционных тестов с БД). Заполняется
	// AgentWorker'ом для агентов с attach_sandbox_services из sandbox_service_configs
	// проекта. Пароль генерится на каждый прогон → не логируется, не хранится.
	Services []sandbox.ServiceSpec

	CodeBackend string

	// AgentSettings — Sprint 16.C: per-agent артефакты для sandbox-runner'а
	// (settings.json/.mcp.json/permission_mode для Claude; config.yaml/mcp.json/skills
	// для Hermes). Заполняется ContextBuilder через AgentSettingsService.BuildSandboxBundle.
	// nil — runner не копирует ничего сверх prompt/context (legacy-агенты без CodeBackend).
	AgentSettings *sandbox.AgentSettingsBundle

	EnvSecrets map[string]string

	// ProjectEnv — «переменные проекта» (project_secrets с inject_as_env=true): произвольные
	// пользовательские env-переменные, доступные агенту в песочнице как $VAR. Отдельно от
	// EnvSecrets, т.к. проходят мягкую валидацию ключей (ValidateProjectEnvKeys) и
	// маскируются в логах полностью. Значения — секреты.
	ProjectEnv map[string]string

	// InjectedEnvFile — «инъекция env-файла» уровня репозитория (опционально): содержимое,
	// имя файла и относительная папка. ContextBuilder заполняет по целевому репо задачи;
	// runner стейджит контент в контейнер, entrypoint пишет файл в рабочую копию репо
	// после checkout и исключает его из git. nil — инъекции нет.
	InjectedEnvFile *sandbox.InjectedEnvFileSpec

	// StructuredContext — сырой JSON доп. контекста роли.
	// Семантика: nil или len==0 трактуется как "{}" при парсинге (см. NormalizeJSONForParse).
	StructuredContext json.RawMessage
}

// ExecutionResult — нормализованный выход одного Execute для записи в Task / TaskMessage оркестратором.
// Success: см. godoc AgentExecutor и таблицу error vs Success в задаче 6.1.
type ExecutionResult struct {
	Success bool
	Summary string
	Output  string

	// ArtifactsJSON — сырой JSON для models.Task.Artifacts; nil/пусто — нет артефактов.
	ArtifactsJSON json.RawMessage

	PromptTokens      int
	CompletionTokens  int
	SandboxInstanceID string
}

// String маскирует значения EnvSecrets как "***" (fmt.Stringer). Длинные текстовые поля усечены для логов.
func (in ExecutionInput) String() string {
	var b strings.Builder
	b.WriteString("ExecutionInput{TaskID:")
	b.WriteString(in.TaskID)
	b.WriteString(" ProjectID:")
	b.WriteString(in.ProjectID)
	b.WriteString(" ExecutionID:")
	b.WriteString(in.ExecutionID)
	b.WriteString(" Title:")
	b.WriteString(truncateForLog(in.Title, stringLogTruncate))
	b.WriteString(" Description:")
	b.WriteString(truncateForLog(in.Description, stringLogTruncate))
	b.WriteString(" ContextJSON:")
	writeJSONLenOrEmpty(&b, in.ContextJSON)
	b.WriteString(" AgentID:")
	b.WriteString(in.AgentID)
	b.WriteString(" AgentName:")
	b.WriteString(in.AgentName)
	b.WriteString(" Role:")
	b.WriteString(in.Role)
	b.WriteString(" Model:")
	b.WriteString(in.Model)
	if in.Temperature != nil {
		fmt.Fprintf(&b, " Temperature:%g", *in.Temperature)
	}
	b.WriteString(" PromptSystem:")
	b.WriteString(truncateForLog(in.PromptSystem, stringLogTruncate))
	b.WriteString(" PromptUser:")
	b.WriteString(truncateForLog(in.PromptUser, stringLogTruncate))
	b.WriteString(" GitURL:")
	b.WriteString(in.GitURL)
	b.WriteString(" GitDefaultBranch:")
	b.WriteString(in.GitDefaultBranch)
	b.WriteString(" BranchName:")
	b.WriteString(in.BranchName)
	b.WriteString(" CodeBackend:")
	b.WriteString(in.CodeBackend)
	if len(in.Services) > 0 {
		// Только count + aliases: Env сервиса несёт сгенерированный пароль БД.
		b.WriteString(" Services:[")
		for i, s := range in.Services {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(s.Alias)
		}
		b.WriteByte(']')
	}
	b.WriteString(" StructuredContext:")
	writeJSONLenOrEmpty(&b, in.StructuredContext)
	b.WriteString(" EnvSecrets:")
	writeMaskedEnvKeys(&b, in.EnvSecrets)
	b.WriteString(" ProjectEnv:")
	writeMaskedEnvKeys(&b, in.ProjectEnv)
	b.WriteString(" InjectedEnvFile:")
	if in.InjectedEnvFile != nil {
		// Имя/папка не секретны; содержимое — секрет (только длина).
		fmt.Fprintf(&b, "{file:%q dir:%q content:<%d bytes>}",
			in.InjectedEnvFile.FileName, in.InjectedEnvFile.TargetDir, len(in.InjectedEnvFile.Content))
	} else {
		b.WriteString("nil")
	}
	b.WriteByte('}')
	return b.String()
}

// writeMaskedEnvKeys печатает {"KEY":***, ...} с отсортированными именами и скрытыми
// значениями — общий помощник для EnvSecrets и ProjectEnv (оба несут секреты).
func writeMaskedEnvKeys(b *strings.Builder, m map[string]string) {
	if len(m) == 0 {
		b.WriteString("{}")
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%q:***", k)
	}
	b.WriteByte('}')
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func writeJSONLenOrEmpty(b *strings.Builder, raw json.RawMessage) {
	if len(raw) == 0 {
		b.WriteString("{}")
		return
	}
	fmt.Fprintf(b, "len=%d", len(raw))
}

// NormalizeJSONForParse возвращает "{}" если raw — nil или пустой слайс, иначе — исходные байты.
// Реализации 6.2–6.3 должны использовать это перед json.Unmarshal, чтобы пустой ввод не был ошибкой:
// контракт — пустой смысл = пустой объект, не invalid JSON.
func NormalizeJSONForParse(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
