package sandbox

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxRunnerTaskOrProjectIDLen = 128

var safeRunnerTaskOrProjectID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// EffectiveTimeout возвращает бизнес-таймаут для таймеров раннера: при Timeout <= 0 — DefaultSandboxTimeout
// (в т.ч. для нуля и отрицательных значений, если Validate пропущен). Validate() отклоняет Timeout < 0.
// Поле Timeout в структуре не мутирует.
func (o SandboxOptions) EffectiveTimeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return DefaultSandboxTimeout
}

// EffectiveStopGrace — длительность graceful stop; при StopGracePeriod <= 0 — DefaultSandboxStopGrace (5.8).
func (o SandboxOptions) EffectiveStopGrace() time.Duration {
	if o.StopGracePeriod > 0 {
		return o.StopGracePeriod
	}
	return DefaultSandboxStopGrace
}

// ValidateTaskID — формат TaskID до Docker/имени контейнера: канонический UUID (github.com/google/uuid) или
// ^[a-zA-Z0-9_-]+$ длиной не более maxRunnerTaskOrProjectIDLen. Без ведущих/хвостовых пробелов (см. Validate).
func ValidateTaskID(s string) error {
	if s == "" {
		return fmt.Errorf("task_id: empty: %w", ErrInvalidTaskID)
	}
	if len(s) > maxRunnerTaskOrProjectIDLen {
		return fmt.Errorf("task_id: exceeds max length: %w", ErrInvalidTaskID)
	}
	if _, err := uuid.Parse(s); err == nil {
		return nil
	}
	if safeRunnerTaskOrProjectID.MatchString(s) {
		return nil
	}
	return fmt.Errorf("task_id: must be UUID or [a-zA-Z0-9_-]+ up to %d chars: %w", maxRunnerTaskOrProjectIDLen, ErrInvalidTaskID)
}

// ValidateProjectID — тот же контракт, что ValidateTaskID, если идентификатор задан (не пустая строка).
// Пустой ProjectID допустим: в контракте 5.5 поле пока не попадает в имена контейнеров/хостовые пути.
func ValidateProjectID(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > maxRunnerTaskOrProjectIDLen {
		return fmt.Errorf("project_id: exceeds max length: %w", ErrInvalidProjectID)
	}
	if _, err := uuid.Parse(s); err == nil {
		return nil
	}
	if safeRunnerTaskOrProjectID.MatchString(s) {
		return nil
	}
	return fmt.Errorf("project_id: must be UUID or [a-zA-Z0-9_-]+ up to %d chars: %w", maxRunnerTaskOrProjectIDLen, ErrInvalidProjectID)
}

func noLeadingTrailingWhitespace(field, s string) error {
	if strings.TrimSpace(s) != s {
		return fmt.Errorf("%w: %s must not have leading or trailing whitespace", ErrInvalidOptions, field)
	}
	return nil
}

// Validate — быстрый фейл до Docker API (вызывается в начале RunTask, 5.5).
// ctx передаётся в ValidateRepoURL для DNS (SSRF); при nil подставляется context.Background().
// Любая ошибка удовлетворяет errors.Is(err, ErrInvalidOptions); часть ошибок дополнительно раскрывает причину через Join.
func (o SandboxOptions) Validate(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// Instruction/Context — многострочные тексты (файлы в контейнере); ведущие/хвостовые пробелы и \n допустимы.
	for _, c := range []struct{ field, val string }{
		{"task_id", o.TaskID},
		{"project_id", o.ProjectID},
		{"backend", string(o.Backend)},
		{"image", o.Image},
		{"repo_url", o.RepoURL},
		{"branch", o.Branch},
	} {
		if err := noLeadingTrailingWhitespace(c.field, c.val); err != nil {
			return err
		}
	}
	if err := ValidateTaskID(o.TaskID); err != nil {
		return errors.Join(ErrInvalidOptions, err)
	}

	if err := ValidateProjectID(o.ProjectID); err != nil {
		return errors.Join(ErrInvalidOptions, err)
	}

	if string(o.Backend) == "" {
		return fmt.Errorf("%w: backend is empty", ErrInvalidOptions)
	}
	switch o.Backend {
	case CodeBackendClaudeCode, CodeBackendAider, CodeBackendCustom:
	default:
		return fmt.Errorf("%w: unsupported backend %q", ErrInvalidOptions, o.Backend)
	}

	if o.Image == "" {
		return fmt.Errorf("%w: image is empty", ErrInvalidOptions)
	}

	if err := ValidateRepoURL(ctx, o.RepoURL); err != nil {
		return errors.Join(ErrInvalidOptions, err)
	}

	if err := ValidateBranchName(o.Branch); err != nil {
		return errors.Join(ErrInvalidOptions, err)
	}

	if o.Instruction == "" {
		return fmt.Errorf("%w: instruction is empty", ErrInvalidOptions)
	}

	if err := ValidateEnvKeys(o.EnvVars); err != nil {
		return errors.Join(ErrInvalidOptions, err)
	}

	if o.Timeout < 0 {
		return fmt.Errorf("%w: timeout must not be negative", ErrInvalidOptions)
	}

	if o.StopGracePeriod < 0 {
		return fmt.Errorf("%w: stop_grace_period must not be negative", ErrInvalidOptions)
	}

	return nil
}
