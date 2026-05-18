package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/devteam/backend/pkg/llm"
)

// Anthropic Messages API:
//   - role: "user" | "assistant"  (НЕ принимает "tool"/"system"; system — top-level)
//   - tool_result — content-block внутри user-сообщения с tool_use_id
//   - tool_use    — content-block внутри assistant-сообщения с id+name+input
//   - подряд идущие same-role messages запрещены
//
// Тесты ниже фиксируют именно эти инварианты — регресс на role:"tool" уже
// был и стоил продакшну 400/invalid_request_error.

func TestMapRequest_RoleTool_BecomesUserWithToolResult(t *testing.T) {
	c := &Client{}
	out := c.mapRequest(llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "посмотри проекты"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: llm.Function{Name: "project_list", Arguments: `{"limit":10}`},
			}}},
			{Role: llm.RoleTool, ToolCallID: "call_1", Name: "project_list",
				Content: `{"items":[]}`},
		},
	})

	if got := len(out.Messages); got != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", got, out.Messages)
	}
	for i, m := range out.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			t.Fatalf("messages[%d].role=%q — Anthropic accepts only user/assistant", i, m.Role)
		}
	}
	// Третье сообщение (бывший RoleTool) → role:"user" + tool_result-block.
	last := out.Messages[2]
	if last.Role != "user" {
		t.Fatalf("tool-message must be remapped to role=user, got %q", last.Role)
	}
	if len(last.Content) != 1 || last.Content[0].Type != "tool_result" {
		t.Fatalf("expected single tool_result content block, got %+v", last.Content)
	}
	if last.Content[0].ToolUseID != "call_1" {
		t.Fatalf("tool_use_id mismatch: %q", last.Content[0].ToolUseID)
	}
	if last.Content[0].Content != `{"items":[]}` {
		t.Fatalf("tool_result.content mismatch: %q", last.Content[0].Content)
	}
}

func TestMapRequest_MultipleToolResults_MergedIntoSingleUserTurn(t *testing.T) {
	// LLM в одном assistant-turn'е заказал 2 tool_use'а → executor отдаёт
	// 2 подряд RoleTool. Без merge получится 2 подряд user-сообщения, что
	// Anthropic отклоняет ("messages: at most one user/assistant per turn"
	// в одних версиях, "consecutive roles must alternate" в других).
	c := &Client{}
	out := c.mapRequest(llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "сделай две проверки"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
				{ID: "c1", Type: "function", Function: llm.Function{Name: "a", Arguments: `{}`}},
				{ID: "c2", Type: "function", Function: llm.Function{Name: "b", Arguments: `{}`}},
			}},
			{Role: llm.RoleTool, ToolCallID: "c1", Content: `{"ok":true}`},
			{Role: llm.RoleTool, ToolCallID: "c2", Content: `{"ok":false}`},
		},
	})

	// Ожидаем чередование user/assistant/user (3 сообщения, последнее — слитное).
	if got := len(out.Messages); got != 3 {
		t.Fatalf("expected merged into 3 messages, got %d: %+v", got, out.Messages)
	}
	got := []string{out.Messages[0].Role, out.Messages[1].Role, out.Messages[2].Role}
	want := []string{"user", "assistant", "user"}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("messages[%d].role=%q want %q", i, got[i], want[i])
		}
	}
	merged := out.Messages[2].Content
	if len(merged) != 2 {
		t.Fatalf("expected 2 tool_result blocks in merged user msg, got %d", len(merged))
	}
	if merged[0].Type != "tool_result" || merged[0].ToolUseID != "c1" {
		t.Fatalf("first block: %+v", merged[0])
	}
	if merged[1].Type != "tool_result" || merged[1].ToolUseID != "c2" {
		t.Fatalf("second block: %+v", merged[1])
	}
}

func TestMapRequest_SystemSkipped_GoesToTopLevel(t *testing.T) {
	c := &Client{}
	out := c.mapRequest(llm.Request{
		SystemPrompt: "ты — ассистент",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "ignored-extra-system"},
			{Role: llm.RoleUser, Content: "hi"},
		},
	})
	for _, m := range out.Messages {
		if m.Role == "system" {
			t.Fatalf("system role must NOT appear in messages[]; got %+v", out.Messages)
		}
	}
	if out.System != "ты — ассистент" {
		t.Fatalf("system prompt must be top-level field, got %q", out.System)
	}
	if len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Fatalf("expected single user message, got %+v", out.Messages)
	}
}

func TestMapRequest_AssistantToolUse_EmitsToolUseBlock(t *testing.T) {
	c := &Client{}
	out := c.mapRequest(llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "go"},
			{
				Role:    llm.RoleAssistant,
				Content: "сейчас вызову tool",
				ToolCalls: []llm.ToolCall{{
					ID:       "call_1",
					Type:     "function",
					Function: llm.Function{Name: "project_list", Arguments: `{"limit":5}`},
				}},
			},
		},
	})
	if len(out.Messages) != 2 || out.Messages[1].Role != "assistant" {
		t.Fatalf("expected assistant turn at index 1, got %+v", out.Messages)
	}
	blocks := out.Messages[1].Content
	if len(blocks) != 2 {
		t.Fatalf("expected text + tool_use blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Type != "text" || blocks[0].Text != "сейчас вызову tool" {
		t.Fatalf("first block: %+v", blocks[0])
	}
	if blocks[1].Type != "tool_use" || blocks[1].ID != "call_1" ||
		blocks[1].Name != "project_list" ||
		string(blocks[1].Input) != `{"limit":5}` {
		t.Fatalf("tool_use block: %+v", blocks[1])
	}
}

// На WIRE-уровне (JSON, который реально летит в API) проверим, что
// поля называются именно так, как ждёт Anthropic.
func TestMapRequest_WireJSON_FieldNamesMatchAPI(t *testing.T) {
	c := &Client{}
	out := c.mapRequest(llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "?"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
				ID: "id1", Type: "function",
				Function: llm.Function{Name: "n", Arguments: `{"x":1}`},
			}}},
			{Role: llm.RoleTool, ToolCallID: "id1", Content: `{"ok":true}`},
		},
	})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, s := range []string{
		`"role":"user"`,
		`"role":"assistant"`,
		`"type":"tool_use"`,
		`"type":"tool_result"`,
		`"tool_use_id":"id1"`,
	} {
		if !contains(body, s) {
			t.Fatalf("wire JSON missing %q\nfull body: %s", s, body)
		}
	}
	for _, s := range []string{
		`"role":"tool"`,
		`"role":"system"`,
	} {
		if contains(body, s) {
			t.Fatalf("wire JSON unexpectedly contains %q\nfull body: %s", s, body)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
