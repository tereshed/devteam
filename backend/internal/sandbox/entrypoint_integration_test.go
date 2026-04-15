//go:build integration

package sandbox

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func dockerRm(t *testing.T, containerID string) {
	t.Helper()
	_ = exec.Command("docker", "rm", "-f", "--", containerID).Run()
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

// statusDoc — подмножество полей status.json из entrypoint.sh (достаточно для приёмочных проверок).
type statusDoc struct {
	Status        string `json:"status"`
	Success       bool   `json:"success"`
	ExitCode      int    `json:"exit_code"`
	Phase         string `json:"phase"`
	Cancelled     bool   `json:"cancelled"`
	Message       string `json:"message"`
	AgentExitCode *int   `json:"agent_exit_code"`
}

// runEntrypointAndCopyStatus создаёт контейнер, ждёт завершения, забирает status.json без bind-mount (UID sandbox).
// Возвращает JSON и код выхода процесса в контейнере (как у `docker start -a`).
// dockerCreateExtras — дополнительные аргументы для `docker create` до имени образа (например `-v` для подмены claude в тесте).
func runEntrypointAndCopyStatus(t *testing.T, image string, env []string, dockerCreateExtras ...string) ([]byte, int) {
	t.Helper()
	tmp := t.TempDir()
	statusHost := filepath.Join(tmp, "status.json")

	args := []string{"create"}
	args = append(args, dockerCreateExtras...)
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, "--", image)

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker create: %v out=%s", err, out)
	}
	cid := strings.TrimSpace(string(out))
	if cid == "" {
		t.Fatal("empty container id")
	}
	t.Cleanup(func() { dockerRm(t, cid) })

	_, err = exec.Command("docker", "start", "-a", "--", cid).CombinedOutput()
	exitCode := exitStatus(err)

	cpOut, err := exec.Command("docker", "cp", "--", cid+":"+StatusJSONPath, statusHost).CombinedOutput()
	if err != nil {
		t.Fatalf("docker cp status.json: %v out=%s", err, cpOut)
	}
	b, err := os.ReadFile(statusHost)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	return b, exitCode
}

func TestEntrypoint_MissingRepoURL_WritesStatusJSON(t *testing.T) {
	if !integrationDockerCLIAvailable(t) {
		t.Skip("docker CLI not in PATH")
	}
	img := integrationSandboxImageRef(t)
	if !integrationDockerImageExistsCLI(t, img) {
		t.Skipf("image %q missing; build: docker build -t %s -f deployment/sandbox/claude/Dockerfile deployment/sandbox/claude", img, img)
	}

	b, dockExit := runEntrypointAndCopyStatus(t, img, []string{
		"REPO_URL=",
		"BRANCH_NAME=test-branch",
		"TASK_INSTRUCTION=noop",
	})

	var st statusDoc
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("parse status.json: %v body=%s", err, b)
	}
	if dockExit != st.ExitCode {
		t.Fatalf("docker exit=%d vs status.exit_code=%d", dockExit, st.ExitCode)
	}
	if st.Status != "error" {
		t.Fatalf("status=%q want error", st.Status)
	}
	if st.Success {
		t.Fatal("success must be false")
	}
	if st.ExitCode == 0 {
		t.Fatalf("exit_code=%d want non-zero", st.ExitCode)
	}
	if st.Phase != "validation" {
		t.Fatalf("phase=%q want validation", st.Phase)
	}
	if st.Cancelled {
		t.Fatal("cancelled must be false")
	}
}

func TestEntrypoint_BranchNameLeadingHyphen_Validation(t *testing.T) {
	if !integrationDockerCLIAvailable(t) {
		t.Skip("docker CLI not in PATH")
	}
	img := integrationSandboxImageRef(t)
	if !integrationDockerImageExistsCLI(t, img) {
		t.Skipf("image %q missing", img)
	}

	b, dockExit := runEntrypointAndCopyStatus(t, img, []string{
		"REPO_URL=https://github.com/octocat/Hello-World.git",
		"BRANCH_NAME=-bad",
		"TASK_INSTRUCTION=noop",
		"BACKEND=claude-code",
		"ANTHROPIC_API_KEY=sk-ant-placeholder",
	})

	var st statusDoc
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("parse status.json: %v body=%s", err, b)
	}
	if dockExit != st.ExitCode {
		t.Fatalf("docker exit=%d vs status.exit_code=%d", dockExit, st.ExitCode)
	}
	if st.Phase != "validation" {
		t.Fatalf("phase=%q want validation (before clone)", st.Phase)
	}
	if st.Success || st.Status != "error" || st.ExitCode == 0 {
		t.Fatalf("expected validation error: success=%v status=%q exit=%d", st.Success, st.Status, st.ExitCode)
	}
}

