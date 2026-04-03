package promptsloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"gopkg.in/yaml.v3"
	"gorm.io/datatypes"
)

// PromptConfig структура для парсинга YAML
type PromptConfig struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Template    string                 `yaml:"template"`
	JSONSchema  map[string]interface{} `yaml:"json_schema"`
	IsActive    bool                   `yaml:"is_active"`
}

// Loader отвечает за загрузку промптов из файлов
type Loader struct {
	repo repository.PromptRepository
}

// New создает новый загрузчик
func New(repo repository.PromptRepository) *Loader {
	return &Loader{repo: repo}
}

// LoadFromDir загружает все .yaml файлы из директории и сохраняет их в БД
func (l *Loader) LoadFromDir(ctx context.Context, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) != ".yaml" && filepath.Ext(file.Name()) != ".yml" {
			continue
		}

		path := filepath.Join(dirPath, file.Name())
		if err := l.loadAndSavePrompt(ctx, path); err != nil {
			return fmt.Errorf("failed to load prompt from %s: %w", file.Name(), err)
		}
	}

	return nil
}

func (l *Loader) loadAndSavePrompt(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config PromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse yaml: %w", err)
	}

	if config.Name == "" {
		return fmt.Errorf("prompt name is required")
	}

	// Конвертируем map в datatypes.JSON для GORM
	// Используем пустой JSON если схема не задана
	var jsonSchema datatypes.JSON
	if len(config.JSONSchema) > 0 {
		jsonSchema = datatypes.JSON(mustMarshalJSON(config.JSONSchema))
	}

	prompt := &models.Prompt{
		Name:        config.Name,
		Description: config.Description,
		Template:    config.Template,
		JSONSchema:  jsonSchema,
		IsActive:    config.IsActive,
	}

	// Если ID указан, парсим его и устанавливаем
	if config.ID != "" {
		id, err := uuid.Parse(config.ID)
		if err != nil {
			return fmt.Errorf("invalid prompt id %s: %w", config.ID, err)
		}
		prompt.ID = id
	}

	// Используем Upsert (Save)
	return l.repo.Upsert(ctx, prompt)
}

// Вспомогательная функция для маршалинга JSON (используется только для валидных данных из YAML)
func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}") // Fallback на пустой JSON, хотя этого быть не должно с данными из yaml unmarshal
	}
	return data
}

