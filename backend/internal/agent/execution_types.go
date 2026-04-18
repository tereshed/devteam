package agent

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const stringLogTruncate = 256

// ExecutionInput — данные, которые оркестратор собрал до вызова исполнителя.
// Рекомендация: не передавать *gorm.DB и не передавать целые ORM-графы без необходимости;
// копируйте строки/UUID в поля ввода, чтобы юнит-тесты исполнителя не тянули БД.
//
// String реализует fmt.Stringer: значения EnvSecrets маскируются как "***" (защита от случайного логирования).
// Запрещено логировать структуру через reflect или %#v в обход String().
type ExecutionInput struct {
	TaskID    string
	ProjectID string

	Title       string
	Description string
	// ContextJSON — сырой JSON задачи (models.Task.Context).
	// Семантика: nil или len==0 трактуется как пустой объект "{}" при парсинге (см. NormalizeJSONForParse).
	ContextJSON json.RawMessage

	AgentID   string
	AgentName string
	Role      string

	Model        string
	PromptSystem string
	PromptUser   string
	// PromptName — идентификатор промпта из backend/agents/*.yaml (задача 6.9).
	PromptName string
	// Temperature / MaxTokens — параметры LLM из предзагруженного YAML-конфига агента (nil = не заданы).
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

	CodeBackend string

	EnvSecrets map[string]string

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
	b.WriteString(" StructuredContext:")
	writeJSONLenOrEmpty(&b, in.StructuredContext)
	b.WriteString(" EnvSecrets:")
	if len(in.EnvSecrets) == 0 {
		b.WriteString("{}")
	} else {
		keys := make([]string, 0, len(in.EnvSecrets))
		for k := range in.EnvSecrets {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q:***", k)
		}
		b.WriteByte('}')
	}
	b.WriteByte('}')
	return b.String()
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
