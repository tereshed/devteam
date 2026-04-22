package vectordb

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/devteam/backend/pkg/vectordb/schema"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"golang.org/x/sync/singleflight"
)

// Client представляет клиент для работы с Weaviate
type Client struct {
	weaviate *weaviate.Client
	config   *Config
	cache    sync.Map
	sfGroup  singleflight.Group
}

// NewClient создает новый клиент для Weaviate
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	if cfg.Scheme == "" {
		cfg.Scheme = "http" // Значение по умолчанию
	}

	// Создание конфигурации Weaviate клиента
	config := weaviate.Config{
		Host:   cfg.Host,
		Scheme: cfg.Scheme,
	}

	// Инициализация клиента
	client, err := weaviate.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create weaviate client: %w", err)
	}

	return &Client{
		weaviate: client,
		config:   cfg,
		cache:    sync.Map{},
		sfGroup:  singleflight.Group{},
	}, nil
}

// GetClassName возвращает имя коллекции для проекта с валидацией UUID.
// Формат: DevTeam_Project_{uuid_without_dashes}
func (c *Client) GetClassName(projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("projectID cannot be empty")
	}

	parsedUUID, err := uuid.Parse(projectID)
	if err != nil {
		return "", fmt.Errorf("invalid projectID (must be UUID): %w", err)
	}

	// Weaviate требует, чтобы имя класса начиналось с заглавной буквы
	// и содержало только алфавитно-цифровые символы.
	cleanID := strings.ReplaceAll(parsedUUID.String(), "-", "")
	return fmt.Sprintf("DevTeam_Project_%s", cleanID), nil
}

// EnsureCollection проверяет существование коллекции и создает её при необходимости.
// Использует singleflight для предотвращения Race Conditions и кэширование для минимизации запросов.
func (c *Client) EnsureCollection(ctx context.Context, projectID string) (string, error) {
	className, err := c.GetClassName(projectID)
	if err != nil {
		return "", err
	}

	// 1. In-memory Cache check
	if _, ok := c.cache.Load(projectID); ok {
		return className, nil
	}

	// 2. Singleflight + Context support
	// Используем context.WithoutCancel(ctx) для выполнения операции создания схемы,
	// чтобы отмена контекста вызывающей стороны не прерывала атомарную операцию для других.
	detachedCtx := context.WithoutCancel(ctx)

	ch := c.sfGroup.DoChan(className, func() (interface{}, error) {
		// Повторная проверка Weaviate (на случай если кэш был пуст, но класс уже создан другой горутиной)
		exists, err := c.weaviate.Schema().ClassExistenceChecker().WithClassName(className).Do(detachedCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check class existence: %w", err)
		}

		if !exists {
			class := schema.GetDocumentClass(className)
			err = c.weaviate.Schema().ClassCreator().WithClass(class).Do(detachedCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to create class %s: %w", className, err)
			}
		}

		return nil, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.Err == nil {
			c.cache.Store(projectID, true)
		}
		return className, res.Err
	}
}

// DeleteCollection удаляет коллекцию проекта из Weaviate.
func (c *Client) DeleteCollection(ctx context.Context, projectID string) error {
	className, err := c.GetClassName(projectID)
	if err != nil {
		return err
	}

	// Гарантированно удаляем из кэша, даже если Weaviate вернет ошибку (например, 404)
	defer c.cache.Delete(projectID)

	err = c.weaviate.Schema().ClassDeleter().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete collection %s: %w", className, err)
	}

	return nil
}

// HealthCheck проверяет доступность Weaviate
func (c *Client) HealthCheck(ctx context.Context) error {
	// Получаем meta информацию для проверки доступности
	meta, err := c.weaviate.Misc().MetaGetter().Do(ctx)
	if err != nil {
		return fmt.Errorf("weaviate health check failed: %w", err)
	}

	if meta == nil {
		return fmt.Errorf("weaviate returned nil meta")
	}

	return nil
}

// GetClient возвращает нативный Weaviate клиент для продвинутых операций
func (c *Client) GetClient() *weaviate.Client {
	return c.weaviate
}

// Close закрывает соединение с Weaviate (если необходимо)
func (c *Client) Close() error {
	// Weaviate Go client не требует явного закрытия соединения
	return nil
}

