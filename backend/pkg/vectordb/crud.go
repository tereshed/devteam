package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	weaviateModels "github.com/weaviate/weaviate/entities/models"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

// Create создает документ в векторной базе
func (c *Client) Create(ctx context.Context, doc *models.VectorDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("document cannot be nil")
	}

	// Валидация обязательных полей
	if doc.ContentID == "" {
		return "", fmt.Errorf("contentId is required")
	}
	if doc.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	if !doc.ContentType.IsValid() {
		return "", fmt.Errorf("invalid content type: %s", doc.ContentType)
	}

	// Сериализуем metadata в JSON string
	var metadataJSON string
	if doc.Metadata != nil {
		metadataBytes, err := json.Marshal(doc.Metadata)
		if err != nil {
			return "", fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	// Подготовка данных для Weaviate
	properties := map[string]interface{}{
		"contentId":   doc.ContentID,
		"content":     doc.Content,
		"contentType": string(doc.ContentType),
		"category":    doc.Category,
		"tags":        doc.Tags,
		"metadata":    metadataJSON,
		"createdAt":   doc.CreatedAt.Format(time.RFC3339),
		"updatedAt":   doc.UpdatedAt.Format(time.RFC3339),
	}

	// Создание объекта
	result, err := c.weaviate.Data().Creator().
		WithClassName(ClassName).
		WithProperties(properties).
		Do(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to create vector document: %w", err)
	}

	// Возвращаем UUID созданного объекта
	if result != nil && result.Object != nil && result.Object.ID != "" {
		return string(result.Object.ID), nil
	}

	return "", fmt.Errorf("weaviate returned empty ID")
}

// Get получает документ по ID
func (c *Client) Get(ctx context.Context, id string) (*models.VectorDocument, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	// Получаем объект
	objects, err := c.weaviate.Data().ObjectsGetter().
		WithClassName(ClassName).
		WithID(id).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get vector document: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("document not found")
	}

	// Парсим результат
	return parseWeaviateObject(objects[0])
}

// Update обновляет документ
func (c *Client) Update(ctx context.Context, id string, doc *models.VectorDocument) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	// Сериализуем metadata в JSON string
	var metadataJSON string
	if doc.Metadata != nil {
		metadataBytes, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	// Подготовка данных
	properties := map[string]interface{}{
		"contentId":   doc.ContentID,
		"content":     doc.Content,
		"contentType": string(doc.ContentType),
		"category":    doc.Category,
		"tags":        doc.Tags,
		"metadata":    metadataJSON,
		"updatedAt":   time.Now().Format(time.RFC3339),
	}

	// Обновление объекта
	err := c.weaviate.Data().Updater().
		WithClassName(ClassName).
		WithID(id).
		WithProperties(properties).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to update vector document: %w", err)
	}

	return nil
}

// Delete удаляет документ
func (c *Client) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	err := c.weaviate.Data().Deleter().
		WithClassName(ClassName).
		WithID(id).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete vector document: %w", err)
	}

	return nil
}

// BatchCreate создает несколько документов за один запрос
func (c *Client) BatchCreate(ctx context.Context, docs []*models.VectorDocument) (*IndexStats, error) {
	if len(docs) == 0 {
		return &IndexStats{}, nil
	}

	stats := &IndexStats{
		TotalProcessed: len(docs),
		Errors:         []string{},
	}

	// Создаем batch
	batch := c.weaviate.Batch().ObjectsBatcher()

	for _, doc := range docs {
		// Валидация
		if doc.ContentID == "" || doc.Content == "" || !doc.ContentType.IsValid() {
			stats.Failed++
			stats.Errors = append(stats.Errors, fmt.Sprintf("invalid document: contentID=%s", doc.ContentID))
			continue
		}

		// Сериализуем metadata в JSON string
		var metadataJSON string
		if doc.Metadata != nil {
			metadataBytes, err := json.Marshal(doc.Metadata)
			if err != nil {
				stats.Failed++
				stats.Errors = append(stats.Errors, fmt.Sprintf("failed to marshal metadata for contentID=%s: %v", doc.ContentID, err))
				continue
			}
			metadataJSON = string(metadataBytes)
		}

		properties := map[string]interface{}{
			"contentId":   doc.ContentID,
			"content":     doc.Content,
			"contentType": string(doc.ContentType),
			"category":    doc.Category,
			"tags":        doc.Tags,
			"metadata":    metadataJSON,
			"createdAt":   doc.CreatedAt.Format(time.RFC3339),
			"updatedAt":   doc.UpdatedAt.Format(time.RFC3339),
		}

		obj := &weaviateModels.Object{
			Class:      ClassName,
			Properties: properties,
		}

		batch = batch.WithObjects(obj)
	}

	// Выполняем batch
	results, err := batch.Do(ctx)
	if err != nil {
		return stats, fmt.Errorf("batch create failed: %w", err)
	}

	// Анализируем результаты
	for _, result := range results {
		if result.Result != nil && result.Result.Errors != nil && result.Result.Errors.Error != nil {
			stats.Failed++
			if len(result.Result.Errors.Error) > 0 {
				stats.Errors = append(stats.Errors, result.Result.Errors.Error[0].Message)
			}
		} else {
			stats.Succeeded++
		}
	}

	return stats, nil
}

