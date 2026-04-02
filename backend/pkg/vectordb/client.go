package vectordb

import (
	"context"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
)

// Client представляет клиент для работы с Weaviate
type Client struct {
	weaviate *weaviate.Client
	config   *Config
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
	}, nil
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

