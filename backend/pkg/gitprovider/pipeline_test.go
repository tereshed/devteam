package gitprovider

import "testing"

func TestMapGitlabPipelineStatus(t *testing.T) {
	cases := map[string]PipelineStatus{
		"success":  PipelineStatusSuccess,
		"failed":   PipelineStatusFailed,
		"canceled": PipelineStatusCanceled,
		"skipped":  PipelineStatusSkipped,
		"running":  PipelineStatusPending,
		"pending":  PipelineStatusPending,
		"created":  PipelineStatusPending,
		"manual":   PipelineStatusPending,
		"weird":    PipelineStatusPending,
	}
	for in, want := range cases {
		if got := mapGitlabPipelineStatus(in); got != want {
			t.Errorf("mapGitlabPipelineStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPipelineStatus_IsTerminal(t *testing.T) {
	terminal := []PipelineStatus{PipelineStatusSuccess, PipelineStatusFailed, PipelineStatusCanceled, PipelineStatusSkipped, PipelineStatusNone}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%q should be terminal", s)
		}
	}
	if PipelineStatusPending.IsTerminal() {
		t.Error("pending must not be terminal")
	}
}
