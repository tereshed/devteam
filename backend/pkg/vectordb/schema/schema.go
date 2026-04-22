package schema

import (
	"context"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

// CreateSchema создает схему в Weaviate
func CreateSchema(ctx context.Context, client *weaviate.Client, className string) error {
	class := GetDocumentClass(className)

	err := client.Schema().ClassCreator().WithClass(class).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// EnsureSchemaExists проверяет существование схемы и создает её при необходимости
func EnsureSchemaExists(ctx context.Context, client *weaviate.Client, className string) error {
	exists, err := client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check schema existence: %w", err)
	}

	if exists {
		return nil // Схема уже существует
	}

	return CreateSchema(ctx, client, className)
}

// DeleteSchema удаляет схему из Weaviate
func DeleteSchema(ctx context.Context, client *weaviate.Client, className string) error {
	err := client.Schema().ClassDeleter().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete schema: %w", err)
	}
	return nil
}

// GetDocumentClass возвращает универсальное определение класса Document с заданным именем
func GetDocumentClass(className string) *models.Class {
	return &models.Class{
		Class:       className,
		Description: "Generic document for vector search",
		Vectorizer:  "text2vec-transformers",

		ModuleConfig: map[string]interface{}{
			"text2vec-transformers": map[string]interface{}{
				"vectorizeClassName": false,
			},
		},

		Properties: []*models.Property{
			// ContentID - ссылка на основную БД
			{
				Name:         "contentId",
				DataType:     []string{"text"},
				Description:  "Reference ID to main database record",
				Tokenization: "field", // Не токенизируем (это ID)
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": true, // Не векторизуем ID
					},
				},
			},

			// Content - основной текст для векторизации
			{
				Name:        "content",
				DataType:    []string{"text"},
				Description: "Main content for vectorization",
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": false, // Это поле векторизуется
					},
				},
			},

			// ContentType - тип контента (для фильтрации)
			{
				Name:         "contentType",
				DataType:     []string{"text"},
				Description:  "Type of content (defined by your application)",
				Tokenization: "field", // Exact match
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": true,
					},
				},
			},

			// Category - категория (для фильтрации)
			{
				Name:         "category",
				DataType:     []string{"text"},
				Description:  "Category for filtering",
				Tokenization: "field", // Exact match
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": true,
					},
				},
			},

			// Tags - теги (для фильтрации)
			{
				Name:        "tags",
				DataType:    []string{"text[]"},
				Description: "Tags for filtering",
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": true,
					},
				},
			},

			// Metadata - дополнительные данные (JSON)
			{
				Name:        "metadata",
				DataType:    []string{"text"},
				Description: "Additional metadata as JSON string",
				ModuleConfig: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"skip": true,
					},
				},
			},

			// CreatedAt - время создания
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "Creation timestamp",
			},

			// UpdatedAt - время обновления
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "Last update timestamp",
			},
		},
	}
}
