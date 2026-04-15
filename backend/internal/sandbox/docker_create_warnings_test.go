package sandbox

import "testing"

func Test_isDockerSwapLimitKernelUnsupportedWarning(t *testing.T) {
	positive := []string{
		"Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.",
		"WARNING: does not support swap limit capabilities",
	}
	for _, w := range positive {
		if !isDockerSwapLimitKernelUnsupportedWarning(w) {
			t.Fatalf("expected true: %q", w)
		}
	}
	if isDockerSwapLimitKernelUnsupportedWarning("Your kernel does not support memory limit capabilities") {
		t.Fatal("unexpected match for unrelated warning")
	}
}

func Test_isDockerCreateWarningFatal_emptyAllowlist(t *testing.T) {
	if isDockerCreateWarningFatal("anything") {
		t.Fatal("fatal allowlist is empty by default")
	}
}
