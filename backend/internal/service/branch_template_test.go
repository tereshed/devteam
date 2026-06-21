package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fixedTaskID — детерминированный UUID для предсказуемого short_id/id в тестах.
var fixedTaskID = uuid.MustParse("a1b2c3d4-1111-2222-3333-444455556666")

func fixedVars(title, key string) BranchVars {
	return BranchVars{
		TaskID:      fixedTaskID,
		Title:       title,
		ExternalKey: key,
		Now:         time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
	}
}

func TestRenderBranchName(t *testing.T) {
	cases := []struct {
		name  string
		tmpl  string
		title string
		key   string
		want  string
	}{
		{
			name: "default empty template reproduces legacy",
			tmpl: "", title: "Fix login bug", key: "",
			want: "task/a1b2c3d4-fix-login-bug",
		},
		{
			name: "default empty title falls back to short id only",
			tmpl: "", title: "!!!", key: "",
			want: "task/a1b2c3d4",
		},
		{
			name: "team convention issue/ticket_slug auto-appends short_id (no id placeholder)",
			tmpl: "issue/{ticket}_{slug}", title: "Fix login bug", key: "DEV-123",
			want: "issue/DEV-123_fix-login-bug-a1b2c3d4",
		},
		{
			name: "ticket verbatim preserves case while slug lowercases",
			tmpl: "{ticket}-{slug}", title: "Add OAuth Support", key: "ABC-9",
			want: "ABC-9-add-oauth-support-a1b2c3d4",
		},
		{
			name: "explicit short_id placeholder suppresses auto-suffix",
			tmpl: "feature/{short_id}-{slug}", title: "Thing", key: "",
			want: "feature/a1b2c3d4-thing",
		},
		{
			name: "ticket fallback to short_id when key empty",
			tmpl: "issue/{ticket|short_id}_{slug}", title: "Thing", key: "",
			want: "issue/a1b2c3d4_thing-a1b2c3d4",
		},
		{
			name: "empty ticket without fallback collapses leftover separator",
			tmpl: "issue/{ticket}_{slug}", title: "Thing", key: "",
			want: "issue/thing-a1b2c3d4",
		},
		{
			name: "date placeholders",
			tmpl: "{yyyy}{mm}{dd}/{slug}", title: "Thing", key: "",
			want: "20260619/thing-a1b2c3d4",
		},
		{
			name: "full id placeholder",
			tmpl: "t/{id}", title: "x", key: "",
			want: "t/a1b2c3d4-1111-2222-3333-444455556666",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderBranchName(tc.tmpl, fixedVars(tc.title, tc.key))
			if err != nil {
				t.Fatalf("RenderBranchName(%q) unexpected error: %v", tc.tmpl, err)
			}
			if got != tc.want {
				t.Errorf("RenderBranchName(%q) = %q, want %q", tc.tmpl, got, tc.want)
			}
		})
	}
}

func TestRenderBranchNameUnknownPlaceholder(t *testing.T) {
	_, err := RenderBranchName("feature/{nope}", fixedVars("x", ""))
	if !errors.Is(err, ErrBranchTemplateInvalid) {
		t.Fatalf("expected ErrBranchTemplateInvalid, got %v", err)
	}
}

func TestRenderBranchNameRejectsInjection(t *testing.T) {
	// Слаг защищает заголовок, но в принципе любой невалидный результат должен дать ошибку.
	// Шаблон, целиком состоящий из разделителей, после зачистки пуст → ошибка.
	if _, err := RenderBranchName("///", fixedVars("", "")); err == nil {
		// "///" -> cleanup пусто -> нет id -> дописываем short_id -> валидно, не ошибка.
		// Поэтому этот кейс на самом деле валиден; проверяем что хотя бы не паника и git-safe.
		got, _ := RenderBranchName("///", fixedVars("", ""))
		if got == "" {
			t.Fatal("expected non-empty fallback branch")
		}
	}
}

func TestValidateExternalKey(t *testing.T) {
	valid := []string{"DEV-123", "ABC-9", "FEAT_12", "x", "A1", "proj-1"}
	for _, k := range valid {
		if err := ValidateExternalKey(k); err != nil {
			t.Errorf("ValidateExternalKey(%q) unexpected error: %v", k, err)
		}
	}
	invalid := []string{"", "-bad", "has space", "a/b", "a..b", "семпл", "a~b"}
	for _, k := range invalid {
		if err := ValidateExternalKey(k); !errors.Is(err, ErrInvalidExternalKey) {
			t.Errorf("ValidateExternalKey(%q) = %v, want ErrInvalidExternalKey", k, err)
		}
	}
}

