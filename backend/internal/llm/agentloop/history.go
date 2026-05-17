package agentloop

import (
	"encoding/json"
	"fmt"

	"github.com/devteam/backend/pkg/llm"
)

// history.go — конвертация Message-истории в llm.Message с защитой от
// переполнения контекста (Sprint 21 §3.4).
//
// Два уровня защиты:
//  1. Per-tool truncation — каждый tool_result, превышающий
//     Config.MaxToolResultBytes, заменяется на marker preview с подсказкой
//     модели использовать пагинацию/фильтры. Полный payload остаётся в БД
//     (для UI и аудита) — здесь мы трогаем только то, что идёт в LLM.
//  2. Sliding-window compaction — если суммарная история превышает
//     Config.MaxHistoryBytes, самые старые tool_result-сообщения
//     сжимаются до коротких summary; user/assistant из «хвоста»
//     (Config.HistoryTailKeep) всегда остаются полными.
//
// Контракт: входной []Message — это полная БД-история (как из repo
// ListMessages, потом перевёрнутая в хронологический порядок).
// На выходе — []llm.Message, готовые для llm.Request.Messages.

// buildLLMMessages применяет truncation + sliding-window и собирает
// нативные сообщения для провайдера.
func buildLLMMessages(history []Message, cfg Config) []llm.Message {
	compacted := slidingWindowCompact(history, cfg)
	out := make([]llm.Message, 0, len(compacted))
	for _, m := range compacted {
		out = append(out, toLLMMessage(m, cfg))
	}
	return out
}

// toLLMMessage конвертирует один Message в llm.Message с применением
// truncateToolResultForHistory для tool-row.
func toLLMMessage(m Message, cfg Config) llm.Message {
	switch m.Role {
	case llm.RoleTool:
		// LLM получает tool-row как role=tool с парным tool_call_id.
		// Content — JSON-строка (truncated при необходимости).
		return llm.Message{
			Role:       llm.RoleTool,
			Content:    string(truncateToolResultForHistory(m.ToolResult, m.ToolName, cfg.MaxToolResultBytes)),
			ToolCallID: m.ToolCallID,
			Name:       m.ToolName,
		}

	case llm.RoleAssistant:
		return llm.Message{
			Role:      llm.RoleAssistant,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		}

	case llm.RoleUser, llm.RoleSystem:
		return llm.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	// Неизвестная роль — отдаём как system, чтобы Executor не упал на
	// случайном мусоре из БД (но это явный баг и должно ловиться
	// валидацией модели AssistantMessageRole.IsValid).
	return llm.Message{Role: llm.RoleSystem, Content: m.Content}
}

// truncateToolResultForHistory режет tool_result для подачи в LLM. Если
// payload помещается в max — возвращается как есть. Иначе — JSON-marker
// `{"status":"ok","truncated":true,"preview":...,"hint":...}`
// (см. план §3.4 п.1). max<=0 → без ограничений (тестовый режим).
func truncateToolResultForHistory(raw json.RawMessage, toolName string, max int) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	if max <= 0 || len(raw) <= max {
		return raw
	}

	// preview берётся в байтах. Это безопасно даже если разрезается посреди
	// UTF-8 кодпоинта — preview лежит как строка внутри JSON, LLM поймёт обрыв.
	// Берём ~75% от max, чтобы остался запас под маркер-обёртку.
	previewLen := max * 3 / 4
	if previewLen > len(raw) {
		previewLen = len(raw)
	}
	preview := string(raw[:previewLen])

	marker := struct {
		Status    string `json:"status"`
		Truncated bool   `json:"truncated"`
		Tool      string `json:"tool,omitempty"`
		Size      int    `json:"original_size_bytes"`
		Preview   string `json:"preview"`
		Hint      string `json:"hint"`
	}{
		Status:    "ok",
		Truncated: true,
		Tool:      toolName,
		Size:      len(raw),
		Preview:   preview,
		Hint:      "результат урезан; используй пагинацию (limit/cursor) или фильтры",
	}
	out, err := json.Marshal(marker)
	if err != nil {
		// Невозможно в теории (все поля сериализуемые), но если случилось —
		// отдаём минимально-валидный JSON, чтобы LLM-провайдер не упал на
		// невалидном `content` поле сообщения.
		return json.RawMessage(fmt.Sprintf(`{"status":"ok","truncated":true,"tool":%q,"hint":"failed to build preview"}`, toolName))
	}
	return out
}

