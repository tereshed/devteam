package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	LLM      LLMConfig
	Admin    AdminConfig
	MCP      MCPConfig
}

// MCPConfig содержит конфигурацию MCP (Model Context Protocol) сервера
type MCPConfig struct {
	Enabled        bool   // Включён ли MCP-сервер
	Port           string // Порт MCP-сервера (по умолчанию "8081")
	PublicURL      string // Публичный URL для конфигов клиентов (Cursor, Claude и т.д.)
	MaxPromptRunes int    // Макс. длина prompt в символах (рунах), по умолчанию 100000
	MaxTokensLimit int    // Верхняя граница max_tokens, по умолчанию 32768
	MaxInputRunes  int    // Макс. длина входных данных workflow в рунах, по умолчанию 50000
}

// ServerConfig содержит конфигурацию HTTP сервера
type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig содержит конфигурацию базы данных
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN возвращает строку подключения к YugabyteDB (PostgreSQL-compatible)
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// JWTConfig содержит конфигурацию JWT токенов
type JWTConfig struct {
	SecretKey          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5433"),
			User:            getEnv("DB_USER", "yugabyte"),
			Password:        getEnv("DB_PASSWORD", "yugabyte"),
			DBName:          getEnv("DB_NAME", "yugabyte"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		JWT: JWTConfig{
			SecretKey:          getEnv("JWT_SECRET_KEY", "change-me-in-production"),
			AccessTokenExpiry:  getDurationEnv("JWT_ACCESS_TOKEN_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: getDurationEnv("JWT_REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		},
		LLM: LLMConfig{
			DefaultProvider:  getEnv("LLM_PROVIDER", "openai"),
			OpenRouterAPIKey: getEnv("OPENROUTER_API_KEY", getEnv("OPENROUTER_KEY", getEnv("LLM_API_KEY", ""))), // Если нет специфичного ключа, берем общий
			OpenAI: ProviderConfig{
				APIKey:  getEnv("OPENAI_API_KEY", getEnv("LLM_API_KEY", "")), // Fallback to LLM_API_KEY for backward compatibility
				BaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
				Model:   getEnv("OPENAI_MODEL", "gpt-4o"),
			},
			Anthropic: ProviderConfig{
				APIKey:  getEnv("ANTHROPIC_API_KEY", ""),
				BaseURL: getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
				Model:   getEnv("ANTHROPIC_MODEL", "claude-3-5-sonnet-20240620"),
			},
			Gemini: ProviderConfig{
				APIKey:  getEnv("GEMINI_API_KEY", ""),
				BaseURL: getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com"),
				Model:   getEnv("GEMINI_MODEL", "gemini-1.5-pro"),
			},
			Deepseek: ProviderConfig{
				APIKey:  getEnv("DEEPSEEK_API_KEY", ""),
				BaseURL: getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
				Model:   getEnv("DEEPSEEK_MODEL", "deepseek-chat"),
			},
			Qwen: ProviderConfig{
				APIKey:  getEnv("QWEN_API_KEY", ""),
				BaseURL: getEnv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
				Model:   getEnv("QWEN_MODEL", "qwen-turbo"),
			},
		},
		Admin: AdminConfig{
			Email:    getEnv("ADMIN_EMAIL", ""),
			Password: getEnv("ADMIN_PASSWORD", ""),
		},
		MCP: MCPConfig{
			Enabled:        getBoolEnv("MCP_ENABLED", false),
			Port:           getEnv("MCP_PORT", "8081"),
			PublicURL:      getEnv("MCP_PUBLIC_URL", ""),
			MaxPromptRunes: getIntEnv("MCP_MAX_PROMPT_RUNES", 100_000),
			MaxTokensLimit: getIntEnv("MCP_MAX_TOKENS_LIMIT", 32_768),
			MaxInputRunes:  getIntEnv("MCP_MAX_INPUT_RUNES", 50_000),
		},
	}

	// Валидация MCP-конфига
	if cfg.MCP.Enabled {
		// Проверяем, что порт — валидное число
		if mcpPort, err := strconv.Atoi(cfg.MCP.Port); err != nil || mcpPort < 1 || mcpPort > 65535 {
			return nil, fmt.Errorf("MCP_PORT must be a valid port number (1-65535), got: %s", cfg.MCP.Port)
		}

		// Проверяем конфликт с основным портом сервера
		if cfg.MCP.Port == cfg.Server.Port {
			return nil, fmt.Errorf("MCP_PORT (%s) must differ from SERVER_PORT (%s)", cfg.MCP.Port, cfg.Server.Port)
		}

		// PublicURL обязателен в production
		if cfg.MCP.PublicURL == "" {
			if os.Getenv("ENV") == "production" {
				return nil, fmt.Errorf("MCP_PUBLIC_URL must be set in production (e.g. https://your-domain.com/mcp)")
			}
			// Для разработки формируем URL по умолчанию
			cfg.MCP.PublicURL = fmt.Sprintf("http://localhost:%s/mcp", cfg.MCP.Port)
		}
	}

	// Предупреждение, если используется дефолтный ключ
	if cfg.JWT.SecretKey == "change-me-in-production" {
		// В production это должно быть ошибкой, но для разработки разрешаем
		// Можно добавить проверку окружения через переменную ENV=production
		if os.Getenv("ENV") == "production" {
			return nil, fmt.Errorf("JWT_SECRET_KEY must be set in production")
		}
	}

	return cfg, nil
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv получает целочисленную переменную окружения или возвращает значение по умолчанию
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getBoolEnv получает булеву переменную окружения или возвращает значение по умолчанию
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getDurationEnv получает переменную окружения как duration или возвращает значение по умолчанию
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// LLMConfig содержит конфигурацию для LLM провайдеров
type LLMConfig struct {
	DefaultProvider  string
	OpenRouterAPIKey string // Специальный ключ для OpenRouter API (получение моделей)
	OpenAI           ProviderConfig
	Anthropic        ProviderConfig
	Gemini           ProviderConfig
	Deepseek         ProviderConfig
	Qwen             ProviderConfig
}

// ProviderConfig содержит конфигурацию конкретного провайдера
type ProviderConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// AdminConfig содержит конфигурацию для администратора
type AdminConfig struct {
	Email    string
	Password string
}