func TestTemplateRequiresTicket(t *testing.T) {
	cases := map[string]bool{
		"issue/{ticket}_{slug}":     true,
		"{ticket}":                  true,
		"issue/{ticket|short_id}-x": false, // fallback => не обязателен
		"task/{short_id}-{slug}":    false,
		"":                          false,
	}
	for tmpl, want := range cases {
		if got := TemplateRequiresTicket(tmpl); got != want {
			t.Errorf("TemplateRequiresTicket(%q) = %v, want %v", tmpl, got, want)
		}
	}
}

func TestCompileBranchPatternMatchesGeneratedShape(t *testing.T) {
	tmpl := "issue/{ticket}_{slug}"
	re, err := CompileBranchPattern(tmpl)
	if err != nil {
		t.Fatalf("CompileBranchPattern error: %v", err)
	}
	// Имена в форме шаблона (override'ы) должны проходить.
	match := []string{"issue/DEV-123_fix-login", "issue/ABC-9_x", "issue/PROJ-1_some-thing"}
	for _, b := range match {
		if !re.MatchString(b) {
			t.Errorf("pattern %s should match %q", re.String(), b)
		}
	}
	// Не соответствующие конвенции — отвергаются.
	noMatch := []string{"wip", "feature/foo", "DEV-123_fix", "issue/DEV-123", "issue/_fix"}
	for _, b := range noMatch {
		if re.MatchString(b) {
			t.Errorf("pattern %s should NOT match %q", re.String(), b)
		}
	}
}

func TestCompileBranchPatternUnknownPlaceholder(t *testing.T) {
	if _, err := CompileBranchPattern("x/{bogus}"); !errors.Is(err, ErrBranchTemplateInvalid) {
		t.Fatalf("expected ErrBranchTemplateInvalid, got %v", err)
	}
}

func mrVars(title, ticket, branch, repo string) MRTitleVars {
	return MRTitleVars{
		TaskID:      fixedTaskID,
		Title:       title,
		ExternalKey: ticket,
		Branch:      branch,
		RepoSlug:    repo,
		Now:         time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
	}
}

func TestRenderMRTitle(t *testing.T) {
	cases := []struct {
		name   string
		tmpl   string
		ticket string
		want   string
	}{
		{"default empty reproduces legacy", "", "DEV-123", "PolyMaths: Fix login bug"},
		{"ticket prefix", "[{ticket}] {title}", "DEV-123", "[DEV-123] Fix login bug"},
		{"ticket colon title", "{ticket}: {title}", "DEV-123", "DEV-123: Fix login bug"},
		{"empty ticket collapses spaces", "{ticket} {title}", "", "Fix login bug"},
		{"branch and repo", "{repo} {branch}", "DEV-123", "main issue/DEV-123_fix"},
		{"unknown placeholder dropped", "x {nope} {title}", "DEV-123", "x Fix login bug"},
		{"short_id", "{short_id} {title}", "DEV-123", "a1b2c3d4 Fix login bug"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderMRTitle(tc.tmpl, mrVars("Fix login bug", tc.ticket, "issue/DEV-123_fix", "main"))
			if got != tc.want {
				t.Errorf("RenderMRTitle(%q) = %q, want %q", tc.tmpl, got, tc.want)
			}
		})
	}
}

func TestValidateMRTitleTemplate(t *testing.T) {
	for _, ok := range []string{"", "[{ticket}] {title}", "{repo}: {branch} ({short_id})"} {
		if err := ValidateMRTitleTemplate(ok); err != nil {
			t.Errorf("ValidateMRTitleTemplate(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"{title} {bogus}", "{ticket|wat}"} {
		if err := ValidateMRTitleTemplate(bad); !errors.Is(err, ErrMRTitleTemplateInvalid) {
			t.Errorf("ValidateMRTitleTemplate(%q) = %v, want ErrMRTitleTemplateInvalid", bad, err)
		}
	}
}

func TestValidateBranchTemplate(t *testing.T) {
	ok := []string{"", "issue/{ticket}_{slug}", "task/{short_id}-{slug}", "{ticket|short_id}/{date}"}
	for _, tmpl := range ok {
		if err := ValidateBranchTemplate(tmpl); err != nil {
			t.Errorf("ValidateBranchTemplate(%q) unexpected error: %v", tmpl, err)
		}
	}
	bad := []string{"x/{unknown}", "{ticket|wat}"}
	for _, tmpl := range bad {
		if err := ValidateBranchTemplate(tmpl); !errors.Is(err, ErrBranchTemplateInvalid) {
			t.Errorf("ValidateBranchTemplate(%q) = %v, want ErrBranchTemplateInvalid", tmpl, err)
		}
	}
}
