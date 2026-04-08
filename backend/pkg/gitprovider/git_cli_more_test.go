package gitprovider

import (
	"context"
	"errors"
	"testing"
)

func TestLocalGitCLI_ListLocalBranches_and_DeleteLocalBranch(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{defaultOut: "main\nfeature\n"}
	cli := LocalGitCLI{creds: Credentials{}, runner: rr}
	ctx := context.Background()
	names, err := cli.ListLocalBranches(ctx, "/w", "feat")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "feature" {
		t.Fatalf("%v", names)
	}
	rr2 := &recordingRunner{}
	rr2.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "", nil
	}
	cli2 := LocalGitCLI{creds: Credentials{}, runner: rr2}
	if err := cli2.DeleteLocalBranch(ctx, "/w", "topic"); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubProvider_CommitAndPush(t *testing.T) {
	t.Parallel()
	n := 0
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		n++
		switch args[0] {
		case "add":
			return "", "", nil
		case "rev-parse":
			if len(args) >= 2 && args[1] == "--verify" {
				return "", "fatal: not a git", errors.New("exit 128")
			}
			return "deadbeef", "", nil
		case "status":
			return "M x\n", "", nil
		case "commit":
			return "", "", nil
		case "remote":
			return "https://github.com/o/r.git", "", nil
		case "push":
			return "", "", nil
		}
		return "", "", errors.New("unexpected argv")
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: "t"}, nil, rr)
	sha, changed, err := p.CommitAndPush(context.Background(), "/w", CommitOptions{Message: "m"}, PushOptions{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || sha != "deadbeef" {
		t.Fatalf("%q %v", sha, changed)
	}
	if n < 5 {
		t.Fatalf("calls %d", n)
	}
}
