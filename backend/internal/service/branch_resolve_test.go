package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

func newBranchTestService() *taskService {
	return &taskService{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func branchProject(tmpl string, locked bool, pattern string) *models.Project {
	p := &models.Project{ID: uuid.New(), BranchNamingLocked: locked}
	if tmpl != "" {
		t := tmpl
		p.BranchNameTemplate = &t
	}
	if pattern != "" {
		pt := pattern
		p.BranchNamePattern = &pt
	}
	return p
}

func TestResolveBranchName(t *testing.T) {
	s := newBranchTestService()
	ctx := context.Background()
	tid := fixedTaskID

	t.Run("template requires ticket, key missing -> error", func(t *testing.T) {
		p := branchProject("issue/{ticket}_{slug}", false, "")
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "Fix bug"}, "")
		if !errors.Is(err, ErrExternalKeyRequired) {
			t.Fatalf("want ErrExternalKeyRequired, got %v", err)
		}
	})

	t.Run("template requires ticket, key present -> generated", func(t *testing.T) {
		p := branchProject("issue/{ticket}_{slug}", false, "")
		got, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "Fix bug"}, "DEV-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "issue/DEV-1_fix-bug-a1b2c3d4" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("locked project rejects manual override", func(t *testing.T) {
		p := branchProject("task/{short_id}-{slug}", true, "")
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("my-branch")}, "")
		if !errors.Is(err, ErrBranchNamingLocked) {
			t.Fatalf("want ErrBranchNamingLocked, got %v", err)
		}
	})

	t.Run("override violating derived pattern is rejected", func(t *testing.T) {
		// key present (ticket requirement satisfied) so we isolate the pattern check
		p := branchProject("issue/{ticket}_{slug}", false, "")
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("wip")}, "DEV-1")
		if !errors.Is(err, ErrBranchPatternMismatch) {
			t.Fatalf("want ErrBranchPatternMismatch, got %v", err)
		}
	})

	t.Run("override matching derived pattern is accepted", func(t *testing.T) {
		p := branchProject("issue/{ticket}_{slug}", false, "")
		got, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("issue/DEV-9_some-thing")}, "DEV-9")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "issue/DEV-9_some-thing" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ticket requirement is absolute even with manual override", func(t *testing.T) {
		p := branchProject("issue/{ticket}_{slug}", false, "")
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("issue/DEV-9_some-thing")}, "")
		if !errors.Is(err, ErrExternalKeyRequired) {
			t.Fatalf("want ErrExternalKeyRequired, got %v", err)
		}
	})

	t.Run("unsafe override is rejected by git floor", func(t *testing.T) {
		p := branchProject("", false, "")
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("bad branch")}, "")
		if !errors.Is(err, ErrTaskInvalidBranch) {
			t.Fatalf("want ErrTaskInvalidBranch, got %v", err)
		}
	})

	t.Run("no template, no override -> default", func(t *testing.T) {
		p := branchProject("", false, "")
		got, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "Hello World"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "task/a1b2c3d4-hello-world" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("explicit pattern overrides derived", func(t *testing.T) {
		p := branchProject("task/{short_id}-{slug}", false, `^custom/.+$`)
		// matches template shape but NOT explicit pattern -> rejected
		_, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("task/a1b2c3d4-x")}, "")
		if !errors.Is(err, ErrBranchPatternMismatch) {
			t.Fatalf("want ErrBranchPatternMismatch, got %v", err)
		}
		// matches explicit pattern -> accepted
		got, err := s.resolveBranchName(ctx, p, tid, dto.CreateTaskRequest{Title: "x", BranchName: strPtr("custom/foo")}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "custom/foo" {
			t.Errorf("got %q", got)
		}
	})
}
