package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	// Environment — значение ENV после strings.TrimSpace и ToLower (пусто, если переменная не задана).
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	JWT        JWTConfig
	LLM        LLMConfig
	Admin      AdminConfig
	MCP        MCPConfig
	Encryption EncryptionConfig
	Git        GitConfig
	// Sandbox — лимиты и таймауты sandbox (SANDBOX_*), задача 5.10.
	Sandbox SandboxConfig
	// WebSocket — конфигурация WebSocket (WS_*), задача 7.7.
	WebSocket WebSocketConfig
	// WorkflowWorkerEnabled — фоновый worker, раз в секунду ищет pending/running executions.
	WorkflowWorkerEnabled bool
	// ClaudeCodeOAuth — настройки OAuth-провайдера Claude Code (Sprint 15.12).
	ClaudeCodeOAuth ClaudeCodeOAuthConfig
	// AntigravityOAuth — настройки OAuth-провайдера Antigravity.
	AntigravityOAuth AntigravityOAuthConfig

	// GitHubOAuth — настройки OAuth-провайдера GitHub (UI Refactoring Stage 3a).
	// Пустой ClientID отключает фичу — хендлеры вернут 503.
	GitHubOAuth GitHubOAuthAppConfig

	// GitLabOAuth — настройки OAuth-провайдера GitLab.com (UI Refactoring Stage 3a).
	// Пустой ClientID отключает shared-flow (self-hosted BYO остаётся доступным).
	GitLabOAuth GitLabOAuthAppConfig

	// Weaviate — конфигурация векторной базы данных Weaviate.
	Weaviate WeaviateConfig

	// Redis — low-latency wakeup воркеров (Pub/Sub) и кросс-нодовая доставка
	// WebSocket-сообщений при горизонтальном масштабировании. Пустой URL → одноинстансный
	// режим (in-memory шина, polling-only воркеры).
	Redis RedisConfig

	// AutoMigrate — применять ли goose-миграции на старте процесса (AUTO_MIGRATE, default true).
	// В multi-instance деплое выставить false и накатывать миграции отдельным one-shot job
	// (cmd/migrate), чтобы goose.Up не бежал на каждой реплике одновременно.
	AutoMigrate bool
}

// RedisConfig — Redis для low-latency wakeup воркеров и кросс-нодовой доставки
// WebSocket-сообщений (горизонтальное масштабирование, см. ws.ClusterBridge).
// Пустой URL → одноинстансный режим. Required форсирует наличие Redis (по умолчанию
// в production), чтобы multi-instance деплой не стартовал в режиме без fan-out'а.
type RedisConfig struct {
	URL      string
	Required bool
}

// WeaviateConfig содержит конфигурацию для подключения к Weaviate
type WeaviateConfig struct {
	Host   string
	Scheme string
}

// GitHubOAuthAppConfig — env GITHUB_OAUTH_CLIENT_ID / SECRET / SCOPES.
type GitHubOAuthAppConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
}

// GitLabOAuthAppConfig — env GITLAB_OAUTH_CLIENT_ID / SECRET / SCOPES.
type GitLabOAuthAppConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
}

// ClaudeCodeOAuthConfig — env CLAUDE_CODE_OAUTH_*. Пустой ClientID отключает фичу
// (хендлеры вернут 503, MCP-инструменты не зарегистрируются).
type ClaudeCodeOAuthConfig struct {
	ClientID      string
	DeviceCodeURL string
	TokenURL      string
	RevokeURL     string
	Scopes        string
}

// AntigravityOAuthConfig — env ANTIGRAVITY_OAUTH_*. Пустой ClientID отключает фичу
type AntigravityOAuthConfig struct {
	ClientID     string
	ClientSecret string
	DeviceCodeURL string
	TokenURL      string
	RevokeURL     string
	Scopes        string
}

// WebSocketConfig содержит конфигурацию WebSocket
type WebSocketConfig struct {
	AllowedOrigins         []string      `env:"WS_ALLOWED_ORIGINS" envSeparator:","`
	MaxConnsPerUserProject int           `env:"WS_MAX_CONNS_PER_USER_PROJECT" envDefault:"5"`
	PingPeriod             time.Duration `env:"WS_PING_PERIOD" envDefault:"54s"`
	PongWait               time.Duration `env:"WS_PONG_WAIT" envDefault:"60s"`
}

// GitConfig — параметры работы с git при импорте/индексации.
type GitConfig struct {
	// ImportDir — каталог для клонов (GIT_IMPORT_DIR).
	ImportDir string
	// ProjectSyncCron — расписание периодической переиндексации проектов (GIT_PROJECT_SYNC_CRON).
	ProjectSyncCron string
}


