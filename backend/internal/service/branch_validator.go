package service

import (
	"errors"
	"regexp"
	"strings"
)

// branch_validator.go — Sprint 17 / Orchestration v2 — защита git-операций от
// command/flag injection через имена веток и базовые ref'ы.
//
// Принцип defence-in-depth: каждый источник имени ветки проходит через эти валидаторы
// ДО формирования exec.Command. CHECK constraint'ы в БД (worktrees.base_branch
// и worktrees.branch_name) служат вторым уровнем защиты.
//
// Ключевая обязанность вызывающего кода: использовать `--` separator во ВСЕХ
// git-командах после фиксированных флагов. Это уже project-wide convention
// (см. internal/agent/execution_types.go godoc).

// ErrBranchUnsafe — sentinel-ошибка для отказа в валидации.
var ErrBranchUnsafe = errors.New("branch name failed safety validation")

// baseBranchRe — формат, идентичный CHECK chk_worktrees_base_branch_safe в миграции 036.
//
// Допускает: a-z, A-Z, 0-9, ., _, /, - (но НЕ в первой позиции).
// Длина: 1-128 символов.
// Это покрывает все типичные паттерны (main, master, develop, release/1.2, feature/foo-bar).
var baseBranchRe = regexp.MustCompile(`^[a-zA-Z0-9._/][a-zA-Z0-9._/-]{0,127}$`)

// dangerousBranchSubstrings — дополнительная защита поверх regex'а: запрещённые
// последовательности, которые могут проскочить через regex (например, "..")
// или указывают на инъекцию.
var dangerousBranchSubstrings = []string{
	"..",
	"//", // двойной слеш может ломать парсер git
	"\x00",
	"\n",
	"\r",
	"\t",
	" ",  // пробел в имени ветки — крайне необычно и подозрительно
	"@{", // git ref-syntax вида branch@{...} может означать reflog
}

// ValidateBaseBranch — главная точка входа для проверки имени базовой ветки,
// приходящего из конфига проекта / ввода пользователя / ответа LLM.
//
// Возвращает nil если имя безопасно для подстановки в `git worktree add ... -- <base>`
// (с обязательным `--` separator перед base).
//
// Гарантии:
//   - Не начинается с '-' (нельзя выдать за git-флаг).
//   - Содержит только разрешённые символы.
//   - Длина 1-128 байт.
//   - Не содержит запрещённых подстрок.
//   - Не является зарезервированным служебным ref'ом (HEAD, FETCH_HEAD).
func ValidateBaseBranch(name string) error {
	if name == "" {
		return errBranchUnsafe("empty")
	}
	if len(name) > 128 {
		return errBranchUnsafe("too long (>128 bytes)")
	}
	// Защита от git-флаг injection: ведущий '-' (regex это уже исключает, но дублируем
	// явно для читаемости и устойчивости к будущим изменениям регулярки).
	if strings.HasPrefix(name, "-") {
		return errBranchUnsafe("leads with '-' (looks like a flag)")
	}
	// Ведущий '.' запрещён git (см. git check-ref-format) и открывает путь к
	// path-traversal вариациям типа "./" или ".branch".
	if strings.HasPrefix(name, ".") {
		return errBranchUnsafe("leads with '.' (disallowed by git ref-format)")
	}
	if !baseBranchRe.MatchString(name) {
		return errBranchUnsafe("contains disallowed characters")
	}
	for _, bad := range dangerousBranchSubstrings {
		if strings.Contains(name, bad) {
			return errBranchUnsafe("contains forbidden sequence " + visibleEscape(bad))
		}
	}
	// Зарезервированные git ref'ы — отказ, даже если синтаксически валидны.
	switch strings.ToUpper(name) {
	case "HEAD", "FETCH_HEAD", "ORIG_HEAD", "MERGE_HEAD", "CHERRY_PICK_HEAD":
		return errBranchUnsafe("reserved git ref: " + name)
	}
	return nil
}

// errBranchUnsafe — обёртка с понятной причиной для логов (БЕЗ значения ветки,
// чтобы не утечь его содержимое в случае намеренной атаки).
func errBranchUnsafe(reason string) error {
	return &branchValidationError{Reason: reason}
}

type branchValidationError struct {
	Reason string
}

func (e *branchValidationError) Error() string {
	return "unsafe branch: " + e.Reason
}

func (e *branchValidationError) Is(target error) bool {
	return target == ErrBranchUnsafe
}

// visibleEscape делает escape-секвенции читаемыми в логах
// (\n / \r / \t / 0x00 / space) без утечки исходного контента.
func visibleEscape(s string) string {
	switch s {
	case "\n":
		return `\n`
	case "\r":
		return `\r`
	case "\t":
		return `\t`
	case "\x00":
		return `\x00`
	case " ":
		return `<space>`
	default:
		return s
	}
}
