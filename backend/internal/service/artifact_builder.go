// Package service / Sprint 16.C — реестр ArtifactBuilder для разных code-backends.
//
// Идея: AgentSettingsService больше не «знает» формат конкретного backend'а
// (claude-code, hermes, …). Он держит registry: map[CodeBackend]ArtifactBuilder и
// делегирует сборку файловых артефактов выбранному билдеру. Добавление нового
// backend'а = новая реализация интерфейса + регистрация, без правки существующих.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
)

// BackendArtifacts — union-структура с per-backend файлами для sandbox-runner'а.
//
// Для конкретного агента заполнены только поля «своего» backend'а (claude или hermes).
// Runner смотрит на agent.CodeBackend и решает, какие поля читать.
//
// Не делаем sealed interface — слишком много callsite'ов читают конкретные поля
// (раннер копирует SettingsJSON в .claude/settings.json и т.п.). Union-struct
// проще и безопаснее, чем type-switch по интерфейсу.
type BackendArtifacts struct {
	// --- Claude Code (Sprint 15.22) ---
	SettingsJSON   []byte
	MCPJSON        []byte
	Skills         []AgentSkillArtifact
	PermissionMode string

	// --- Hermes (Sprint 16.C) ---
	// HermesConfigYAML — содержимое ~/.hermes/config.yaml (модель/toolsets/display).
	HermesConfigYAML []byte
	// HermesMCPJSON — ~/.hermes/mcp.json (массив MCP-серверов в hermes-схеме).
	HermesMCPJSON []byte
	// HermesSkills — map относительный_путь → содержимое; ключи нормализованы и
	// проверены на path traversal в HermesArtifactBuilder. Runner кладёт их в
	// /home/sandbox/.hermes/skills/<rel-path>.
	HermesSkills map[string][]byte
	// HermesEnv — пер-агентные env, которые runner добавит в SandboxOptions.EnvVars.
	// Сюда попадают DEVTEAM_HERMES_TOOLSETS / SKILLS / PERMISSION_MODE / MAX_TURNS
	// и HERMES_MCP_* секреты для MCP-серверов.
	HermesEnv map[string]string
}

// ArtifactBuilder — собирает per-backend файловые артефакты для sandbox-агента.
// Реализации регистрируются в AgentSettingsService и вызываются по agent.CodeBackend.
//
// Sprint 16.C-2: project передаётся явно — это «якорь владельца» для резолва
// секретов (project.UserID → user_llm_credentials, project.ID → agent_secrets).
// Антипаттерн с UserID в ctx.Value сознательно не используем: типизация и
// статическая проверка важнее, чем удобство «не править сигнатуры».
type ArtifactBuilder interface {
	// Backend — какой code_backend обслуживает реализация.
	Backend() models.CodeBackend
	// Build — собрать артефакты по агенту и shared-зависимостям.
	// Должен корректно работать при пустом/nil agent.CodeBackendSettings:
	// валидатор уже мог отклонить плохой JSON, дефолты применяются здесь же.
	// project может быть nil только для билдеров, которым контекст владельца
	// не нужен (например, Claude без MCP-секретов).
	Build(ctx context.Context, agent *models.Agent, project *models.Project, deps ArtifactBuilderDeps) (*BackendArtifacts, error)
}

// ArtifactBuilderDeps — общие зависимости, которые хочет любой builder.
// Не передаём конкретные репозитории/сервисы — только узкие интерфейсы (DI),
// чтобы builder можно было собрать в тесте без поднятия целого AgentSettingsService.
type ArtifactBuilderDeps struct {
	// MCPRegistry — глобальный реестр MCP-серверов (Claude использует, Hermes тоже может).
	MCPRegistry MCPRegistryLookup
	// SecretResolver — резолвит ${secret:NAME} → plaintext (для Hermes mcp.json env).
	// Если nil, builder обязан игнорировать секрет-шаблоны (или вернуть ошибку при их наличии).
	SecretResolver SecretResolver
}

// SecretResolver — узкий интерфейс для подстановки ${secret:NAME} в MCP-конфигах.
// Реализуется поверх user_llm_credentials / agent_secrets.
//
// Sprint 16.C-2: project — обязательный «якорь» владельца. Без него мы не знаем,
// чей user_llm_credentials читать, и не можем гарантировать изоляцию секретов
// между проектами. nil-project запрещён — резолвер вернёт ошибку.
type SecretResolver interface {
	// Resolve возвращает plaintext значение по логическому имени.
	// Возвращает ErrSecretNotFound если имени нет.
	Resolve(ctx context.Context, project *models.Project, name string) (string, error)
}

// ErrSecretNotFound — резолвер не нашёл секрет с таким именем.
var ErrSecretNotFound = errors.New("secret not found")

// MCPRepositoryLookupAdapter — адаптер MCPServerRegistryRepository → MCPRegistryLookup.
//
// MCPRegistryLookup для исторической совместимости синхронный (без ctx);
// в адаптере используем context.Background() — допустимо для read-only-запроса
// по индексу name (быстрый GET, без сетевого latency). При замене на удалённый
// реестр менять интерфейс на ctx-aware.
type MCPRepositoryLookupAdapter struct {
	repo mcpServerRegistryReader
}

// mcpServerRegistryReader — узкий интерфейс, чтобы адаптер не тянул весь репозиторный API.
// Реализуется repository.MCPServerRegistryRepository.
type mcpServerRegistryReader interface {
	GetByName(ctx context.Context, name string) (*models.MCPServerRegistry, error)
}

// NewMCPRepositoryLookupAdapter — конструктор адаптера.
func NewMCPRepositoryLookupAdapter(repo mcpServerRegistryReader) *MCPRepositoryLookupAdapter {
	return &MCPRepositoryLookupAdapter{repo: repo}
}

// LookupMCPServer — реализация MCPRegistryLookup.
func (a *MCPRepositoryLookupAdapter) LookupMCPServer(name string) (*models.MCPServerRegistry, bool) {
	if a == nil || a.repo == nil {
		return nil, false
	}
	srv, err := a.repo.GetByName(context.Background(), name)
	if err != nil || srv == nil {
		return nil, false
	}
	return srv, true
}

// ArtifactBuilderRegistry — реестр билдеров по code_backend.
//
// Минимальный API: Register / Get. Дубликаты при Register — паника на старте,
// чтобы не получить двух конкурирующих билдеров под один backend в тестах.
type ArtifactBuilderRegistry struct {
	builders map[models.CodeBackend]ArtifactBuilder
}

// NewArtifactBuilderRegistry собирает пустой реестр.
func NewArtifactBuilderRegistry() *ArtifactBuilderRegistry {
	return &ArtifactBuilderRegistry{builders: map[models.CodeBackend]ArtifactBuilder{}}
}

// Register добавляет билдер в реестр. Дубликат → panic (программная ошибка инициализации).
func (r *ArtifactBuilderRegistry) Register(b ArtifactBuilder) {
	if b == nil {
		panic("artifact_builder: cannot register nil builder")
	}
	be := b.Backend()
	if !be.IsValid() {
		panic(fmt.Sprintf("artifact_builder: invalid backend %q", be))
	}
	if _, dup := r.builders[be]; dup {
		panic(fmt.Sprintf("artifact_builder: duplicate registration for backend %q", be))
	}
	r.builders[be] = b
}

// Get возвращает билдер по backend'у. ok=false если такого нет.
func (r *ArtifactBuilderRegistry) Get(be models.CodeBackend) (ArtifactBuilder, bool) {
	b, ok := r.builders[be]
	return b, ok
}
