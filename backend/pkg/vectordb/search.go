package vectordb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	weaviateModels "github.com/weaviate/weaviate/entities/models"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

// Search выполняет гибридный поиск в Weaviate
func (c *Client) Search(ctx context.Context, params SearchParams) ([]*SearchResult, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	if params.Limit <= 0 {
		params.Limit = 10 // Значение по умолчанию
	}

	if params.Alpha < 0 || params.Alpha > 1 {
		params.Alpha = 0.5 // Гибридный поиск по умолчанию
	}

	// Построение GraphQL запроса
	hybridBuilder := graphql.HybridArgumentBuilder{}
	hybridBuilder.WithQuery(params.Query)
	hybridBuilder.WithAlpha(params.Alpha)

	builder := c.weaviate.GraphQL().Get().
		WithClassName(ClassName).
		WithHybrid(&hybridBuilder).
		WithLimit(params.Limit).
		WithFields(
			graphql.Field{Name: "contentId"},
			graphql.Field{Name: "content"},
			graphql.Field{Name: "contentType"},
			graphql.Field{Name: "category"},
			graphql.Field{Name: "tags"},
			graphql.Field{Name: "metadata"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "score"},
					{Name: "distance"},
				},
			},
		)

	// Применение фильтров
	if len(params.ContentTypes) > 0 || params.Category != "" || len(params.Tags) > 0 || len(params.ContentIDs) > 0 {
		whereFilter := buildWhereFilter(params)
		if whereFilter != nil {
			builder = builder.WithWhere(whereFilter)
		}
	}

	// Выполнение запроса
	result, err := builder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	// Парсинг результатов
	return parseSearchResults(result)
}

// SemanticSearch выполняет только векторный поиск (Alpha = 1.0)
func (c *Client) SemanticSearch(ctx context.Context, query string, category string, limit int) ([]*SearchResult, error) {
	return c.Search(ctx, SearchParams{
		Query:    query,
		Category: category,
		Limit:    limit,
		Alpha:    1.0, // Только векторный поиск
	})
}

// KeywordSearch выполняет только BM25 поиск (Alpha = 0.0)
func (c *Client) KeywordSearch(ctx context.Context, query string, category string, limit int) ([]*SearchResult, error) {
	return c.Search(ctx, SearchParams{
		Query:    query,
		Category: category,
		Limit:    limit,
		Alpha:    0.0, // Только keyword поиск
	})
}

// buildWhereFilter строит WHERE фильтр для Weaviate
func buildWhereFilter(params SearchParams) *filters.WhereBuilder {
	var conditions []*filters.WhereBuilder

	// Фильтр по типу контента
	if len(params.ContentTypes) > 0 {
		if len(params.ContentTypes) == 1 {
			// Одно значение
			conditions = append(conditions, filters.Where().
				WithPath([]string{"contentType"}).
				WithOperator(filters.Equal).
				WithValueText(string(params.ContentTypes[0])))
		} else {
			// Несколько значений - OR
			typeConditions := make([]*filters.WhereBuilder, len(params.ContentTypes))
			for i, ct := range params.ContentTypes {
				typeConditions[i] = filters.Where().
					WithPath([]string{"contentType"}).
					WithOperator(filters.Equal).
					WithValueText(string(ct))
			}
			conditions = append(conditions, filters.Where().
				WithOperator(filters.Or).
				WithOperands(typeConditions))
		}
	}

	// Фильтр по категории
	if params.Category != "" {
		conditions = append(conditions, filters.Where().
			WithPath([]string{"category"}).
			WithOperator(filters.Equal).
			WithValueText(params.Category))
	}

	// Фильтр по тегам (ContainsAny)
	if len(params.Tags) > 0 {
		conditions = append(conditions, filters.Where().
			WithPath([]string{"tags"}).
			WithOperator(filters.ContainsAny).
			WithValueText(params.Tags...))
	}

	// Фильтр по конкретным ID контента
	if len(params.ContentIDs) > 0 {
		if len(params.ContentIDs) == 1 {
			conditions = append(conditions, filters.Where().
				WithPath([]string{"contentId"}).
				WithOperator(filters.Equal).
				WithValueText(params.ContentIDs[0]))
		} else {
			idConditions := make([]*filters.WhereBuilder, len(params.ContentIDs))
			for i, id := range params.ContentIDs {
				idConditions[i] = filters.Where().
					WithPath([]string{"contentId"}).
					WithOperator(filters.Equal).
					WithValueText(id)
			}
			conditions = append(conditions, filters.Where().
				WithOperator(filters.Or).
				WithOperands(idConditions))
		}
	}

	// Объединение условий через AND
	if len(conditions) == 0 {
		return nil
	}

	if len(conditions) == 1 {
		return conditions[0]
	}

	return filters.Where().
		WithOperator(filters.And).
		WithOperands(conditions)
}

// parseSearchResults парсит результаты из Weaviate GraphQL ответа
func parseSearchResults(result *weaviateModels.GraphQLResponse) ([]*SearchResult, error) {
	if result.Errors != nil && len(result.Errors) > 0 {
		return nil, fmt.Errorf("weaviate returned errors: %v", result.Errors)
	}

	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing Get")
	}

	items, ok := data[ClassName].([]interface{})
	if !ok {
		return []*SearchResult{}, nil // Пустой результат
	}

	results := make([]*SearchResult, 0, len(items))

	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		searchResult := &SearchResult{
			Metadata: make(map[string]interface{}),
		}

		// Извлечение полей
		if contentId, ok := itemMap["contentId"].(string); ok {
			searchResult.ContentID = contentId
		}

		if content, ok := itemMap["content"].(string); ok {
			searchResult.Content = content
		}

		if contentType, ok := itemMap["contentType"].(string); ok {
			searchResult.ContentType = models.ContentType(contentType)
		}

		if category, ok := itemMap["category"].(string); ok {
			searchResult.Category = category
		}

		// Десериализуем metadata из JSON string
		if metadataJSON, ok := itemMap["metadata"].(string); ok && metadataJSON != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
				searchResult.Metadata = metadata
			}
		}

		// Извлечение _additional (ID, score, distance)
		if additional, ok := itemMap["_additional"].(map[string]interface{}); ok {
			if id, ok := additional["id"].(string); ok {
				searchResult.VectorID = id
			}

			if score, ok := additional["score"].(float64); ok {
				searchResult.Score = float32(score)
			}

			if distance, ok := additional["distance"].(float64); ok {
				searchResult.Distance = float32(distance)
			}
		}

		results = append(results, searchResult)
	}

	return results, nil
}
