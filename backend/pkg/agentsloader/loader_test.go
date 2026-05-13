package agentsloader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateAgentConfigs(t *testing.T) {
	// agents is at backend/agents/, this file is at backend/pkg/agentsloader/
	// need to go up two directories: pkg -> backend -> agents
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "agents")
	promptsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	cache, err := NewCache(dir, promptsDir)
	if err != nil {
		t.Fatalf("failed to load agent cache: %v", err)
	}
	if err := cache.ValidateRequiredAgents(); err != nil {
		t.Fatalf("required agents validation failed: %v", err)
	}
	for _, name := range []string{"orchestrator", "planner", "developer", "reviewer", "tester"} {
		cfg, ok := cache.GetByName(name)
		if !ok {
			t.Errorf("agent %q not found", name)
			continue
		}
		if !cfg.IsActive {
			t.Errorf("agent %q is_active=false but should be required", name)
		}
		if cfg.ModelConfig.Temperature < 0 || cfg.ModelConfig.Temperature > 2.0 {
			t.Errorf("agent %q has invalid temperature: %f", name, cfg.ModelConfig.Temperature)
		}
	}
}

func TestPromptNameValidation(t *testing.T) {
	valid := []string{"orchestrator_prompt", "planner_prompt", "test123", "a_b", "x-y"}
	for _, name := range valid {
		if err := validatePromptName(name); err != nil {
			t.Errorf("expected %q to be valid, got: %v", name, err)
		}
	}
	invalid := []string{"../etc/passwd", "foo/bar", "foo.bar", "foo\\bar", "foo\x00bar"}
	for _, name := range invalid {
		if err := validatePromptName(name); err == nil {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestTemperatureBounds(t *testing.T) {
	valid := []float64{0.0, 0.1, 1.0, 1.5, 2.0}
	for _, temp := range valid {
		if err := validateTemperatureBounds(temp); err != nil {
			t.Errorf("expected temperature %f to be valid, got: %v", temp, err)
		}
	}
	invalid := []float64{-0.1, 2.1, -1.0, 3.0}
	for _, temp := range invalid {
		if err := validateTemperatureBounds(temp); err == nil {
			t.Errorf("expected temperature %f to be invalid", temp)
		}
	}
}

func TestNewCache_FileNotFound(t *testing.T) {
	_, err := NewCache("/nonexistent/path", "")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestCache_GetByPromptName(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "agents")
	promptsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	cache, err := NewCache(dir, promptsDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	cfg, ok := cache.GetByPromptName("orchestrator_prompt")
	if !ok {
		t.Error("expected to find orchestrator by prompt_name")
	}
	if cfg.Name != "orchestrator" {
		t.Errorf("expected name orchestrator, got: %s", cfg.Name)
	}
	_, ok = cache.GetByPromptName("nonexistent_prompt")
	if ok {
		t.Error("expected not to find nonexistent_prompt")
	}
}

func TestLoadSchema_Integration(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "agents")
	promptsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	schemaPath := filepath.Join(dir, "agent_schema.json")
	// loadSchema is called internally by NewCache, test the public interface
	_, err := NewCache(dir, promptsDir)
	if err != nil {
		t.Fatalf("failed to create cache from %s: %v", schemaPath, err)
	}
	// Verify schema file exists and is readable
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("schema file not readable: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("schema file is empty")
	}
}

func TestCache_AgentsList(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "agents")
	promptsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "prompts")
	cache, err := NewCache(dir, promptsDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	// Verify orchestrator
	cfg, ok := cache.GetByName("orchestrator")
	if !ok {
		t.Fatal("orchestrator not found")
	}
	if cfg.Role != "orchestrator" {
		t.Errorf("expected role orchestrator, got: %s", cfg.Role)
	}
	// Sprint 15.Major: assert не на конкретное имя модели (меняется по мере апгрейдов),
	// а на формат и non-empty — детали валидируются json-schema.
	if cfg.ModelConfig.Model == "" {
		t.Errorf("orchestrator model must be non-empty")
	}
	if cfg.ModelConfig.Temperature != 0.1 {
		t.Errorf("expected temperature 0.1, got: %f", cfg.ModelConfig.Temperature)
	}
	// Verify developer
	cfg, ok = cache.GetByName("developer")
	if !ok {
		t.Fatal("developer not found")
	}
	if cfg.ModelConfig.Temperature != 0.2 {
		t.Errorf("expected developer temperature 0.2, got: %f", cfg.ModelConfig.Temperature)
	}
}

var _ = fmt.Sprintf // for go vet
