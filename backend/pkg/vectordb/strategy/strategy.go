package strategy

import (
	"fmt"
	"strings"

	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

// PreparationStrategy определяет интерфейс для подготовки контента к векторизации
// Реализуйте этот интерфейс для каждого типа контента в вашем проекте
type PreparationStrategy interface {
	// PrepareContent подготавливает текст для векторизации
	// Возвращает строку, которая будет преобразована в вектор
	PrepareContent(data interface{}) (string, error)

	// ExtractMetadata извлекает метаданные из данных для хранения в Weaviate
	// Метаданные используются для фильтрации при поиске
	ExtractMetadata(data interface{}) (map[string]interface{}, error)

	// Validate проверяет валидность данных перед индексацией
	Validate(data interface{}) error
}

// StrategyRegistry реестр стратегий для разных типов контента
// Используйте для регистрации своих стратегий
type StrategyRegistry struct {
	strategies map[models.ContentType]PreparationStrategy
}

// NewStrategyRegistry создает новый реестр стратегий
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{
		strategies: make(map[models.ContentType]PreparationStrategy),
	}
}

// Register регистрирует стратегию для типа контента
func (r *StrategyRegistry) Register(contentType models.ContentType, strategy PreparationStrategy) {
	r.strategies[contentType] = strategy
}

// Get возвращает стратегию для типа контента
func (r *StrategyRegistry) Get(contentType models.ContentType) (PreparationStrategy, error) {
	strategy, exists := r.strategies[contentType]
	if !exists {
		return nil, fmt.Errorf("no strategy registered for content type: %s", contentType)
	}
	return strategy, nil
}

// DefaultRegistry глобальный реестр стратегий (опционально)
var DefaultRegistry = NewStrategyRegistry()

// RegisterStrategy регистрирует стратегию в глобальном реестре
func RegisterStrategy(contentType models.ContentType, strategy PreparationStrategy) {
	DefaultRegistry.Register(contentType, strategy)
}

// GetStrategy возвращает стратегию из глобального реестра
func GetStrategy(contentType models.ContentType) (PreparationStrategy, error) {
	return DefaultRegistry.Get(contentType)
}

// ====================================================================================
// ПРИМЕР: GenericStrategy - универсальная стратегия для простых случаев
// Используйте как базу для создания своих стратегий
// ====================================================================================

// GenericStrategy универсальная стратегия для map[string]interface{} данных
type GenericStrategy struct {
	// ContentField имя поля с основным контентом для векторизации
	ContentField string
	// IDField имя поля с ID
	IDField string
	// RequiredFields обязательные поля для валидации
	RequiredFields []string
	// MetadataFields поля для извлечения в metadata
	MetadataFields []string
}

// NewGenericStrategy создает новую универсальную стратегию
func NewGenericStrategy(contentField, idField string) *GenericStrategy {
	return &GenericStrategy{
		ContentField:   contentField,
		IDField:        idField,
		RequiredFields: []string{idField, contentField},
		MetadataFields: []string{},
	}
}

// WithRequiredFields добавляет обязательные поля (fluent interface)
func (s *GenericStrategy) WithRequiredFields(fields ...string) *GenericStrategy {
	s.RequiredFields = append(s.RequiredFields, fields...)
	return s
}

// WithMetadataFields добавляет поля для metadata (fluent interface)
func (s *GenericStrategy) WithMetadataFields(fields ...string) *GenericStrategy {
	s.MetadataFields = append(s.MetadataFields, fields...)
	return s
}

// PrepareContent извлекает контент из данных
func (s *GenericStrategy) PrepareContent(data interface{}) (string, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("GenericStrategy expects map[string]interface{}, got %T", data)
	}

	content, ok := dataMap[s.ContentField].(string)
	if !ok {
		return "", fmt.Errorf("field '%s' not found or not a string", s.ContentField)
	}

	return strings.TrimSpace(content), nil
}

// ExtractMetadata извлекает метаданные из данных
func (s *GenericStrategy) ExtractMetadata(data interface{}) (map[string]interface{}, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("GenericStrategy expects map[string]interface{}, got %T", data)
	}

	metadata := make(map[string]interface{})

	for _, field := range s.MetadataFields {
		if value, exists := dataMap[field]; exists {
			metadata[field] = value
		}
	}

	return metadata, nil
}

// Validate проверяет наличие обязательных полей
func (s *GenericStrategy) Validate(data interface{}) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("GenericStrategy expects map[string]interface{}, got %T", data)
	}

	for _, field := range s.RequiredFields {
		value, exists := dataMap[field]
		if !exists {
			return fmt.Errorf("required field '%s' is missing", field)
		}
		// Проверяем что строковые поля не пустые
		if strValue, isString := value.(string); isString && strings.TrimSpace(strValue) == "" {
			return fmt.Errorf("required field '%s' is empty", field)
		}
	}

	return nil
}

// ====================================================================================
// ПРИМЕР: Использование в вашем проекте
// ====================================================================================
//
// 1. Определите типы контента:
//
//    const (
//        ContentTypeArticle models.ContentType = "article"
//        ContentTypeProduct models.ContentType = "product"
//    )
//
// 2. Создайте и зарегистрируйте стратегии:
//
//    func init() {
//        // Простая стратегия для статей
//        articleStrategy := NewGenericStrategy("body", "id").
//            WithRequiredFields("title").
//            WithMetadataFields("author", "category", "published_at")
//
//        RegisterStrategy(ContentTypeArticle, articleStrategy)
//
//        // Или создайте свою стратегию, реализовав интерфейс PreparationStrategy
//        RegisterStrategy(ContentTypeProduct, &ProductStrategy{})
//    }
//
// 3. Используйте:
//
//    strategy, err := GetStrategy(ContentTypeArticle)
//    if err != nil {
//        return err
//    }
//
//    content, err := strategy.PrepareContent(articleData)
//    metadata, err := strategy.ExtractMetadata(articleData)
//
// ====================================================================================
