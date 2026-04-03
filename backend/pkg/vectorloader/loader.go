package vectorloader

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/devteam/backend/pkg/vectordb/strategy"
)

// VectorRepository определяет интерфейс для работы с векторной БД
// (копия из repository, чтобы избежать циклических зависимостей)
type VectorRepository interface {
	BatchCreate(ctx context.Context, docs []*models.VectorDocument) (*vectordb.IndexStats, error)
	CountByContentType(ctx context.Context, contentType models.ContentType, category string) (int64, error)
	DeleteByContentType(ctx context.Context, contentType models.ContentType, category string) error
}

// DataSource интерфейс для источника данных
// Реализуйте этот интерфейс для каждого типа контента в вашем проекте
type DataSource interface {
	// GetItems возвращает данные для индексации
	// Каждый item должен быть совместим со стратегией (map[string]interface{} или структура)
	GetItems(ctx context.Context, category string) ([]interface{}, error)

	// GetItemID возвращает ID элемента
	GetItemID(item interface{}) string

	// GetItemName возвращает имя/описание элемента для логов
	GetItemName(item interface{}) string
}

// VectorLoader отвечает за загрузку данных в векторную БД
type VectorLoader struct {
	vectorRepo VectorRepository
}

// NewVectorLoader создает новый loader
func NewVectorLoader(vectorRepo VectorRepository) *VectorLoader {
	return &VectorLoader{
		vectorRepo: vectorRepo,
	}
}

// LoadResult результат загрузки
type LoadResult struct {
	ContentType  models.ContentType `json:"content_type"`
	Category     string             `json:"category,omitempty"`
	TotalItems   int                `json:"total_items"`
	IndexedItems int                `json:"indexed_items"`
	SkippedItems int                `json:"skipped_items"`
	FailedItems  int                `json:"failed_items"`
	Duration     time.Duration      `json:"duration"`
	Errors       []string           `json:"errors,omitempty"`
}

// LoadFromDataSource загружает данные из источника в векторную БД
func (l *VectorLoader) LoadFromDataSource(
	ctx context.Context,
	dataSource DataSource,
	contentType models.ContentType,
	category string,
) (*LoadResult, error) {
	startTime := time.Now()

	result := &LoadResult{
		ContentType: contentType,
		Category:    category,
		Errors:      []string{},
	}

	log.Printf("[VectorLoader] Starting to load %s (category: %s)", contentType, category)

	// 1. Проверяем, нужна ли индексация
	indexedCount, err := l.vectorRepo.CountByContentType(ctx, contentType, category)
	if err != nil {
		return nil, fmt.Errorf("failed to count indexed items: %w", err)
	}

	if indexedCount > 0 {
		log.Printf("[VectorLoader] %s already indexed for category '%s' (%d documents), skipping",
			contentType, category, indexedCount)
		result.SkippedItems = int(indexedCount)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// 2. Получаем данные из источника
	items, err := dataSource.GetItems(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("failed to get items from data source: %w", err)
	}

	result.TotalItems = len(items)

	if len(items) == 0 {
		log.Printf("[VectorLoader] No items found for %s (category: %s)", contentType, category)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	log.Printf("[VectorLoader] Found %d items to index", len(items))

	// 3. Получаем стратегию подготовки
	strat, err := strategy.GetStrategy(contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy for %s: %w", contentType, err)
	}

	// 4. Подготавливаем документы для векторизации
	documents := make([]*models.VectorDocument, 0, len(items))

	for _, item := range items {
		itemID := dataSource.GetItemID(item)
		itemName := dataSource.GetItemName(item)

		// Валидация
		if err := strat.Validate(item); err != nil {
			result.FailedItems++
			result.Errors = append(result.Errors, fmt.Sprintf("validation failed for %s: %v", itemName, err))
			continue
		}

		// Подготовка контента
		content, err := strat.PrepareContent(item)
		if err != nil {
			result.FailedItems++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to prepare content for %s: %v", itemName, err))
			continue
		}

		// Извлечение метаданных
		metadata, err := strat.ExtractMetadata(item)
		if err != nil {
			result.FailedItems++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to extract metadata for %s: %v", itemName, err))
			continue
		}

		// Создаем векторный документ
		doc := models.NewVectorDocument(itemID, content, contentType)
		if category != "" {
			doc.WithCategory(category)
		}
		doc.Metadata = metadata

		documents = append(documents, doc)
	}

	if len(documents) == 0 {
		log.Printf("[VectorLoader] No valid documents to index")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	log.Printf("[VectorLoader] Prepared %d documents for indexing", len(documents))

	// 5. Batch индексация в Weaviate
	stats, err := l.vectorRepo.BatchCreate(ctx, documents)
	if err != nil {
		return nil, fmt.Errorf("batch indexing failed: %w", err)
	}

	result.IndexedItems = stats.Succeeded
	result.FailedItems += stats.Failed
	result.Errors = append(result.Errors, stats.Errors...)
	result.Duration = time.Since(startTime)

	log.Printf("[VectorLoader] Indexing complete: %d indexed, %d failed, duration: %v",
		result.IndexedItems, result.FailedItems, result.Duration)

	return result, nil
}

// RebuildIndex пересоздает индекс для типа контента и категории
func (l *VectorLoader) RebuildIndex(
	ctx context.Context,
	dataSource DataSource,
	contentType models.ContentType,
	category string,
) (*LoadResult, error) {
	log.Printf("[VectorLoader] Rebuilding index for %s (category: %s)", contentType, category)

	// 1. Удаляем существующие документы
	err := l.vectorRepo.DeleteByContentType(ctx, contentType, category)
	if err != nil {
		return nil, fmt.Errorf("failed to delete existing documents: %w", err)
	}

	log.Printf("[VectorLoader] Deleted existing documents for %s/%s", contentType, category)

	// 2. Загружаем заново
	return l.LoadFromDataSource(ctx, dataSource, contentType, category)
}

// GetIndexStatistics возвращает статистику индексации для указанных типов контента
func (l *VectorLoader) GetIndexStatistics(ctx context.Context, contentTypes []models.ContentType) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	for _, ct := range contentTypes {
		count, err := l.vectorRepo.CountByContentType(ctx, ct, "")
		if err != nil {
			return nil, fmt.Errorf("failed to count %s: %w", ct, err)
		}
		stats[fmt.Sprintf("indexed_%s", ct)] = count
	}

	return stats, nil
}

// GetTotalCount возвращает общее количество документов
func (l *VectorLoader) GetTotalCount(ctx context.Context) (int64, error) {
	return l.vectorRepo.CountByContentType(ctx, "", "")
}
