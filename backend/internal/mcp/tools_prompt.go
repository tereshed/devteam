package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
)

// maxPromptNameLength — разумный лимит длины имени промпта для валидации входных параметров.
const maxPromptNameLength = 255

// --- Params ---

// PromptListParams — параметры для prompt_list
type PromptListParams struct {
	Limit  *int `json:"limit,omitempty" jsonschema:"description=Макс. количество промптов в ответе (1-100; по умолчанию 50)"`
	Offset *int `json:"offset,omitempty" jsonschema:"description=Сдвиг для пагинации (по умолчанию 0)"`
}

// PromptGetParams — параметры для prompt_get
type PromptGetParams struct {
	// Поиск по UUID или имени — одно из двух обязательно
	ID   string `json:"id,omitempty" jsonschema:"description=UUID промпта. Укажите id ИЛИ name"`
	Name string `json:"name,omitempty" jsonschema:"description=Имя промпта (точное совпадение). Укажите id ИЛИ name"`
}

// --- Data ---

// PromptListData — payload для prompt_list
type PromptListData struct {
	Prompts []PromptItem `json:"prompts"`
	Count   int          `json:"count"`
}

// PromptItem — один промпт в списке (краткая форма).
// Поле IsActive не включено: prompt_list возвращает только активные промпты.
type PromptItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PromptGetData — полная информация о промпте.
// Template возвращается целиком (без truncation), т.к. основная цель prompt_get —
// дать LLM-клиенту полный шаблон для использования.
type PromptGetData struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Template    string          `json:"template"`
	JSONSchema  json.RawMessage `json:"json_schema,omitempty"`
	IsActive    bool            `json:"is_active"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// --- Registration ---

// RegisterPromptTools регистрирует MCP-инструменты для работы с промптами
func RegisterPromptTools(server *mcp.Server, promptService service.PromptService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "prompt_list",
		Description: "Получить список активных промптов. Неактивные промпты не включаются. Поддерживает пагинацию (limit/offset).",
	}, makePromptListHandler(promptService))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "prompt_get",
		Description: "Получить полную информацию о промпте по ID или имени, включая template и json_schema. Возвращает в том числе неактивные промпты (с пометкой в details).",
	}, makePromptGetHandler(promptService))
}

// --- Handlers ---

func makePromptListHandler(promptSvc service.PromptService) func(ctx context.Context, req *mcp.CallToolRequest, params *PromptListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *PromptListParams) (*mcp.CallToolResult, any, error) {
		prompts, err := promptSvc.List(ctx)
		if err != nil {
			return Err("failed to list prompts", err)
		}

		// Фильтруем только активные
		items := make([]PromptItem, 0, len(prompts))
		for _, p := range prompts {
			if !p.IsActive {
				continue
			}
			items = append(items, PromptItem{
				ID:          p.ID.String(),
				Name:        p.Name,
				Description: p.Description,
			})
		}

		// Пагинация (in-memory; TODO: перенести limit/offset в PromptService.List когда интерфейс будет расширен)
		limit, offset := PaginateDefaults(params.limitVal(), params.offsetVal(), 50, 100)
		total := len(items)
		items = Paginate(items, limit, offset)

		return OK(
			fmt.Sprintf("found %d active prompts (showing %d, offset %d)", total, len(items), offset),
			&PromptListData{
				Prompts: items,
				Count:   total,
			},
		)
	}
}

func makePromptGetHandler(promptSvc service.PromptService) func(ctx context.Context, req *mcp.CallToolRequest, params *PromptGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *PromptGetParams) (*mcp.CallToolResult, any, error) {
		if params == nil {
			return ValidationErr("parameters are required (id or name)")
		}

		id := strings.TrimSpace(params.ID)
		name := strings.TrimSpace(params.Name)

		// Ровно один из двух должен быть указан
		if id == "" && name == "" {
			return ValidationErr("either id or name is required")
		}
		if id != "" && name != "" {
			return ValidationErr("specify only one of id or name, not both")
		}

		// Валидация длины name
		if name != "" && len(name) > maxPromptNameLength {
			return ValidationErr(fmt.Sprintf("name too long: %d chars (max %d)", len(name), maxPromptNameLength))
		}

		var prompt *models.Prompt

		if id != "" {
			// Поиск по UUID
			parsedID, err := uuid.Parse(id)
			if err != nil {
				return ValidationErr(fmt.Sprintf("invalid id: %q is not a valid UUID", id))
			}

			prompt, err = promptSvc.GetByID(ctx, parsedID)
			if err != nil {
				return Err(fmt.Sprintf("failed to get prompt by id %s", parsedID.String()), err)
			}
		} else {
			// Поиск по имени
			var err error
			prompt, err = promptSvc.GetByName(ctx, name)
			if err != nil {
				return Err(fmt.Sprintf("failed to get prompt by name %q", name), err)
			}
		}

		promptData := toPromptGetData(prompt)

		// Формируем details с учётом статуса активности
		details := fmt.Sprintf("prompt %q (%s)", promptData.Name, promptData.ID)
		if !prompt.IsActive {
			details += " [WARNING: inactive]"
		}

		return OK(details, promptData)
	}
}

// --- Helpers ---

// toPromptGetData конвертирует models.Prompt в PromptGetData.
func toPromptGetData(p *models.Prompt) *PromptGetData {
	data := &PromptGetData{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Template:    p.Template,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}

	// JSONSchema: проверяем валидность перед вставкой в ответ.
	// Невалидный JSON привёл бы к ошибке сериализации Response — лучше пропустить.
	if len(p.JSONSchema) > 0 && json.Valid([]byte(p.JSONSchema)) {
		data.JSONSchema = json.RawMessage(p.JSONSchema)
	}

	return data
}

// --- Pagination getter'ы (методы — единый стиль с tools_workflow.go) ---

func (p *PromptListParams) limitVal() *int {
	if p == nil {
		return nil
	}
	return p.Limit
}

func (p *PromptListParams) offsetVal() *int {
	if p == nil {
		return nil
	}
	return p.Offset
}