func TestEntrypoint_MissingAnthropicAPIKey_Validation(t *testing.T) {
	if !integrationDockerCLIAvailable(t) {
		t.Skip("docker CLI not in PATH")
	}
	img := integrationSandboxImageRef(t)
	if !integrationDockerImageExistsCLI(t, img) {
		t.Skipf("image %q missing", img)
	}

	b, dockExit := runEntrypointAndCopyStatus(t, img, []string{
		"REPO_URL=https://github.com/octocat/Hello-World.git",
		"BRANCH_NAME=test-branch",
		"TASK_INSTRUCTION=noop",
		"BACKEND=claude-code",
		"ANTHROPIC_API_KEY=",
	})

	var st statusDoc
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("parse status.json: %v body=%s", err, b)
	}
	if dockExit != st.ExitCode {
		t.Fatalf("docker exit=%d vs status.exit_code=%d", dockExit, st.ExitCode)
	}
	if st.Phase != "validation" {
		t.Fatalf("phase=%q want validation (fast fail before clone)", st.Phase)
	}
	if st.Success || st.Status != "error" || st.ExitCode == 0 {
		t.Fatalf("expected validation error: success=%v status=%q exit=%d", st.Success, st.Status, st.ExitCode)
	}
}

func TestEntrypoint_UnsupportedBackend_WritesStatusJSON(t *testing.T) {
	if !integrationDockerCLIAvailable(t) {
		t.Skip("docker CLI not in PATH")
	}
	img := integrationSandboxImageRef(t)
	if !integrationDockerImageExistsCLI(t, img) {
		t.Skipf("image %q missing", img)
	}

	b, dockExit := runEntrypointAndCopyStatus(t, img, []string{
		"REPO_URL=https://example.com/repo.git",
		"BRANCH_NAME=test-branch",
		"TASK_INSTRUCTION=noop",
		"BACKEND=aider",
	})

	var st statusDoc
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if dockExit != st.ExitCode {
		t.Fatalf("docker exit=%d vs status.exit_code=%d", dockExit, st.ExitCode)
	}
	if st.Status != "error" || st.Phase != "validation" {
		t.Fatalf("status=%s phase=%s", st.Status, st.Phase)
	}
}

// TestEntrypoint_HappyPath_Success — контейнер завершается с ненулевым кодом (агент падает); runEntrypointAndCopyStatus
// не требует «успешного» docker start — после падения claude entrypoint собирает diff и финализирует с phase «done»
// (раньше в status.json оставалась фаза «agent»; см. deployment/sandbox/claude/entrypoint.sh).
// Реальный claude-code не детерминирован; подменяем /usr/local/bin/claude на testdata/fake-claude.sh (exit 42).
func TestEntrypoint_HappyPath_Success(t *testing.T) {
	if !integrationDockerCLIAvailable(t) {
		t.Skip("docker CLI not in PATH")
	}
	img := integrationSandboxImageRef(t)
	if !integrationDockerImageExistsCLI(t, img) {
		t.Skipf("image %q missing", img)
	}

	fakeClaude := filepath.Join(testdataDir(t), "fake-claude.sh")
	if err := os.Chmod(fakeClaude, 0o755); err != nil {
		t.Fatalf("chmod fake-claude: %v", err)
	}

	// Плейсхолдер ключа — нужен для fast-fail валидации до clone; реальный вызов идёт в fake-claude.sh.
	b, dockExit := runEntrypointAndCopyStatus(t, img, []string{
		"REPO_URL=https://github.com/octocat/Hello-World.git",
		"BRANCH_NAME=test-agent-branch",
		"BASE_REF=master",
		"TASK_INSTRUCTION=This prompt is ignored; fake claude exits immediately.",
		"BACKEND=claude-code",
		"ANTHROPIC_API_KEY=sk-ant-api03-ci-placeholder-not-used-with-fake-claude",
	}, "-v", fakeClaude+":/usr/local/bin/claude")

	var st statusDoc
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("parse: %v body=%s", err, b)
	}
	if dockExit != st.ExitCode {
		t.Fatalf("docker exit=%d vs status.exit_code=%d", dockExit, st.ExitCode)
	}

	if st.Phase == "validation" || st.Phase == "clone" || st.Phase == "branch" {
		t.Fatalf("failed too early in phase: %s, message: %s", st.Phase, st.Message)
	}
	if st.Cancelled {
		t.Fatal("cancelled must be false")
	}

	// Агент отработал (fake exit 42); финальная запись status.json — после diff, phase «done» (или «agent» на старых образах).
	if st.Phase != "done" && st.Phase != "agent" {
		t.Fatalf("Expected phase 'done' or 'agent', got %q. Message: %s", st.Phase, st.Message)
	}
	if st.Success {
		t.Fatal("Expected success=false when the agent CLI exits with an error (fake claude)")
	}
	if st.Status != "error" {
		t.Fatalf("Expected status='error', got %q", st.Status)
	}
	if st.ExitCode == 0 {
		t.Fatal("Expected non-zero exit code from the container")
	}
	if dockExit == 0 {
		t.Fatal("Expected non-zero docker exit code")
	}
	if st.AgentExitCode == nil || *st.AgentExitCode == 0 {
		t.Fatal("Expected non-zero agent_exit_code in status.json")
	}
	if *st.AgentExitCode != 42 {
		t.Fatalf("expected agent_exit_code 42 from testdata/fake-claude.sh, got %d", *st.AgentExitCode)
	}
}
