package mcp

import (
	"encoding/json"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Response — универсальная обёртка ответа для ВСЕХ MCP-инструментов.
//
// Каждый tool возвращает ровно одну такую структуру.
// LLM-клиент всегда получает предсказуемый формат:
//
//	{
//	  "status": "ok" | "error",
//	  "details": "человеко-читаемое описание",
//	  "data": { ... }   // payload конкретного инструмента, null при ошибке
//	}
//
// Контракт: Response.Status и CallToolResult.IsError ВСЕГДА синхронизированы:
//   - Status="ok"    → IsError=false
//   - Status="error" → IsError=true
//
// Источник правды — Response.Status (он внутри JSON-тела и доступен клиенту).
// IsError дублирует сигнал на уровне MCP-протокола для совместимости с SDK.
type Response struct {
	Status  string `json:"status"`         // "ok" или "error"
	Details string `json:"details"`        // описание результата / текст ошибки
	Data    any    `json:"data,omitempty"` // payload (nil при ошибке)
}

const (
	StatusOK    = "ok"
	StatusError = "error"
)

// --- Фабрики ---

// OK создаёт успешный Response и сразу возвращает MCP-кортеж (всегда HTTP 2xx).
//   - details: краткое описание для LLM ("Generated 512 tokens", "Found 3 workflows" и т.д.)
//   - data: payload конкретного инструмента (любая структура)
func OK(details string, data any) (*mcp.CallToolResult, any, error) {
	resp := &Response{
		Status:  StatusOK,
		Details: details,
		Data:    data,
	}
	return toMCPResult(resp, false)
}

// Err создаёт ответ с ошибкой и сразу возвращает MCP-кортеж (всегда HTTP 2xx, IsError=true).
//   - details: user-facing сообщение для LLM
//   - internalErr: полная ошибка для серверных логов (может быть nil)
func Err(details string, internalErr error) (*mcp.CallToolResult, any, error) {
	if internalErr != nil {
		log.Printf("[mcp/tool] error: %v (user message: %s)", internalErr, details)
	}
	resp := &Response{
		Status:  StatusError,
		Details: details,
		Data:    nil,
	}
	return toMCPResult(resp, true)
}

// ValidationErr — shortcut для ошибки валидации входных параметров.
// Не логирует внутреннюю ошибку (нет internalErr), так как это user input error.
func ValidationErr(details string) (*mcp.CallToolResult, any, error) {
	resp := &Response{
		Status:  StatusError,
		Details: details,
		Data:    nil,
	}
	return toMCPResult(resp, true)
}

// --- Константы для truncation ---

const (
	// TruncateDefault — лимит обрезки для основных текстовых полей (input/output)
	TruncateDefault = 500
	// TruncateShort — лимит обрезки для вторичных полей (step context)
	TruncateShort = 300
)

// Truncate обрезает строку до maxRunes рун, добавляя "..." если обрезана.
func Truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// --- Пагинация ---

// PaginateDefaults нормализует limit/offset с дефолтами и верхней границей
func PaginateDefaults(limitPtr, offsetPtr *int, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if limitPtr != nil && *limitPtr > 0 {
		limit = *limitPtr
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset = 0
	if offsetPtr != nil && *offsetPtr > 0 {
		offset = *offsetPtr
	}
	return
}

// Paginate применяет limit/offset к слайсу
func Paginate[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// --- Внутренние хелперы ---

// toMCPResult сериализует Response в JSON и оборачивает в CallToolResult.
// ГАРАНТИЯ: 3-й return (error) ВСЕГДА nil → HTTP ВСЕГДА 2xx.
func toMCPResult(resp *Response, isError bool) (*mcp.CallToolResult, any, error) {
	text, err := json.Marshal(resp)
	if err != nil {
		// Крайний fallback — не должно случаться, но если случится — не ломаем транспорт
		log.Printf("[mcp/result] CRITICAL: failed to marshal Response: %v", err)
		text = []byte(`{"status":"error","details":"internal serialization error"}`)
		isError = true
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(text)},
		},
		IsError: isError,
	}, resp, nil // structured = тот же Response; error = ВСЕГДА nil
}
