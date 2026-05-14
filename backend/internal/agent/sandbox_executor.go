package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/devteam/backend/internal/sandbox"
)

// SandboxAgentExecutor реализует AgentExecutor через запуск задачи в изолированном контейнере.
// Используется для ролей Developer и Tester.
//
// Безопасность обеспечивается на уровне SandboxRunner: инструкция передается через
// монтирование файла в контейнер (/workspace/instruction.txt), а не через shell-аргументы.
// Это исключает возможность command injection.
type SandboxAgentExecutor struct {
	runner sandbox.SandboxRunner
	image  string // Дефолтный образ (используется когда нет per-backend mapping)
	// Sprint 16: per-backend образа. Если CodeBackend агента есть в этой map —
	// используется соответствующий образ; иначе fallback на image.
	// Immutable после конструктора: read-only из горячего Execute, не требует
	// синхронизации. Регистрация образов происходит при сборке executor'а.
	imageByBackend map[string]string
}

// NewSandboxAgentExecutor создает новый экземпляр SandboxAgentExecutor.
// defaultImage — fallback (например, devteam/sandbox-claude:local) когда
// для CodeBackend нет специализированного образа.
// backendImages — read-only map code_backend → docker image; копируется
// внутрь executor'а, последующая мутация исходной map не влияет на executor.
// Передайте nil или пустой map, если кастомных образов нет.
func NewSandboxAgentExecutor(runner sandbox.SandboxRunner, defaultImage string, backendImages map[string]string) *SandboxAgentExecutor {
	images := make(map[string]string, len(backendImages))
	for k, v := range backendImages {
		if k != "" && v != "" {
			images[k] = v
		}
	}
	return &SandboxAgentExecutor{
		runner:         runner,
		image:          defaultImage,
		imageByBackend: images,
	}
}

// resolveImage выбирает образ контейнера по code_backend агента.
// Если для backend образа не зарегистрировано — fallback на дефолтный.
func (e *SandboxAgentExecutor) resolveImage(codeBackend string) string {
	if img, ok := e.imageByBackend[codeBackend]; ok && img != "" {
		return img
	}
	return e.image
}

