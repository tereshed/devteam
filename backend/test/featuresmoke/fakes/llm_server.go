//go:build featuresmoke

// Package fakes — детерминированные HTTP-стабы для PR-gate featuresmoke.
//
// llm_server.go реализует HTTP-сервер, имитирующий Anthropic Messages API
// (/v1/messages, /messages) и OpenAI-совместимый /v1/chat/completions
// (используется DeepSeek/OpenRouter/Ollama-провайдерами). Маршрутизация
// ответов — детерминированная: по содержимому prompt'а (regexp/contains).
//
// КРИТИЧНО (thread-safety): тесты бегут с t.Parallel(), значит fakes должны
// быть потокобезопасны — все мутации через мьютексы.
//
// КРИТИЧНО (fast-fail): неизвестный prompt → `t.Fatalf` через зарегистрированный
// *testing.T. Это ловит регрессии в самих тестах (новый сценарий → нужно
// добавить response), а не позволяет ответу-по-умолчанию замаскировать баг.
package fakes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// FakeLLM — HTTP-стаб для Anthropic + OpenAI-совместимых API.
// Конструируется через NewFakeLLM(t). Использовать только в featuresmoke-тестах.
type FakeLLM struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	rules   []llmRule
	calls   []LLMCall
	fastFail bool
}

// LLMCall — запись о вызове, для assert'ов из тестов.
type LLMCall struct {
	Path    string
	Prompt  string
	Headers http.Header
	Body    []byte
}

type llmRule struct {
	match  *regexp.Regexp
	reply  string // plain-текстовый assistant-ответ
}

// NewFakeLLM поднимает httptest-сервер. Cleanup — через t.Cleanup.
// По умолчанию fast-fail активен: неизвестный prompt роняет тест.
func NewFakeLLM(t *testing.T) *FakeLLM {
	t.Helper()
	f := &FakeLLM{t: t, fastFail: true}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

// NewFakeLLMGlobal поднимает FakeLLM без привязки к конкретному *testing.T.
// Используется harness'ом в bootstrapServer'е, чтобы перехватить ANY LLM-запрос
// от стартующего backend'а (фоновые orchestrator-воркеры, assistant и т.п.)
// ДО того как control дойдёт до конкретного теста.
//
// Fast-fail OFF: для глобального стаба «неизвестный prompt» — нормальное состояние
// (никто не регистрирует rules заранее), и роняем не отдельный тест, а возвращаем
// generic-reply, чтобы backend не получал HTTP 500 и не уходил в retry-петли.
//
// Close() ОБЯЗАН вызвать caller — обычно через registerGlobalCleanup в harness.go.
func NewFakeLLMGlobal() *FakeLLM {
	f := &FakeLLM{t: nil, fastFail: false}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

// Close останавливает httptest-сервер. Для NewFakeLLM (с t) cleanup идёт через
// t.Cleanup и эту функцию вызывать не нужно. Для NewFakeLLMGlobal обязательно
// дёргать вручную при остановке тестового процесса.
func (f *FakeLLM) Close() {
	if f != nil && f.srv != nil {
		f.srv.Close()
	}
}

// URL возвращает базовый URL стаба (без trailing slash). Подходит для
// ANTHROPIC_BASE_URL / OPENAI_BASE_URL / DEEPSEEK_BASE_URL в backend env.
func (f *FakeLLM) URL() string { return f.srv.URL }

// SetFastFail отключает/включает fast-fail. По умолчанию — true.
// Отключать стоит только в тестах, которые специально проверяют, что backend
// корректно обрабатывает пустые / некорректные ответы LLM.
func (f *FakeLLM) SetFastFail(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fastFail = v
}

// On регистрирует rule «если prompt матчит regex — ответить reply».
// Правила проверяются в порядке регистрации, побеждает первый матч.
func (f *FakeLLM) On(promptRegex, reply string) *FakeLLM {
	re := regexp.MustCompile(promptRegex)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules = append(f.rules, llmRule{match: re, reply: reply})
	return f
}

// Calls возвращает копию журнала вызовов. Безопасно вызывать из тестового
// горутины параллельно с активными запросами стабе.
func (f *FakeLLM) Calls() []LLMCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]LLMCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// CallCount возвращает текущее число записанных вызовов. Удобный шорткат для
// смоук-проверок «должно быть ровно N вызовов после step X».
func (f *FakeLLM) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// Reset очищает журнал вызовов и зарегистрированные rules. Используется
// смоук-тестами, чтобы изолировать наблюдение от предыдущих тестов в shared-
// процессе. Mutex обеспечивает thread-safety.
func (f *FakeLLM) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
	f.rules = nil
}