// slidingWindowCompact урезает историю по бюджету Config.MaxHistoryBytes.
// Алгоритм:
//  1. Считаем суммарный размер каждого сообщения (после tool-truncation).
//  2. Если суммарно ≤ бюджета — возвращаем как есть.
//  3. Иначе: фиксируем «хвост» (последние HistoryTailKeep user/assistant
//     сообщений вместе с их tool_call/tool_result связками) — оставляем
//     полностью. Промежуточные tool/assistant-with-toolcalls сжимаем до
//     summary-сообщений «{tool}: ok/error, N bytes».
//  4. Старые user-сообщения всегда сохраняются (короткие).
//
// Цель — не убить семантику диалога, но влезть в context window.
func slidingWindowCompact(history []Message, cfg Config) []Message {
	if cfg.MaxHistoryBytes <= 0 {
		return history
	}

	// Шаг 1: размер с уже применённой tool-truncation (без аллокации llm.Message).
	sizes := make([]int, len(history))
	total := 0
	for i, m := range history {
		sizes[i] = approxMessageSize(m, cfg)
		total += sizes[i]
	}
	if total <= cfg.MaxHistoryBytes {
		return history
	}

	// Шаг 2: определяем границу «хвоста» — индекс первого сообщения хвоста.
	keep := cfg.HistoryTailKeep
	if keep < 0 {
		keep = 0
	}
	tailStart := tailStartIndex(history, keep)

	// Шаг 3: сжатие префиксного диапазона [0..tailStart) — все tool-row и
	// assistant-row с ToolCalls → краткий summary; user-row остаются как есть
	// (они обычно короткие — это команды/вопросы пользователя).
	out := make([]Message, 0, len(history))
	for i := 0; i < tailStart; i++ {
		m := history[i]
		switch m.Role {
		case llm.RoleTool:
			out = append(out, compactToolMessage(m))
		case llm.RoleAssistant:
			if len(m.ToolCalls) > 0 {
				out = append(out, compactAssistantWithCalls(m))
				continue
			}
			out = append(out, m)
		default:
			out = append(out, m)
		}
	}
	out = append(out, history[tailStart:]...)
	return out
}

// approxMessageSize даёт нижнюю оценку размера сообщения в байтах после
// per-tool truncation. Используется только для решения «сжимать или нет».
func approxMessageSize(m Message, cfg Config) int {
	size := len(m.Content)
	if m.Role == llm.RoleTool {
		// Tool-row передаётся как Content=string(truncated). Учитываем
		// max — реальная нагрузка на бюджет именно такая.
		if l := len(m.ToolResult); l > 0 {
			if cfg.MaxToolResultBytes > 0 && l > cfg.MaxToolResultBytes {
				size = cfg.MaxToolResultBytes
			} else {
				size = l
			}
		} else {
			size = 0
		}
	}
	// Tool-call args в assistant-row тоже бьют по бюджету.
	for _, tc := range m.ToolCalls {
		size += len(tc.Function.Arguments) + len(tc.Function.Name)
	}
	return size + 16 // overhead на JSON-обёртку
}

// tailStartIndex возвращает индекс, начиная с которого history считается
// «хвостом» — последние keep user/assistant сообщений плюс все tool-row
// между ними (чтобы не разорвать пару assistant.tool_call ↔ tool.result).
func tailStartIndex(history []Message, keep int) int {
	if keep <= 0 || len(history) == 0 {
		return len(history)
	}
	seen := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == llm.RoleUser || history[i].Role == llm.RoleAssistant {
			seen++
			if seen >= keep {
				return i
			}
		}
	}
	return 0
}

// compactToolMessage заменяет tool-row на короткий summary-маркер.
// Статус берётся через classifyStatus (executor.go) — единый JSON-парсер,
// устойчивый к пробелам/форматированию, ему доверяет UI/WS-уровень.
func compactToolMessage(m Message) Message {
	status := classifyStatus(m.ToolResult)
	summary, _ := json.Marshal(struct {
		Status    string `json:"status"`
		Compacted bool   `json:"compacted"`
		Tool      string `json:"tool,omitempty"`
		Size      int    `json:"original_size_bytes"`
		Hint      string `json:"hint"`
	}{
		Status:    status,
		Compacted: true,
		Tool:      m.ToolName,
		Size:      len(m.ToolResult),
		Hint:      "более ранний tool_result сжат для экономии контекста; перевызови инструмент, если нужны детали",
	})
	out := m
	out.ToolResult = summary
	out.Content = string(summary)
	return out
}

// compactAssistantWithCalls сворачивает assistant-row с tool_calls в текстовый
// маркер (LLM поймет, что это сжатый промежуточный ответ).
//
// Замечание: для строгой совместимости с tool-calling провайдерами
// (assistant.tool_calls должен иметь парный tool.tool_call_id) мы сохраняем
// исходное число пар: compactToolMessage сохраняет ToolCallID, а здесь
// оставляем ToolCalls без изменений. Поэтому реальное сжатие — только в Content/ToolResult.
func compactAssistantWithCalls(m Message) Message {
	out := m
	if out.Content == "" {
		out.Content = "(сжатый промежуточный ответ ассистента с вызовами инструментов)"
	}
	return out
}
