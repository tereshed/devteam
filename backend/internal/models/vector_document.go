package models

import (
	"time"
)

// ContentType определяет тип контента в векторной базе
// В конкретном проекте определите свои типы контента
type ContentType string

// Примеры типов контента (замените на свои в реальном проекте):
// const (
//     ContentTypeArticle  ContentType = "article"
//     ContentTypeProduct  ContentType = "product"
//     ContentTypeUser     ContentType = "user"
//     ContentTypeFAQ      ContentType = "faq"
// )

// IsValid проверяет валидность типа контента
// ContentType валиден если он не пустой
func (ct ContentType) IsValid() bool {
	return ct != ""
}

// String возвращает строковое представление типа контента
func (ct ContentType) String() string {
	return string(ct)
}

// VectorDocument представляет документ в векторной базе данных (Weaviate)
// Это универсальная структура для любого типа контента
type VectorDocument struct {
	// ID документа в Weaviate (генерируется автоматически)
	ID string `json:"id"`

	// ContentID - ссылка на ID записи в основной БД (например, PostgreSQL/YugabyteDB)
	// Позволяет связать векторный документ с исходными данными
	ContentID string `json:"content_id" validate:"required"`

	// Content - текст для векторизации
	// Это поле будет преобразовано в вектор моделью Transformers
	Content string `json:"content" validate:"required,min=1"`

	// ContentType - тип контента для фильтрации при поиске
	// Определите свои типы в конкретном проекте
	ContentType ContentType `json:"content_type" validate:"required"`

	// Category - опциональная категория для дополнительной фильтрации
	Category string `json:"category,omitempty"`

	// Tags - теги для фильтрации
	Tags []string `json:"tags,omitempty"`

	// Metadata - дополнительные данные в формате JSON
	// Используйте для хранения любых дополнительных полей
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Временные метки
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewVectorDocument создает новый документ для векторной базы
func NewVectorDocument(contentID, content string, contentType ContentType) *VectorDocument {
	now := time.Now()
	return &VectorDocument{
		ContentID:   contentID,
		Content:     content,
		ContentType: contentType,
		Metadata:    make(map[string]interface{}),
		Tags:        []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// WithCategory устанавливает категорию (fluent interface)
func (vd *VectorDocument) WithCategory(category string) *VectorDocument {
	vd.Category = category
	return vd
}

// WithTags устанавливает теги (fluent interface)
func (vd *VectorDocument) WithTags(tags ...string) *VectorDocument {
	vd.Tags = tags
	return vd
}

// SetMetadata устанавливает значение в metadata
func (vd *VectorDocument) SetMetadata(key string, value interface{}) {
	if vd.Metadata == nil {
		vd.Metadata = make(map[string]interface{})
	}
	vd.Metadata[key] = value
}

// GetMetadata получает значение из metadata
func (vd *VectorDocument) GetMetadata(key string) (interface{}, bool) {
	if vd.Metadata == nil {
		return nil, false
	}
	val, exists := vd.Metadata[key]
	return val, exists
}

// AddTag добавляет тег
func (vd *VectorDocument) AddTag(tag string) {
	if vd.Tags == nil {
		vd.Tags = []string{}
	}
	vd.Tags = append(vd.Tags, tag)
}

// HasTag проверяет наличие тега
func (vd *VectorDocument) HasTag(tag string) bool {
	for _, t := range vd.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
