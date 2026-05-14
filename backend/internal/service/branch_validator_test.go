package service

import (
	"errors"
	"strings"
	"testing"
)

// TestValidateBaseBranch_AcceptsValid — нормальные имена веток должны пропускаться.
func TestValidateBaseBranch_AcceptsValid(t *testing.T) {
	valid := []string{
		"main",
		"master",
		"develop",
		"release/1.2.3",
		"feature/add-auth",
		"feat_foo",
		"hotfix-2026-01",
		"v1.0.0",
		"a", // минимальная длина
		"users/john.doe/feature-x",
		strings.Repeat("a", 128), // максимальная длина
	}
	for _, name := range valid {
		if err := ValidateBaseBranch(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}
}

// TestValidateBaseBranch_RejectsFlagInjection — главный adversarial-кейс:
// имена, выдающие себя за git-флаги. Это причина существования всего этого валидатора
// плюс `--` separator в exec.Command.
func TestValidateBaseBranch_RejectsFlagInjection(t *testing.T) {
	adversarial := []struct {
		name   string
		reason string
	}{
		{"-h", "ведущий -, подходит под git -h"},
		{"--help", "ведущий -, длинный флаг"},
		{"--upload-pack=evil.sh", "флаг с присваиванием — может выполнить произвольный pack-protocol handler"},
		{"-uevil.sh", "короткий флаг -u с параметром"},
		{"--exec=rm -rf /", "вариант --exec из CVE-stylе"},
		{"-fbranch", "флаг -f"},
	}
	for _, c := range adversarial {
		err := ValidateBaseBranch(c.name)
		if err == nil {
			t.Errorf("expected REJECT for adversarial %q (%s), got nil", c.name, c.reason)
			continue
		}
		if !errors.Is(err, ErrBranchUnsafe) {
			t.Errorf("expected ErrBranchUnsafe sentinel for %q, got: %v", c.name, err)
		}
	}
}

// TestValidateBaseBranch_RejectsPathTraversal — относительные пути не допускаются,
// чтобы git не интерпретировал имя как путь.
func TestValidateBaseBranch_RejectsPathTraversal(t *testing.T) {
	for _, name := range []string{
		"../etc/passwd",
		"../../etc/passwd",
		"feature/../../../etc",
		"./relative",
		"..",
	} {
		if err := ValidateBaseBranch(name); err == nil {
			t.Errorf("expected REJECT for path-traversal %q", name)
		}
	}
}

// TestValidateBaseBranch_RejectsControlChars — нулевые байты, переводы строк,
// табы — частые векторы инъекции в shell-парсерах.
func TestValidateBaseBranch_RejectsControlChars(t *testing.T) {
	for _, name := range []string{
		"main\x00malicious",
		"main\nrm -rf /",
		"main\rcmd",
		"main\tinjected",
		"main with space",
	} {
		if err := ValidateBaseBranch(name); err == nil {
			t.Errorf("expected REJECT for control-char %q", name)
		}
	}
}

// TestValidateBaseBranch_RejectsEmptyOrTooLong — границы.
func TestValidateBaseBranch_RejectsEmptyOrTooLong(t *testing.T) {
	if err := ValidateBaseBranch(""); err == nil {
		t.Error("expected REJECT for empty branch")
	}
	if err := ValidateBaseBranch(strings.Repeat("a", 129)); err == nil {
		t.Error("expected REJECT for >128 chars")
	}
}

// TestValidateBaseBranch_RejectsReservedRefs — HEAD/FETCH_HEAD и т.д. — special git ref'ы,
// которые могут привести к неожиданному поведению.
func TestValidateBaseBranch_RejectsReservedRefs(t *testing.T) {
	for _, name := range []string{
		"HEAD", "head", "Head",
		"FETCH_HEAD", "ORIG_HEAD",
		"MERGE_HEAD", "CHERRY_PICK_HEAD",
	} {
		if err := ValidateBaseBranch(name); err == nil {
			t.Errorf("expected REJECT for reserved ref %q", name)
		}
	}
}

// TestValidateBaseBranch_RejectsReflogSyntax — branch@{...} может означать reflog
// или другие специальные значения, которые git понимает как ссылки во времени.
func TestValidateBaseBranch_RejectsReflogSyntax(t *testing.T) {
	for _, name := range []string{
		"main@{yesterday}",
		"master@{0}",
		"feature@{1.day.ago}",
	} {
		if err := ValidateBaseBranch(name); err == nil {
			t.Errorf("expected REJECT for reflog-syntax %q", name)
		}
	}
}

// TestValidateBaseBranch_DoubleSlash — двойной слеш отвергаем (нестандартный для git).
func TestValidateBaseBranch_DoubleSlash(t *testing.T) {
	if err := ValidateBaseBranch("feat//foo"); err == nil {
		t.Error("expected REJECT for double-slash")
	}
}

// TestErrBranchUnsafe_DoesNotLeakName — текст ошибки НЕ должен содержать само
// небезопасное имя (защита от случайного логирования вредоносной строки).
func TestErrBranchUnsafe_DoesNotLeakName(t *testing.T) {
	adversarial := "--upload-pack=evil.sh"
	err := ValidateBaseBranch(adversarial)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), adversarial) {
		t.Errorf("error message leaks adversarial branch name: %q", err.Error())
	}
}