// Execute запускает жизненный цикл задачи в контейнере.
func (e *SandboxAgentExecutor) Execute(ctx context.Context, in ExecutionInput) (*ExecutionResult, error) {
	// 1. Валидация входных данных (Early Return)
	if e.runner == nil {
		return nil, ErrExecutorNotConfigured
	}
	if in.TaskID == "" || in.ProjectID == "" || in.GitURL == "" || in.BranchName == "" {
		return nil, fmt.Errorf("%w: TaskID, ProjectID, GitURL and BranchName are required", ErrInvalidExecutionInput)
	}

	// Строгая валидация GitURL (защита от Command Injection / Path Traversal)
	if !strings.HasPrefix(in.GitURL, "http://") &&
		!strings.HasPrefix(in.GitURL, "https://") &&
		!strings.HasPrefix(in.GitURL, "git://") {
		return nil, fmt.Errorf("%w: GitURL must start with http://, https://, or git://", ErrInvalidExecutionInput)
	}

	// Строгая валидация BranchName (защита от инъекций)
	if err := sandbox.ValidateBranchName(in.BranchName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidExecutionInput, err)
	}

	// 2. Установка таймаута (Fallback Timeout)
	executeCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		executeCtx, cancel = context.WithTimeout(ctx, 1*time.Hour) // Дефолт 1 час для sandbox задач
		defer cancel()
	}

	slog.Info("SandboxAgentExecutor.Execute started", "task_id", in.TaskID, "project_id", in.ProjectID)

	// 3. Подготовка опций для SandboxRunner
	// Инструкция передается как есть - безопасность обеспечивается на уровне Runner
	// через монтирование файла в контейнер, а не через shell
	instruction := e.buildInstruction(in)

	envVars := map[string]string{}
	if len(in.EnvSecrets) > 0 {
		maps.Copy(envVars, in.EnvSecrets)
	}
	if in.Model != "" {
		envVars["DEVTEAM_AGENT_MODEL"] = in.Model
	}
	if in.Temperature != nil {
		envVars["DEVTEAM_AGENT_TEMPERATURE"] = fmt.Sprintf("%g", *in.Temperature)
	}
	if in.PromptName != "" {
		envVars["DEVTEAM_AGENT_PROMPT_NAME"] = in.PromptName
	}
	// Reviewer/Tester должны стартовать с уже пушнутой Developer'ом ветки,
	// а не с main, иначе они не увидят его коммита. Developer — наоборот:
	// стартует с main (default — START_REF не выставляем, entrypoint падёт на BASE_REF).
	switch in.Role {
	case "reviewer", "tester":
		if in.BranchName != "" {
			envVars[sandbox.EnvStartRef] = in.BranchName
		}
	}

	opts := sandbox.SandboxOptions{
		TaskID:      in.TaskID,
		ProjectID:   in.ProjectID,
		Backend:     sandbox.CodeBackendType(in.CodeBackend),
		Image:       e.resolveImage(in.CodeBackend),
		RepoURL:     in.GitURL,
		Branch:      in.BranchName,
		Instruction: instruction,
		Context:     EmbedJSONForXML(NormalizeJSONForParse(in.ContextJSON)),
		EnvVars:     envVars,
		Timeout:     0, // SandboxRunner сам подставит дефолт или можно вычислить из ctx
	}

	// 4. Запуск задачи
	instance, err := e.runner.RunTask(executeCtx, opts)

	// Гарантированная очистка контейнера (КРИТИЧНО: независимый контекст)
	// Если RunTask вернул ошибку, но при этом вернул непустой sandboxID, экзекутор ОБЯЗАН вызвать Cleanup.
	// SANDBOX_KEEP_ON_FAILURE=1 — env-gated debug: оставляет контейнер живым ТОЛЬКО при
	// неуспешном завершении (RunTask error, Wait error, или status != Completed). На
	// success-пути cleanup всегда срабатывает — иначе следующий шаг pipeline'а упадёт
	// на `run conflict for task id` (имя контейнера содержит task_id, который pipeline
	// переиспользует для review/testing).
	keepOnFailure := os.Getenv("SANDBOX_KEEP_ON_FAILURE") == "1"
	// keepThisOne обновляется по результатам Wait ниже; defer читает его по closure.
	keepThisOne := false
	if instance != nil && instance.ID != "" {
		sandboxID := instance.ID
		defer func() {
			if keepOnFailure && keepThisOne {
				slog.Warn("SANDBOX_KEEP_ON_FAILURE=1 + failure — sandbox container kept for inspection",
					"sandbox_id", sandboxID, "task_id", in.TaskID)
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if cleanupErr := e.runner.Cleanup(cleanupCtx, sandboxID); cleanupErr != nil {
				slog.Error("failed to cleanup sandbox", "sandbox_id", sandboxID, "error", cleanupErr, "task_id", in.TaskID)
			}
		}()
	}

	if err != nil {
		keepThisOne = true // RunTask failed: запоминаем для defer (cleanup отработает)
		return nil, fmt.Errorf("failed to run sandbox task: %w", err)
	}

	sandboxID := instance.ID

	// 5. Ожидание завершения
	status, err := e.runner.Wait(executeCtx, sandboxID)
	if err != nil {
		keepThisOne = true // Wait failed
		return nil, fmt.Errorf("failed to wait for sandbox: %w", err)
	}
	// Sprint 16 debug: pipeline переиспользует task_id для review/testing, поэтому
	// сохраняем контейнер ТОЛЬКО при non-Completed статусе (failed/timed_out/stopped).
	if status.Status != sandbox.SandboxStatusCompleted {
		keepThisOne = true
	}

	// 6. Обработка результата
	res := &ExecutionResult{
		SandboxInstanceID: sandboxID,
	}

	// Sprint 15.e2e: статусный лог без preview артефактов — Output/Diff/Logs могут
	// содержать токены/URL'ы, утечка которых нарушает инвариант 15.37 (no secret leakage).
	// Превью оставляли только на время диагностики --bare/OAuth; постоянный observability —
	// только counters/flags без содержимого. Для разовой отладки временно закомментируйте
	// defer e.runner.Cleanup выше — контейнер останется, и его логи/файлы можно посмотреть
	// руками через `docker logs <sandbox_id>` и `docker cp <sandbox_id>:/workspace/...`.
	slog.Info("SandboxAgentExecutor.Execute waited",
		"task_id", in.TaskID,
		"sandbox_id", sandboxID,
		"status", string(status.Status),
		"has_result", status.HasResult(),
		"result_success", status.Result != nil && status.Result.Success,
		"log_count", len(status.Logs))
	if status.Status == sandbox.SandboxStatusCompleted && status.HasResult() {
		res.Success = status.Result.Success
		res.Output = e.truncateArtifact(status.Result.Output, "Output")
		res.Summary = fmt.Sprintf("Task completed in sandbox. Success: %v", status.Result.Success)

		// Формируем ArtifactsJSON из CodeResult
		artifacts := map[string]interface{}{
			"diff":        e.truncateArtifact(status.Result.Diff, "Diff"),
			"commit_hash": status.Result.CommitHash,
			"branch_name": status.Result.BranchName,
		}
		artBytes, _ := json.Marshal(artifacts)
		res.ArtifactsJSON = artBytes
	} else {
		res.Success = false
		res.Summary = fmt.Sprintf("Sandbox finished with status: %s", status.Status)
		if len(status.Logs) > 0 {
			res.Output = strings.Join(status.Logs, "\n")
		}
	}

	return res, nil
}

// buildInstruction собирает инструкцию для агента из входных данных.
// Текст передается как есть - безопасность обеспечивается на уровне SandboxRunner.
func (e *SandboxAgentExecutor) buildInstruction(in ExecutionInput) string {
	var sb strings.Builder
	// Оптимизация аллокаций
	sb.Grow(len(in.Title) + len(in.Description) + len(in.PromptUser) + 100)

	if in.Title != "" {
		sb.WriteString("Title: ")
		sb.WriteString(in.Title)
		sb.WriteString("\n\n")
	}
	if in.Description != "" {
		sb.WriteString("Description: ")
		sb.WriteString(in.Description)
		sb.WriteString("\n\n")
	}
	if in.PromptUser != "" {
		sb.WriteString("Instruction: ")
		sb.WriteString(in.PromptUser)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *SandboxAgentExecutor) truncateArtifact(s string, name string) string {
	const limit = 5 * 1024 * 1024 // 5 MB
	if len(s) <= limit {
		return s
	}
	slog.Warn("artifact truncated", "name", name, "size", len(s))
	return s[:limit] + "\n...[TRUNCATED]"
}
