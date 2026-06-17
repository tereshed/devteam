package service

import (
	"strings"
	"testing"

	"github.com/devteam/backend/pkg/gitprovider"
)

func TestBuildCIFailureReason(t *testing.T) {
	t.Run("with failed jobs and trace", func(t *testing.T) {
		res := &gitprovider.PipelineResult{
			Status: gitprovider.PipelineStatusFailed,
			WebURL: "https://gl/pipelines/3935",
			FailedJobs: []gitprovider.PipelineFailedJob{
				{Name: "test_backend", Stage: "test", WebURL: "https://gl/jobs/1", LogTail: "FAILED app/tests/test_x.py::test_y"},
			},
		}
		reason := buildCIFailureReason(ciGateTarget{slug: "self-service", branch: "task/abc", prURL: "https://gl/mr/282"}, res)
		for _, want := range []string{"CI-пайплайн упал", "self-service", "task/abc", "https://gl/mr/282", "https://gl/pipelines/3935", "test_backend", "[test]", "FAILED app/tests"} {
			if !strings.Contains(reason, want) {
				t.Errorf("reason missing %q\n--- reason ---\n%s", want, reason)
			}
		}
	})

	t.Run("no job details", func(t *testing.T) {
		res := &gitprovider.PipelineResult{Status: gitprovider.PipelineStatusFailed, WebURL: "https://gl/p/1"}
		reason := buildCIFailureReason(ciGateTarget{slug: "main", branch: "b"}, res)
		if !strings.Contains(reason, "детали недоступны") {
			t.Errorf("expected fallback message, got: %s", reason)
		}
	})

	t.Run("trace tail truncated", func(t *testing.T) {
		long := strings.Repeat("x", ciReasonTraceTailBytes+500)
		res := &gitprovider.PipelineResult{
			Status:     gitprovider.PipelineStatusFailed,
			FailedJobs: []gitprovider.PipelineFailedJob{{Name: "j", Stage: "s", LogTail: long}},
		}
		reason := buildCIFailureReason(ciGateTarget{slug: "r", branch: "b"}, res)
		if !strings.Contains(reason, "truncated") {
			t.Errorf("expected truncation marker for long trace")
		}
		if strings.Count(reason, "x") > ciReasonTraceTailBytes+50 {
			t.Errorf("trace tail not truncated")
		}
	})
}
