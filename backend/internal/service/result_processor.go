package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
)

const (
	// defaultMaxReviewIterations - лимит по умолчанию для циклов Review → Develop
	defaultMaxReviewIterations = 3
	// defaultMaxTestIterations - лимит по умолчанию для циклов Test → Develop
	defaultMaxTestIterations = 3
	// defaultOutputLimit - лимит на размер вывода агента (OOM protection)
	defaultOutputLimit = 10 * 1024 // 10KB
)

var (
	// ErrUnknownRole возвращается при неизвестной роли
	ErrUnknownRole = errors.New("result_processor: unknown role")
	// ErrNilExecutionResult возвращается при nil ExecutionResult
	ErrNilExecutionResult = errors.New("result_processor: execution result is nil")
	// ErrPathTraversal возвращается при обнаружении path traversal
	ErrPathTraversal = errors.New("result_processor: path traversal detected")
	// ErrIterationLimitReached возвращается при превышении лимита итераций
	ErrIterationLimitReached = errors.New("result_processor: iteration limit reached")
)

// secretPatterns — скомпилированные один раз при загрузке пакета (MustCompile).
// Используются MaskSecrets и scrub в orchestrator_context_builder.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|auth[_-]?token|secret|password|passwd|bearer|token)[\s:=]+[^\s,]{8,}`),
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9\-._~+/]+=*`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)[^\s]+`),
}

// PipelineDecision - решение о следующем шаге пайплайна
type PipelineDecision string

const (
	// DecisionNextStep - переход к следующей роли
	DecisionNextStep PipelineDecision = "next_step"
	// DecisionRetry - возврат к предыдущей роли (Developer)
	DecisionRetry PipelineDecision = "retry"
	// DecisionFail - перевод задачи в failed
	DecisionFail PipelineDecision = "fail"
	// DecisionComplete - завершение пайплайна
	DecisionComplete PipelineDecision = "complete"
)

// ProcessResult - результат обработки, определяющий следующий шаг
// Возвращается по значению (не по указателю) — оптимизация аллокаций
type ProcessResult struct {
	Decision         PipelineDecision
	NextRole         string            // Роль следующего агента (если Decision == next_step)
	NewStatus        string            // Новый статус задачи
	Iterations       IterationCounters // Обновлённые значения счётчиков
	ErrorMessage     string            // При Decision == fail
	ContextAdditions map[string]string // Дополнительный контекст для следующего агента
}

// IterationCounters - счётчики циклов возврата
type IterationCounters struct {
	ReviewIterations int
	TestIterations   int
}

// Config - конфигурация ResultProcessor
type ResultProcessorConfig struct {
	MaxReviewIterations *int   // default: 3
	MaxTestIterations   *int   // default: 3
	OutputLimit         int    // default: 10KB (OOM protection)
	WorkspaceRoot       string // базовая директория для валидации путей (path traversal protection)
}

// RoleProcessor - обработчик для конкретной роли (Strategy pattern)
type RoleProcessor interface {
	// Process анализирует результат выполнения агента и возвращает решение о следующем шаге
	Process(
		ctx context.Context,
		result *agent.ExecutionResult,
		iterations IterationCounters,
	) (ProcessResult, error)
}

// ResultProcessor — основной маршрутизатор результатов агентов
type ResultProcessor interface {
	Process(
		ctx context.Context,
		currentRole string,
		currentStatus string,
		executionResult *agent.ExecutionResult,
		iterations IterationCounters,
	) (ProcessResult, error)
}

// resultProcessorImpl - реализация маршрутизатора результатов агентов
type resultProcessorImpl struct {
	cfg        ResultProcessorConfig
	processors map[string]RoleProcessor // маппинг роль → обработчик
}

// NewResultProcessor создаёт новый экземпляр ResultProcessor
func NewResultProcessor(cfg ResultProcessorConfig, processors map[string]RoleProcessor) ResultProcessor {
	// Применяем значения по умолчанию
	if cfg.MaxReviewIterations == nil {
		def := defaultMaxReviewIterations
		cfg.MaxReviewIterations = &def
	}
	if cfg.MaxTestIterations == nil {
		def := defaultMaxTestIterations
		cfg.MaxTestIterations = &def
	}
	if cfg.OutputLimit <= 0 {
		cfg.OutputLimit = defaultOutputLimit
	}

	// Если процессоры не переданы, используем дефолтные
	if processors == nil {
		processors = make(map[string]RoleProcessor)
	}

	rp := &resultProcessorImpl{
		cfg:        cfg,
		processors: processors,
	}

	// Регистрируем стандартные процессоры, если они ещё не зарегистрированы
	rp.registerDefaultProcessors()

	return rp
}

// registerDefaultProcessors регистрирует стандартные обработчики ролей
func (p *resultProcessorImpl) registerDefaultProcessors() {
	plannerKey := string(models.AgentRolePlanner)
	if _, exists := p.processors[plannerKey]; !exists {
		p.processors[plannerKey] = NewPlannerProcessor(p.cfg)
	}
	devKey := string(models.AgentRoleDeveloper)
	if _, exists := p.processors[devKey]; !exists {
		p.processors[devKey] = NewDeveloperProcessor(p.cfg)
	}
	revKey := string(models.AgentRoleReviewer)
	if _, exists := p.processors[revKey]; !exists {
		p.processors[revKey] = NewReviewerProcessor(p.cfg)
	}
	testKey := string(models.AgentRoleTester)
	if _, exists := p.processors[testKey]; !exists {
		p.processors[testKey] = NewTesterProcessor(p.cfg)
	}
}