// maxRequestBodyBytes — защита от исчерпания памяти, если тестируемый клиент
// уйдёт в петлю и начнёт слать гигабайты. Реальные prompts featuresmoke
// помещаются в десятки килобайт; 10 MiB — на порядок выше, но всё ещё
// безопасно для парсинга/regexp.
const maxRequestBodyBytes = 10 << 20

func (f *FakeLLM) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	prompt := extractPrompt(body)

	f.mu.Lock()
	f.calls = append(f.calls, LLMCall{Path: r.URL.Path, Prompt: prompt, Headers: r.Header.Clone(), Body: append([]byte(nil), body...)})
	var reply string
	var matched bool
	for _, rule := range f.rules {
		if rule.match.MatchString(prompt) {
			reply = rule.reply
			matched = true
			break
		}
	}
	fastFail := f.fastFail
	f.mu.Unlock()

	if !matched {
		if fastFail {
			// f.t может быть nil для NewFakeLLMGlobal (harness); там fastFail OFF,
			// сюда не дойдём. На всякий случай — guard.
			if f.t != nil {
				f.t.Errorf("FakeLLM: неизвестный prompt на %s: %q", r.URL.Path, truncate(prompt, 500))
			}
			http.Error(w, "FakeLLM: unknown prompt", http.StatusInternalServerError)
			return
		}
		reply = "FakeLLM default reply (fast-fail off)"
	}

	switch {
	case strings.HasSuffix(r.URL.Path, "/messages"):
		writeAnthropic(w, reply)
	case strings.HasSuffix(r.URL.Path, "/chat/completions"):
		writeOpenAI(w, reply)
	default:
		// По умолчанию — OpenAI-формат, он наиболее распространён.
		writeOpenAI(w, reply)
	}
}

// extractPrompt извлекает «всё, что отправил клиент в LLM» в одну строку:
// system + messages.content (Anthropic) или messages[*].content (OpenAI).
// Достаточно для regexp-матчинга в rules; точный JSON-парсинг не нужен.
func extractPrompt(body []byte) string {
	var anth struct {
		System   string `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &anth); err == nil {
		var b strings.Builder
		if anth.System != "" {
			b.WriteString(anth.System)
			b.WriteByte('\n')
		}
		for _, m := range anth.Messages {
			b.WriteString(m.Role)
			b.WriteString(": ")
			b.Write(flattenContent(m.Content))
			b.WriteByte('\n')
		}
		return b.String()
	}
	return string(body)
}

// flattenContent поддерживает оба формата content:
//   - "string"
//   - [{"type":"text","text":"..."}, ...]
func flattenContent(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return []byte(s)
		}
	}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var b bytes.Buffer
		for _, p := range arr {
			b.WriteString(p.Text)
			b.WriteByte('\n')
		}
		return b.Bytes()
	}
	return raw
}

// fakeAnthropicModel / fakeOpenAIModel — имена дешёвых моделей, под которые
// маскируется стаб. Совпадают с TestModelAnthropic/TestModelOpenAI в harness.go;
// дублируем константой здесь, чтобы fakes/ не импортировал родительский пакет
// (избегаем import-cycle featuresmoke ↔ fakes).
const (
	fakeAnthropicModel = "claude-haiku-4-5-20251001"
	fakeOpenAIModel    = "gpt-4o-mini"
)

func writeAnthropic(w http.ResponseWriter, reply string) {
	resp := map[string]any{
		"id":         "fake-msg-1",
		"type":       "message",
		"role":       "assistant",
		"model":      fakeAnthropicModel,
		"stop_reason": "end_turn",
		"content": []map[string]any{
			{"type": "text", "text": reply},
		},
		"usage": map[string]int{"input_tokens": 10, "output_tokens": 10},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeOpenAI(w http.ResponseWriter, reply string) {
	resp := map[string]any{
		"id":      "fake-chatcmpl-1",
		"object":  "chat.completion",
		"created": 1,
		"model":   fakeOpenAIModel,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": reply},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("...(+%d bytes)", len(s)-n)
}