// DeleteByContentID удаляет документы по contentId
func (c *Client) DeleteByContentID(ctx context.Context, contentID string) error {
	if contentID == "" {
		return fmt.Errorf("contentID cannot be empty")
	}

	// Используем batch deleter с фильтром
	_, err := c.weaviate.Batch().ObjectsBatchDeleter().
		WithClassName(ClassName).
		WithWhere(filters.Where().
			WithPath([]string{"contentId"}).
			WithOperator(filters.Equal).
			WithValueString(contentID)).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete by contentID: %w", err)
	}

	return nil
}

// DeleteByContentType удаляет все документы определенного типа
func (c *Client) DeleteByContentType(ctx context.Context, contentType models.ContentType, category string) error {
	// Если contentType пустой - удаляем все документы данной категории
	var whereBuilder *filters.WhereBuilder

	if contentType != "" {
		whereBuilder = filters.Where().
			WithPath([]string{"contentType"}).
			WithOperator(filters.Equal).
			WithValueString(string(contentType))

		// Если указана категория, добавляем дополнительный фильтр
		if category != "" {
			whereBuilder = filters.Where().
				WithOperator(filters.And).
				WithOperands([]*filters.WhereBuilder{
					whereBuilder,
					filters.Where().
						WithPath([]string{"category"}).
						WithOperator(filters.Equal).
						WithValueString(category),
				})
		}
	} else if category != "" {
		// Только по категории
		whereBuilder = filters.Where().
			WithPath([]string{"category"}).
			WithOperator(filters.Equal).
			WithValueString(category)
	} else {
		// Удаляем все документы класса
		return c.deleteAllDocuments(ctx)
	}

	_, err := c.weaviate.Batch().ObjectsBatchDeleter().
		WithClassName(ClassName).
		WithWhere(whereBuilder).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete by content type: %w", err)
	}

	return nil
}

// deleteAllDocuments удаляет все документы из класса
func (c *Client) deleteAllDocuments(ctx context.Context) error {
	// Удаляем документы где contentId не пустой (т.е. все документы)
	_, err := c.weaviate.Batch().ObjectsBatchDeleter().
		WithClassName(ClassName).
		WithWhere(filters.Where().
			WithPath([]string{"contentId"}).
			WithOperator(filters.NotEqual).
			WithValueString("")).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete all documents: %w", err)
	}

	return nil
}

// CountByContentType возвращает количество документов определенного типа и категории
func (c *Client) CountByContentType(ctx context.Context, contentType models.ContentType, category string) (int64, error) {
	// Строим фильтр
	var whereConditions []*filters.WhereBuilder

	// Добавляем фильтр по типу если указан
	if contentType != "" {
		whereConditions = append(whereConditions, filters.Where().
			WithPath([]string{"contentType"}).
			WithOperator(filters.Equal).
			WithValueString(string(contentType)))
	}

	// Добавляем фильтр по категории если указана
	if category != "" {
		whereConditions = append(whereConditions, filters.Where().
			WithPath([]string{"category"}).
			WithOperator(filters.Equal).
			WithValueString(category))
	}

	// Строим запрос
	builder := c.weaviate.GraphQL().Aggregate().
		WithClassName(ClassName).
		WithFields(graphql.Field{Name: "meta", Fields: []graphql.Field{{Name: "count"}}})

	// Добавляем фильтр если есть условия
	if len(whereConditions) > 0 {
		var whereFilter *filters.WhereBuilder
		if len(whereConditions) == 1 {
			whereFilter = whereConditions[0]
		} else {
			whereFilter = filters.Where().
				WithOperator(filters.And).
				WithOperands(whereConditions)
		}
		builder = builder.WithWhere(whereFilter)
	}

	// Выполняем aggregate запрос
	result, err := builder.Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Парсим результат
	if result.Data == nil {
		return 0, nil
	}

	aggregate, ok := result.Data["Aggregate"].(map[string]interface{})
	if !ok {
		return 0, nil
	}

	items, ok := aggregate[ClassName].([]interface{})
	if !ok || len(items) == 0 {
		return 0, nil
	}

	item, ok := items[0].(map[string]interface{})
	if !ok {
		return 0, nil
	}

	meta, ok := item["meta"].(map[string]interface{})
	if !ok {
		return 0, nil
	}

	count, ok := meta["count"].(float64)
	if !ok {
		return 0, nil
	}

	return int64(count), nil
}

// parseWeaviateObject преобразует Weaviate объект в VectorDocument
func parseWeaviateObject(obj *weaviateModels.Object) (*models.VectorDocument, error) {
	if obj == nil || obj.Properties == nil {
		return nil, fmt.Errorf("invalid weaviate object")
	}

	props := obj.Properties.(map[string]interface{})

	doc := &models.VectorDocument{
		ID:       string(obj.ID),
		Metadata: make(map[string]interface{}),
		Tags:     []string{},
	}

	if contentId, ok := props["contentId"].(string); ok {
		doc.ContentID = contentId
	}

	if content, ok := props["content"].(string); ok {
		doc.Content = content
	}

	if contentType, ok := props["contentType"].(string); ok {
		doc.ContentType = models.ContentType(contentType)
	}

	if category, ok := props["category"].(string); ok {
		doc.Category = category
	}

	if tags, ok := props["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				doc.Tags = append(doc.Tags, tagStr)
			}
		}
	}

	// Десериализуем metadata из JSON string
	if metadataJSON, ok := props["metadata"].(string); ok && metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			doc.Metadata = metadata
		}
	}

	if createdAt, ok := props["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			doc.CreatedAt = t
		}
	}

	if updatedAt, ok := props["updatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}
	}

	return doc, nil
}
