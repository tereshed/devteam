package agentprompts

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateAllYAMLAgainstSchema(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	if err := ValidateAllYAMLAgainstSchema(dir); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestNewComposer(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	c, err := NewComposer(dir)
	if err != nil {
		t.Fatal(err)
	}
	sys, err := c.ComposeSystem("planner")
	if err != nil {
		t.Fatal(err)
	}
	if len(sys) < 200 {
		t.Fatalf("merged system prompt unexpectedly short: %d", len(sys))
	}
	ut, err := c.UserTemplate("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if ut == "" {
		t.Fatal("expected non-empty user template for reviewer")
	}
}
