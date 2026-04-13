package sandbox

import "testing"

func TestSandboxStatus_HasResult(t *testing.T) {
	if (*SandboxStatus)(nil).HasResult() {
		t.Fatal("nil receiver")
	}
	s := &SandboxStatus{Result: nil}
	if s.HasResult() {
		t.Fatal("nil Result")
	}
	s.Result = &CodeResult{}
	if !s.HasResult() {
		t.Fatal("want true")
	}
}
