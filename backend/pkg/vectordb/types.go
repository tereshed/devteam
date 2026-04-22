package vectordb

import "github.com/devteam/backend/internal/models"

// Config содержит конфигурацию для подключения к Weaviate
type Config struct {
	Host   string // Например: "weaviate:8080" или "localhost:8081"
	Scheme string // "http" или "https"
}

// SearchParams определяет параметры поиска в векторной базе
type SearchParams struct {
	// ProjectID - ID проекта (обязательно для изоляции коллекций)
	ProjectID string

	// Query - поисковый запрос
	Query string

	// ContentTypes - фильтр по типам контента
	ContentTypes []models.ContentType

	// Category - фильтр по категории (опционально)
	Category string

	// Tags - фильтр по тегам (опционально)
	Tags []string

	// ContentIDs - фильтр по конкретным ID контента (опционально)
	ContentIDs []string

	// Limit - максимальное количество результатов
	Limit int

	// Alpha - баланс между keyword (BM25) и semantic (vector) поиском
	// 0.0 = только keyword (BM25)
	// 1.0 = только semantic (vector)
	// 0.5 = гибридный (50/50)
	Alpha float32
}

// SearchResult представляет результат поиска
type SearchResult struct {
	// VectorID - ID документа в Weaviate
	VectorID string `json:"vector_id"`

	// ContentID - ID записи в основной БД
	ContentID string `json:"content_id"`

	// ContentType - тип контента
	ContentType models.ContentType `json:"content_type"`

	// Category - категория контента
	Category string `json:"category,omitempty"`

	// Content - текст документа
	Content string `json:"content"`

	// Score - BM25 score (relevance score)
	Score float32 `json:"score"`

	// Distance - косинусное расстояние для векторного поиска
	// 0 = идентичные, 1 = противоположные
	Distance float32 `json:"distance"`

	// Metadata - дополнительные данные
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IndexStats содержит статистику индексации
type IndexStats struct {
	// TotalProcessed - всего обработано документов
	TotalProcessed int `json:"total_processed"`

	// Succeeded - успешно проиндексировано
	Succeeded int `json:"succeeded"`

	// Failed - не удалось проиндексировать
	Failed int `json:"failed"`

	// Errors - список ошибок (если есть)
	Errors []string `json:"errors,omitempty"`
}
