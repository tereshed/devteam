package repository

import (
	"context"
	"errors"

	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/pkg/vectordb"
)

var (
	ErrVectorDocumentNotFound = errors.New("vector document not found")
)

// VectorRepository определяет интерфейс для работы с векторной базой данных
type VectorRepository interface {
	// CRUD операции
	Create(ctx context.Context, doc *models.VectorDocument) (string, error)
	Get(ctx context.Context, id string) (*models.VectorDocument, error)
	Update(ctx context.Context, id string, doc *models.VectorDocument) error
	Delete(ctx context.Context, id string) error

	// Batch операции
	BatchCreate(ctx context.Context, docs []*models.VectorDocument) (*vectordb.IndexStats, error)
	DeleteByContentID(ctx context.Context, contentID string) error
	DeleteByContentType(ctx context.Context, contentType models.ContentType, category string) error

	// Поиск
	Search(ctx context.Context, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error)
	SemanticSearch(ctx context.Context, query string, category string, limit int) ([]*vectordb.SearchResult, error)
	KeywordSearch(ctx context.Context, query string, category string, limit int) ([]*vectordb.SearchResult, error)

	// Утилиты
	CountByContentType(ctx context.Context, contentType models.ContentType, category string) (int64, error)
}

// vectorRepository реализация VectorRepository
type vectorRepository struct {
	client *vectordb.Client
}

// NewVectorRepository создает новый репозиторий для векторной БД
func NewVectorRepository(client *vectordb.Client) VectorRepository {
	return &vectorRepository{
		client: client,
	}
}

// Create создает документ в векторной базе
func (r *vectorRepository) Create(ctx context.Context, doc *models.VectorDocument) (string, error) {
	return r.client.Create(ctx, doc)
}

// Get получает документ по ID
func (r *vectorRepository) Get(ctx context.Context, id string) (*models.VectorDocument, error) {
	doc, err := r.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Update обновляет документ
func (r *vectorRepository) Update(ctx context.Context, id string, doc *models.VectorDocument) error {
	return r.client.Update(ctx, id, doc)
}

// Delete удаляет документ
func (r *vectorRepository) Delete(ctx context.Context, id string) error {
	return r.client.Delete(ctx, id)
}

// BatchCreate создает несколько документов за один запрос
func (r *vectorRepository) BatchCreate(ctx context.Context, docs []*models.VectorDocument) (*vectordb.IndexStats, error) {
	return r.client.BatchCreate(ctx, docs)
}

// DeleteByContentID удаляет документы по contentId
func (r *vectorRepository) DeleteByContentID(ctx context.Context, contentID string) error {
	return r.client.DeleteByContentID(ctx, contentID)
}

// DeleteByContentType удаляет все документы определенного типа
func (r *vectorRepository) DeleteByContentType(ctx context.Context, contentType models.ContentType, category string) error {
	return r.client.DeleteByContentType(ctx, contentType, category)
}

// Search выполняет поиск с заданными параметрами
func (r *vectorRepository) Search(ctx context.Context, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error) {
	if params == nil {
		params = &vectordb.SearchParams{
			Limit: 10,
			Alpha: 0.5, // Гибридный поиск
		}
	}
	return r.client.Search(ctx, *params)
}

// SemanticSearch выполняет только векторный поиск
func (r *vectorRepository) SemanticSearch(ctx context.Context, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	return r.client.SemanticSearch(ctx, query, category, limit)
}

// KeywordSearch выполняет только BM25 поиск
func (r *vectorRepository) KeywordSearch(ctx context.Context, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	return r.client.KeywordSearch(ctx, query, category, limit)
}

// CountByContentType возвращает количество документов определенного типа и категории
func (r *vectorRepository) CountByContentType(ctx context.Context, contentType models.ContentType, category string) (int64, error) {
	return r.client.CountByContentType(ctx, contentType, category)
}
