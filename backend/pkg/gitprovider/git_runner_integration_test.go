//go:build integration

package gitprovider

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestExecGitRunner_RunGit_version(t *testing.T) {
	t.Parallel()
	r := NewExecGitRunner()
	out, stderr, err := r.RunGit(context.Background(), t.TempDir(), "version")
	if err != nil {
		t.Fatalf("%v stderr=%q", err, stderr)
	}
	if !strings.Contains(out, "git version") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestExecGitRunner_GitStdoutPipe_partialReadAndClose(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	steps := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "a@b.c"},
		{"git", "config", "user.name", "n"},
		{"git", "commit", "--allow-empty", "-m", "a"},
		{"git", "commit", "--allow-empty", "-m", "b"},
	}
	for _, argv := range steps {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	r := NewExecGitRunner()
	rc, err := r.GitStdoutPipe(context.Background(), dir, "", "diff", "HEAD~1..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	_, _ = rc.Read(buf)
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
}