// Process анализирует результат выполнения агента и возвращает решение о следующем шаге
func (p *resultProcessorImpl) Process(
	ctx context.Context,
	currentRole string,
	currentStatus string,
	executionResult *agent.ExecutionResult,
	iterations IterationCounters,
) (ProcessResult, error) {
	// Проверка контекста
	if err := ctx.Err(); err != nil {
		return ProcessResult{}, err
	}

	// Early Return: проверка на nil
	if executionResult == nil {
		slog.Error("ResultProcessor.Process: execution result is nil", "role", currentRole)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: ErrNilExecutionResult.Error(),
			Iterations:   iterations,
		}, ErrNilExecutionResult
	}

	// Копируем результат, чтобы не мутировать оригинал (Stateless)
	resultCopy := *executionResult

	// Логирование начала обработки
	slog.Info("ResultProcessor.Process: processing result",
		"role", currentRole,
		"status", currentStatus,
		"success", resultCopy.Success,
	)

	// Находим обработчик для роли
	processor, exists := p.processors[strings.ToLower(currentRole)]
	if !exists {
		slog.Error("ResultProcessor.Process: unknown role", "role", currentRole)
		return ProcessResult{
			Decision:     DecisionFail,
			NewStatus:    string(models.TaskStatusFailed),
			ErrorMessage: fmt.Sprintf("unknown role: %s", currentRole),
			Iterations:   iterations,
		}, ErrUnknownRole
	}

	// OOM Protection: truncate вывода ДО обработки (работаем с копией)
	p.truncateResult(&resultCopy)

	// Делегируем обработку конкретному процессору
	result, err := processor.Process(ctx, &resultCopy, iterations)

	// Логирование результата
	if err != nil {
		slog.Error("ResultProcessor.Process: processor failed",
			"role", currentRole,
			"error", err,
			"decision", result.Decision,
		)
	} else {
		slog.Info("ResultProcessor.Process: result processed",
			"role", currentRole,
			"decision", result.Decision,
			"next_role", result.NextRole,
			"new_status", result.NewStatus,
			"review_iterations", result.Iterations.ReviewIterations,
			"test_iterations", result.Iterations.TestIterations,
		)
	}

	return result, err
}

// truncateResult усекает вывод агента для защиты от OOM
// ВАЖНО: выполняется ДО любой обработки строк
// Использует strings.Builder с Grow() как требуется в ТЗ
func (p *resultProcessorImpl) truncateResult(result *agent.ExecutionResult) {
	const truncatedSuffix = "\n...[truncated]"

	// Truncate Output (сохраняем UTF-8, используя strings.Builder)
	if len(result.Output) > p.cfg.OutputLimit {
		truncated := truncateUTF8(result.Output, p.cfg.OutputLimit)
		var b strings.Builder
		b.Grow(len(truncated) + len(truncatedSuffix))
		b.WriteString(truncated)
		b.WriteString(truncatedSuffix)
		result.Output = b.String()
	}

	// Truncate Summary (используем strings.Builder)
	if len(result.Summary) > p.cfg.OutputLimit {
		truncated := truncateUTF8(result.Summary, p.cfg.OutputLimit)
		var b strings.Builder
		b.Grow(len(truncated) + len(truncatedSuffix))
		b.WriteString(truncated)
		b.WriteString(truncatedSuffix)
		result.Summary = b.String()
	}

	// Если ArtifactsJSON слишком большой — не пытаемся чинить (это почти невозможно сделать безопасно)
	// Вместо этого очищаем и логируем
	if len(result.ArtifactsJSON) > p.cfg.OutputLimit {
		slog.Warn("ResultProcessor: ArtifactsJSON exceeds limit, clearing",
			"size", len(result.ArtifactsJSON),
			"limit", p.cfg.OutputLimit,
		)
		result.ArtifactsJSON = []byte(`{"error": "artifacts truncated due to size limit"}`)
	}
}

// MaskSecrets маскирует секреты в строке (токены, пароли, ключи)
// Используется перед сохранением вывода агента в ErrorMessage или ContextAdditions
func MaskSecrets(input string) string {
	if input == "" {
		return input
	}
	result := input
	for _, re := range secretPatterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// truncateUTF8 усекает строку по границе руны (UTF-8 safe)
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Идем с конца обрезанной строки и ищем границу руны
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return s[:maxBytes]
}

// ValidateArtifactPath проверяет путь на path traversal
// Используется filepath.Clean() + filepath.Rel() для безопасной проверки
func ValidateArtifactPath(path string, workspaceRoot string) error {
	if path == "" {
		return nil
	}

	// Нормализуем путь
	cleaned := filepath.Clean(path)

	// Проверяем на выход за пределы рабочей директории
	// Путь должен быть относительным и не содержать ../
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("%w: absolute path not allowed: %s", ErrPathTraversal, path)
	}

	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") {
		return fmt.Errorf("%w: path traversal detected in: %s", ErrPathTraversal, path)
	}

	// Если указана базовая директория, проверяем что путь внутри неё
	// Используем filepath.Rel для безопасной проверки (не уязвима к /tmp/workspace_hacked атаке)
	if workspaceRoot != "" {
		absWorkspace, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace root: %w", err)
		}

		// Строим абсолютный путь относительно workspaceRoot
		fullPath := filepath.Join(absWorkspace, cleaned)
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			return fmt.Errorf("failed to resolve path: %w", err)
		}

		// Используем filepath.Rel для безопасной проверки
		// Если путь вне workspace, Rel вернёт путь, начинающийся с ".."
		rel, err := filepath.Rel(absWorkspace, absPath)
		if err != nil {
			return fmt.Errorf("%w: failed to get relative path: %w", ErrPathTraversal, err)
		}
		if strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%w: path %s is outside workspace %s", ErrPathTraversal, path, workspaceRoot)
		}
	}

	return nil
}
