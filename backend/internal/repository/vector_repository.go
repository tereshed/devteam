package repository

import (
	"context"
	"errors"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/vectordb"
)

var (
	ErrVectorDocumentNotFound = errors.New("vector document not found")
)

// VectorRepository определяет интерфейс для работы с векторной базой данных
type VectorRepository interface {
	// CRUD операции
	Create(ctx context.Context, projectID string, doc *models.VectorDocument) (string, error)
	Get(ctx context.Context, projectID string, id string) (*models.VectorDocument, error)
	Update(ctx context.Context, projectID string, id string, doc *models.VectorDocument) error
	Delete(ctx context.Context, projectID string, id string) error

	// Batch операции
	BatchCreate(ctx context.Context, projectID string, docs []*models.VectorDocument) (*vectordb.IndexStats, error)
	DeleteByContentID(ctx context.Context, projectID string, contentID string) error
	DeleteByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) error

	// Поиск
	Search(ctx context.Context, projectID string, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error)
	SemanticSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error)
	KeywordSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error)

	// Утилиты
	CountByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) (int64, error)
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
func (r *vectorRepository) Create(ctx context.Context, projectID string, doc *models.VectorDocument) (string, error) {
	return r.client.Create(ctx, projectID, doc)
}

// Get получает документ по ID
func (r *vectorRepository) Get(ctx context.Context, projectID string, id string) (*models.VectorDocument, error) {
	doc, err := r.client.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Update обновляет документ
func (r *vectorRepository) Update(ctx context.Context, projectID string, id string, doc *models.VectorDocument) error {
	return r.client.Update(ctx, projectID, id, doc)
}

// Delete удаляет документ
func (r *vectorRepository) Delete(ctx context.Context, projectID string, id string) error {
	return r.client.Delete(ctx, projectID, id)
}

// BatchCreate создает несколько документов за один запрос
func (r *vectorRepository) BatchCreate(ctx context.Context, projectID string, docs []*models.VectorDocument) (*vectordb.IndexStats, error) {
	return r.client.BatchCreate(ctx, projectID, docs)
}

// DeleteByContentID удаляет документы по contentId
func (r *vectorRepository) DeleteByContentID(ctx context.Context, projectID string, contentID string) error {
	return r.client.DeleteByContentID(ctx, projectID, contentID)
}

// DeleteByContentType удаляет все документы определенного типа
func (r *vectorRepository) DeleteByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) error {
	return r.client.DeleteByContentType(ctx, projectID, contentType, category)
}

// Search выполняет поиск с заданными параметрами
func (r *vectorRepository) Search(ctx context.Context, projectID string, params *vectordb.SearchParams) ([]*vectordb.SearchResult, error) {
	if params == nil {
		params = &vectordb.SearchParams{
			Limit: 10,
			Alpha: 0.5, // Гибридный поиск
		}
	}
	params.ProjectID = projectID
	return r.client.Search(ctx, *params)
}

// SemanticSearch выполняет только векторный поиск
func (r *vectorRepository) SemanticSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	return r.client.SemanticSearch(ctx, projectID, query, category, limit)
}

// KeywordSearch выполняет только BM25 поиск
func (r *vectorRepository) KeywordSearch(ctx context.Context, projectID string, query string, category string, limit int) ([]*vectordb.SearchResult, error) {
	return r.client.KeywordSearch(ctx, projectID, query, category, limit)
}

// CountByContentType возвращает количество документов определенного типа и категории
func (r *vectorRepository) CountByContentType(ctx context.Context, projectID string, contentType models.ContentType, category string) (int64, error) {
	return r.client.CountByContentType(ctx, projectID, contentType, category)
}
