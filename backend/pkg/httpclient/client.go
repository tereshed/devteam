package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/tidwall/gjson"
)

// Client HTTP клиент для вызова внешних API
type Client struct {
	httpClient *http.Client
}

// Request описывает HTTP запрос
type Request struct {
	Method       string
	URL          string
	Headers      map[string]string
	BodyTemplate string
	TimeoutSec   int
	ExtractPath  string // JSONPath для извлечения результата
}

// Response результат HTTP запроса
type Response struct {
	StatusCode int
	Body       string
	Extracted  string // Извлечённое значение по ExtractPath
	DurationMs int
}

// New создаёт новый HTTP клиент
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute выполняет HTTP запрос с подстановкой переменных
func (c *Client) Execute(ctx context.Context, req Request, input string) (*Response, error) {
	startTime := time.Now()

	// Устанавливаем таймаут
	timeout := 30 * time.Second
	if req.TimeoutSec > 0 {
		timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Подготавливаем URL с шаблонизацией
	url, err := c.renderTemplate(req.URL, input)
	if err != nil {
		return nil, fmt.Errorf("failed to render URL template: %w", err)
	}

	// Подготавливаем тело запроса
	var body io.Reader
	if req.BodyTemplate != "" {
		bodyStr, err := c.renderTemplate(req.BodyTemplate, input)
		if err != nil {
			return nil, fmt.Errorf("failed to render body template: %w", err)
		}
		body = strings.NewReader(bodyStr)
	}

	// Создаём HTTP запрос
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	for key, value := range req.Headers {
		// Шаблонизируем значения заголовков
		renderedValue, err := c.renderTemplate(value, input)
		if err != nil {
			return nil, fmt.Errorf("failed to render header %s: %w", key, err)
		}
		httpReq.Header.Set(key, renderedValue)
	}

	// Устанавливаем Content-Type по умолчанию для POST/PUT
	if (req.Method == http.MethodPost || req.Method == http.MethodPut) && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Выполняем запрос
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	duration := time.Since(startTime)

	result := &Response{
		StatusCode: resp.StatusCode,
		Body:       string(bodyBytes),
		DurationMs: int(duration.Milliseconds()),
	}

	// Извлекаем значение по JSONPath если указан
	if req.ExtractPath != "" {
		extracted := gjson.Get(result.Body, req.ExtractPath)
		if extracted.Exists() {
			result.Extracted = extracted.String()
		} else {
			result.Extracted = result.Body // Если путь не найден, возвращаем всё тело
		}
	} else {
		result.Extracted = result.Body
	}

	return result, nil
}

// renderTemplate выполняет шаблонизацию строки с подстановкой input
func (c *Client) renderTemplate(templateStr, input string) (string, error) {
	// Простая замена {{.Input}} на значение
	// Для более сложных случаев используем text/template

	data := struct {
		Input string
	}{
		Input: input,
	}

	// Пробуем как JSON для доступа к полям
	var jsonData map[string]any
	if err := json.Unmarshal([]byte(input), &jsonData); err == nil {
		// Input - валидный JSON, добавляем поля в data
		// Можно расширить при необходимости
	}

	tmpl, err := template.New("request").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ValidateMethod проверяет корректность HTTP метода
func ValidateMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}
