package openai

import (
	"encoding/json"
	"testing"

	"github.com/devteam/backend/pkg/llm"
)

// Контракт OpenAI Chat Completions API:
//   - role: "system" | "user" | "assistant" | "tool"
//   - tool-результат: {role:"tool", tool_call_id:"...", content:"..."}
//   - tool-вызов:    {role:"assistant", tool_calls:[{id, type:"function", function:{name,arguments}}]}
//
// Тот же формат принимают: deepseek, qwen, openrouter, moonshot, zhipu, oaicompat,
// ollama (через openai-совместимый шим). Поэтому регресс-сеть здесь страхует
// всю семью openai-style провайдеров — если кто-то поломает RoleTool→role:"tool",
// тест свистит.

func TestMapRequest_RoleTool_RemainsToolWithCallID(t *testing.T) {
	c := &Client{}
	out := c.mapRequest(llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "list"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
				ID: "call_1", Type: "function",
				Function: llm.Function{Name: "project_list", Arguments: `{"limit":10}`},
			}}},
			{Role: llm.RoleTool, ToolCallID: "call_1",
				Content: `{"items":[]}`},
		},
	})

	if got := len(out.Messages); got != 3 {
		t.Fatalf("expected 3 messages, got %d", got)
	}
	tr := out.Messages[2]
	if tr.Role != "tool" {
		t.Fatalf("tool-message must keep role=tool for OpenAI-family, got %q", tr.Role)
	}
	if tr.ToolCallID != "call_1" {
		t.Fatalf("tool_call_id mismatch: %q", tr.ToolCallID)
	}
	if tr.Content != `{"items":[]}` {
		t.Fatalf("tool content mismatch: %q", tr.Content)
	}

	// На WIRE-уровне: имя поля — именно tool_call_id (snake_case API).
	raw, _ := json.Marshal(out)
	body := string(raw)
	for _, want := range []string{
		`"role":"tool"`,
		`"tool_call_id":"call_1"`,
		`"role":"assistant"`,
		`"tool_calls":`,
	} {
		if !contains(body, want) {
			t.Fatalf("wire JSON missing %q\nfull body: %s", want, body)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