// EncryptionConfig — ключ для AES-256-GCM (32 байта после декодирования ENCRYPTION_KEY).
type EncryptionConfig struct {
	Key []byte // пусто, если ENCRYPTION_KEY не задан; иначе ровно 32 байта из HEX (64 символа)
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

// IsProd — «боевое» окружение: ENV=production или prod (без учёта регистра при загрузке).
func (c *Config) IsProd() bool {
	switch c.Environment {
	case "production", "prod":
		return true
	default:
		return false
	}
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{
		Environment: normalizeEnvName(os.Getenv("ENV")),
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
			// Дефолты — самые дешёвые модели каждого провайдера. Пользователь может
			// перебить через ENV если нужна более жирная модель. См. audit cost-leak
			// Phase 2: на Sonnet 4.6 + gpt-4o уходило в 3-15× больше денег, чем нужно
			// для подавляющего большинства pipeline-задач.
			OpenAI: ProviderConfig{
				APIKey:  getEnv("OPENAI_API_KEY", getEnv("LLM_API_KEY", "")), // Fallback to LLM_API_KEY for backward compatibility
				BaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
				Model:   getEnv("OPENAI_MODEL", "gpt-4o-mini"), // $0.15/$0.60 vs gpt-4o $2.50/$10
			},
			Anthropic: ProviderConfig{
				APIKey:  getEnv("ANTHROPIC_API_KEY", ""),
				BaseURL: getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
				Model:   getEnv("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"), // $1/$5 vs Sonnet 4.6 $3/$15
			},
			Gemini: ProviderConfig{
				APIKey:  getEnv("GEMINI_API_KEY", ""),
				BaseURL: getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com"),
				Model:   getEnv("GEMINI_MODEL", "gemini-1.5-flash"), // $0.075/$0.30 vs 1.5-pro $1.25/$5
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
			// OpenRouter — глобальный провайдер для assistant/orchestrator/planner.
			// Введён после Phase 5 review: дешёвая v4-flash модель (~$0.0000001/M
			// токенов) ускоряет цепочку router-decisions в pipeline в разы по
			// сравнению с anthropic+haiku при сопоставимом качестве на простых
			// roвертикальных задачах. Эндпоинт совпадает с OpenAI-форматом,
			// поэтому FakeLLM перехватывает запросы тем же handler'ом, что и
			// /v1/chat/completions (см. test/featuresmoke/fakes/llm_server.go).
			OpenRouter: ProviderConfig{
				APIKey:  getEnv("OPENROUTER_API_KEY", getEnv("OPENROUTER_KEY", "")),
				BaseURL: getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
				Model:   getEnv("OPENROUTER_MODEL", "deepseek/deepseek-v4-flash"),
			},
			Zhipu: ProviderConfig{
				APIKey:  getEnv("ZHIPU_API_KEY", ""),
				BaseURL: getEnv("ZHIPU_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
				Model:   getEnv("ZHIPU_MODEL", "glm-4-plus"),
			},
			Antigravity: ProviderConfig{
				APIKey:  getEnv("ANTIGRAVITY_API_KEY", ""),
				BaseURL: getEnv("ANTIGRAVITY_BASE_URL", "https://api.antigravity.ai/v1"),
				Model:   getEnv("ANTIGRAVITY_MODEL", "antigravity-default"),
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
		Encryption: EncryptionConfig{},
		Git: GitConfig{
			ImportDir:       getEnv("GIT_IMPORT_DIR", "/tmp/devteam-import"),
			ProjectSyncCron: getEnv("GIT_PROJECT_SYNC_CRON", "*/10 * * * *"),
		},
		WorkflowWorkerEnabled: getBoolEnv("WORKFLOW_WORKER_ENABLED", true),
		ClaudeCodeOAuth: ClaudeCodeOAuthConfig{
			ClientID:      getEnv("CLAUDE_CODE_OAUTH_CLIENT_ID", ""),
			DeviceCodeURL: getEnv("CLAUDE_CODE_OAUTH_DEVICE_URL", "https://console.anthropic.com/v1/oauth/device"),
			TokenURL:      getEnv("CLAUDE_CODE_OAUTH_TOKEN_URL", "https://console.anthropic.com/v1/oauth/token"),
			RevokeURL:     getEnv("CLAUDE_CODE_OAUTH_REVOKE_URL", ""),
			Scopes:        getEnv("CLAUDE_CODE_OAUTH_SCOPES", "org:create_api_key user:profile user:inference"),
		},
		AntigravityOAuth: AntigravityOAuthConfig{
			ClientID:     getEnv("ANTIGRAVITY_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("ANTIGRAVITY_OAUTH_CLIENT_SECRET", ""),
			DeviceCodeURL: getEnv("ANTIGRAVITY_OAUTH_DEVICE_URL", "https://api.antigravity.ai/oauth/device"),
			TokenURL:      getEnv("ANTIGRAVITY_OAUTH_TOKEN_URL", "https://api.antigravity.ai/oauth/token"),
			RevokeURL:     getEnv("ANTIGRAVITY_OAUTH_REVOKE_URL", ""),
			Scopes:        getEnv("ANTIGRAVITY_OAUTH_SCOPES", "user:profile user:inference"),
		},
		GitHubOAuth: GitHubOAuthAppConfig{
			ClientID:     getEnv("GITHUB_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("GITHUB_OAUTH_CLIENT_SECRET", ""),
			Scopes:       getEnv("GITHUB_OAUTH_SCOPES", "repo read:user"),
		},
		GitLabOAuth: GitLabOAuthAppConfig{
			ClientID:     getEnv("GITLAB_OAUTH_CLIENT_ID", ""),
			ClientSecret: getEnv("GITLAB_OAUTH_CLIENT_SECRET", ""),
			Scopes:       getEnv("GITLAB_OAUTH_SCOPES", "api read_user read_repository write_repository"),
		},
		Weaviate: WeaviateConfig{
			Host:   getEnv("WEAVIATE_HOST", "localhost:8082"),
			Scheme: getEnv("WEAVIATE_SCHEME", "http"),
		},
	}

	encKeyRaw := strings.TrimSpace(getEnv("ENCRYPTION_KEY", ""))
	if encKeyRaw == "" && cfg.IsProd() {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be set in production")
	}
	if encKeyRaw != "" {
		key, err := DecodeEncryptionKeyHex(encKeyRaw)
		if err != nil {
			return nil, fmt.Errorf("ENCRYPTION_KEY: %w", err)
		}
		cfg.Encryption.Key = key
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
			if cfg.IsProd() {
				return nil, fmt.Errorf("MCP_PUBLIC_URL must be set in production (e.g. https://your-domain.com/mcp)")
			}
			// Для разработки формируем URL по умолчанию
			cfg.MCP.PublicURL = fmt.Sprintf("http://localhost:%s/mcp", cfg.MCP.Port)
		}
	}

	if cfg.JWT.SecretKey == "change-me-in-production" && cfg.IsProd() {
		return nil, fmt.Errorf("JWT_SECRET_KEY must be set in production")
	}

	cfg.WebSocket = WebSocketConfig{
		AllowedOrigins:         getSliceEnv("WS_ALLOWED_ORIGINS", ",", nil),
		MaxConnsPerUserProject: getIntEnv("WS_MAX_CONNS_PER_USER_PROJECT", 5),
		PingPeriod:             getDurationEnv("WS_PING_PERIOD", 54*time.Second),
		PongWait:               getDurationEnv("WS_PONG_WAIT", 60*time.Second),
	}

	// Валидация WebSocket-конфига
	if len(cfg.WebSocket.AllowedOrigins) == 0 || (len(cfg.WebSocket.AllowedOrigins) == 1 && cfg.WebSocket.AllowedOrigins[0] == "") {
		if cfg.IsProd() {
			return nil, fmt.Errorf("WS_ALLOWED_ORIGINS must be set in production")
		}
		// Для разработки разрешаем localhost
		cfg.WebSocket.AllowedOrigins = []string{"http://localhost:8080", "http://localhost:5173"}
	}

	// Redis: по умолчанию обязателен в production (multi-instance деплой без него
	// потеряет кросс-нодовый WebSocket fan-out и low-latency wakeup воркеров).
	cfg.Redis = RedisConfig{
		URL:      getEnv("REDIS_URL", ""),
		Required: getBoolEnv("REDIS_REQUIRED", cfg.IsProd()),
	}

	cfg.AutoMigrate = getBoolEnv("AUTO_MIGRATE", true)

	sandboxCfg, err := loadSandboxConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid sandbox config: %w", err)
	}
	cfg.Sandbox = sandboxCfg

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

// getSliceEnv получает переменную окружения как слайс строк или возвращает значение по умолчанию
func getSliceEnv(key, separator string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, separator)
	}
	return defaultValue
}

func normalizeEnvName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// DecodeEncryptionKeyHex декодирует ENCRYPTION_KEY: ровно 64 hex-символа → 32 байта.
func DecodeEncryptionKeyHex(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return nil, fmt.Errorf("must be exactly 64 hexadecimal characters, got %d", len(s))
	}
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hexadecimal: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("decoded key must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

// LLMConfig содержит конфигурацию для LLM провайдеров
type LLMConfig struct {
	DefaultProvider  string
	OpenRouterAPIKey string // Специальный ключ для OpenRouter API (legacy: получение моделей через /models)
	OpenAI           ProviderConfig
	Anthropic        ProviderConfig
	Gemini           ProviderConfig
	Deepseek         ProviderConfig
	Qwen             ProviderConfig
	// OpenRouter — полноценный ProviderConfig для использования как backend
	// глобальными LLM-агентами (assistant/orchestrator/planner). APIKey
	// совпадает с OpenRouterAPIKey, но мы держим поля раздельно: историческое
	// поле осталось для models-listing, а ProviderConfig — для Generate().
	OpenRouter  ProviderConfig
	Zhipu       ProviderConfig
	Antigravity ProviderConfig
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
