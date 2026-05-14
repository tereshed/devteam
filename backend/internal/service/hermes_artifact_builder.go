// Package service / Sprint 16.C — HermesArtifactBuilder.
//
// Собирает per-agent файлы для sandbox-контейнера hermes:
//   ~/.hermes/config.yaml — model/toolsets/display defaults
//   ~/.hermes/mcp.json    — пер-агентные MCP-сервера (с резолвом ${secret:NAME})
//   ~/.hermes/skills/...  — содержимое Skills (валидируется на path traversal)
//   DEVTEAM_HERMES_*      — env-vars для entrypoint (toolsets/skills/mode/turns)
//   HERMES_MCP_*          — секреты для MCP-серверов (значения резолвлены)
//
// Контракт безопасности:
//   - permission_mode∈{plan,default} НЕ доходит до билдера: его отклоняет
//     team_service.validateHermesSection (400). Если всё-таки прошло —
//     builder тоже отказывается (defense in depth).
//   - ключи Skills проходят filepath.Clean + проверку, что путь остаётся под
//     baseDir, до возврата HermesArtifacts (Path Traversal через JSON в БД).
//   - ${secret:NAME} резолвится через ArtifactBuilderDeps.SecretResolver;
//     если резолвера нет, а секреты заявлены — ошибка.
package service

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/devteam/backend/internal/models"
)

// HermesArtifactBuilder реализует ArtifactBuilder для CodeBackendHermes.
type HermesArtifactBuilder struct{}

// NewHermesArtifactBuilder — конструктор; зависимостей нет (всё — через ArtifactBuilderDeps).
func NewHermesArtifactBuilder() *HermesArtifactBuilder { return &HermesArtifactBuilder{} }

// Backend — задача 16.C: hermes.
func (b *HermesArtifactBuilder) Backend() models.CodeBackend { return models.CodeBackendHermes }

// HermesToolsetCatalog — built-in каталог Hermes toolsets.
//
// Источник: `hermes toolsets list` upstream. Поддерживается как whitelist для
// валидации agent.code_backend_settings.hermes.toolsets и как тело ответа
// GET /api/v1/hermes/toolsets (UI dropdown).
//
// Расширение: добавление нового toolset'а требует и записи здесь, и обновления
// e2e-теста (см. tests). Не парсим из upstream в рантайме — image-pin в Dockerfile
// (HERMES_REF) фиксирует версию, но статический список безопаснее.
var HermesToolsetCatalog = []HermesToolsetInfo{
	{Name: "file_ops", Description: "Read/Edit/Write/Glob/Grep over the workspace"},
	{Name: "shell", Description: "Run shell commands (bash) inside sandbox"},
	{Name: "web_fetch", Description: "Fetch HTTP(S) resources"},
	{Name: "web_search", Description: "Search the web (provider-dependent)"},
	{Name: "code_review", Description: "Higher-level code review helpers"},
	{Name: "todo", Description: "Persistent TODO list for the agent"},
}

// HermesToolsetInfo — одна запись каталога toolsets.
type HermesToolsetInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// hermesDefaults — те же дефолты, что в HermesSettings на фронте (frontend/.../agent_settings_model.dart).
// Если код этих дефолтов меняется тут — синхронизировать там же, чтобы UI не показывал
// другие значения, чем backend применяет при пустом code_backend_settings.
func hermesDefaults() HermesAgentSettings {
	return HermesAgentSettings{
		Toolsets:       []string{"file_ops", "shell"},
		PermissionMode: "yolo",
		MaxTurns:       12,
	}
}

// Build — собирает HermesArtifacts из agent.CodeBackendSettings.
//
// При nil/пустом CodeBackendSettings применяются hermesDefaults() — никакой паники
// при разыменовании. Это гарантирует, что любой существующий агент с
// code_backend=hermes (в т.ч. из миграций где settings = NULL) получит рабочие
// артефакты.
//
// project — обязателен только если в mcp_servers.env присутствуют ${secret:NAME}
// шаблоны (резолвятся через user_llm_credentials по project.UserID). Для агентов
// без секретов nil-project допустим: все равно дойдём до buildHermesMCPJSON только
// если есть servers, а билдер при отсутствии шаблонов в env не дёргает резолвер.
func (b *HermesArtifactBuilder) Build(ctx context.Context, agent *models.Agent, project *models.Project, deps ArtifactBuilderDeps) (*BackendArtifacts, error) {
	if agent == nil {
		return nil, errors.New("hermes builder: agent is nil")
	}
	settings, err := decodeHermesSettings(agent.CodeBackendSettings)
	if err != nil {
		return nil, fmt.Errorf("hermes builder: %w", err)
	}
	// Defense in depth: validateHermesSection в team_service уже отсёк plan/default,
	// но если кто-то вызвал Build напрямую (тесты/скрипты), не молчим.
	if err := validateHermesPermissionMode(settings.PermissionMode); err != nil {
		return nil, fmt.Errorf("hermes builder: %w", err)
	}

	envOut := map[string]string{}
	envOut["DEVTEAM_HERMES_PERMISSION_MODE"] = settings.PermissionMode
	if len(settings.Toolsets) > 0 {
		envOut["DEVTEAM_HERMES_TOOLSETS"] = strings.Join(settings.Toolsets, ",")
	}
	if settings.MaxTurns > 0 {
		envOut["DEVTEAM_HERMES_MAX_TURNS"] = strconv.Itoa(settings.MaxTurns)
	}
	if len(settings.Skills) > 0 {
		names := make([]string, 0, len(settings.Skills))
		for _, sk := range settings.Skills {
			names = append(names, sk.Name)
		}
		envOut["DEVTEAM_HERMES_SKILLS"] = strings.Join(names, ",")
	}

	configYAML, err := buildHermesConfigYAML(settings)
	if err != nil {
		return nil, fmt.Errorf("hermes builder: config.yaml: %w", err)
	}

	var mcpJSON []byte
	if len(settings.MCPServers) > 0 {
		mcpBytes, mcpEnv, err := buildHermesMCPJSON(ctx, project, settings.MCPServers, deps.SecretResolver)
		if err != nil {
			return nil, fmt.Errorf("hermes builder: mcp.json: %w", err)
		}
		mcpJSON = mcpBytes
		for k, v := range mcpEnv {
			envOut[k] = v
		}
	}

	skillsFiles, err := buildHermesSkillsFiles(settings.Skills)
	if err != nil {
		return nil, fmt.Errorf("hermes builder: skills: %w", err)
	}

	return &BackendArtifacts{
		HermesConfigYAML: configYAML,
		HermesMCPJSON:    mcpJSON,
		HermesSkills:     skillsFiles,
		HermesEnv:        envOut,
	}, nil
}

func decodeHermesSettings(raw []byte) (HermesAgentSettings, error) {
	defaults := hermesDefaults()
	cs, err := decodeCodeBackendSettings(raw)
	if err != nil {
		return defaults, err
	}
	if cs.Hermes == nil {
		return defaults, nil
	}
	out := *cs.Hermes
	// Sprint 16.C: дефолты применяем для пустых полей (а не «hermes==nil»):
	// если пользователь явно указал toolsets=[], уважаем — это его выбор.
	// Но permission_mode "" → yolo, max_turns 0 → 12.
	if out.PermissionMode == "" {
		out.PermissionMode = defaults.PermissionMode
	}
	if out.MaxTurns == 0 {
		out.MaxTurns = defaults.MaxTurns
	}
	if out.Toolsets == nil {
		out.Toolsets = defaults.Toolsets
	}
	return out, nil
}

func validateHermesPermissionMode(mode string) error {
	switch mode {
	case "yolo", "accept":
		return nil
	}
	return fmt.Errorf("permission_mode %q not allowed (use yolo|accept)", mode)
}

// buildHermesConfigYAML — минимальный config.yaml.
//
// MVP: только базовые поля (model/provider читает hermes из CLI-флагов,
// здесь — defaults для toolsets и display). Не используем стороннюю YAML-библиотеку,
// чтобы не тащить новую зависимость; формат тривиален и валидируется entrypoint'ом
// через `hermes mcp list` (sanity check).
func buildHermesConfigYAML(s HermesAgentSettings) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("# DevTeam Hermes config (auto-generated; do not edit by hand)\n")
	sb.WriteString("display:\n")
	sb.WriteString("  banner: false\n")
	sb.WriteString("  spinner: false\n")
	if len(s.Toolsets) > 0 {
		sb.WriteString("tools:\n")
		sb.WriteString("  toolsets:\n")
		for _, t := range s.Toolsets {
			if !isSafeYAMLToken(t) {
				return nil, fmt.Errorf("toolset %q contains forbidden chars", t)
			}
			sb.WriteString("    - " + t + "\n")
		}
	}
	if s.Temperature != nil {
		sb.WriteString(fmt.Sprintf("temperature: %g\n", *s.Temperature))
	}
	return []byte(sb.String()), nil
}

// isSafeYAMLToken — узкий whitelist для значений, которые мы кладём как plain-scalar
// в YAML без quoting'а (toolset names и т.п.). Гарантирует отсутствие YAML-инъекции
// и shell-meta при возможной интерполяции entrypoint'ом.
func isSafeYAMLToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// buildHermesMCPJSON — сериализует mcp_servers в формате ~/.hermes/mcp.json
// и возвращает map env-переменных (HERMES_MCP_<name>_<key>) с резолвленными секретами.
//
// Шаблоны ${secret:NAME} в env: значение в mcp.json остаётся ссылкой на env-имя,
// а реальный plaintext уезжает в SandboxOptions.EnvVars (HERMES_MCP_*),
// откуда entrypoint их пробросит в окружение mcp-server'а через `hermes mcp serve`.
// Так секреты не попадают в персистентный mcp.json внутри контейнера.
//
// project передаётся в SecretResolver.Resolve как «якорь» владельца секрета —
// см. doc интерфейса SecretResolver. Если хоть один env содержит ${secret:...},
// а project == nil — это конфигурационная ошибка, и мы предпочитаем явно
// упасть, чем тихо забить на изоляцию проектов.
func buildHermesMCPJSON(ctx context.Context, project *models.Project, servers []HermesMCPServerSpec, resolver SecretResolver) ([]byte, map[string]string, error) {
	envOut := map[string]string{}
	type entry struct {
		Name      string            `json:"name"`
		Transport string            `json:"transport"`
		Command   string            `json:"command,omitempty"`
		Args      []string          `json:"args,omitempty"`
		URL       string            `json:"url,omitempty"`
		Env       map[string]string `json:"env,omitempty"`
	}
	out := make([]entry, 0, len(servers))
	// Стабильный порядок: сортируем по имени, чтобы JSON был детерминирован
	// (диффы в audit log / снапшот-тестах не шатаются).
	sorted := append([]HermesMCPServerSpec(nil), servers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, srv := range sorted {
		e := entry{
			Name:      srv.Name,
			Transport: srv.Transport,
			Command:   srv.Command,
			Args:      append([]string(nil), srv.Args...),
			URL:       srv.URL,
		}
		if len(srv.Env) > 0 {
			e.Env = map[string]string{}
			for k, v := range srv.Env {
				if strings.HasPrefix(v, "${secret:") && strings.HasSuffix(v, "}") {
					name := strings.TrimSuffix(strings.TrimPrefix(v, "${secret:"), "}")
					if name == "" {
						return nil, nil, fmt.Errorf("server %q env %q: empty secret name", srv.Name, k)
					}
					if resolver == nil {
						return nil, nil, fmt.Errorf("server %q env %q: secret resolver not configured", srv.Name, k)
					}
					if project == nil {
						return nil, nil, fmt.Errorf("server %q env %q: project context required to resolve secret %q (owner anchor)", srv.Name, k, name)
					}
					plain, err := resolver.Resolve(ctx, project, name)
					if err != nil {
						return nil, nil, fmt.Errorf("server %q env %q: resolve secret %q: %w", srv.Name, k, name, err)
					}
					envName := hermesMCPEnvName(srv.Name, k)
					envOut[envName] = plain
					// в mcp.json — пишем «$ENVNAME», hermes подхватит из process env.
					e.Env[k] = "$" + envName
				} else {
					e.Env[k] = v
				}
			}
		}
		out = append(out, e)
	}
	body, err := jsonMarshalIndent(map[string]any{"mcpServers": out})
	if err != nil {
		return nil, nil, err
	}
	return body, envOut, nil
}

// hermesMCPEnvName — детерминированное имя env-переменной для секрета MCP-сервера.
// Формат: HERMES_MCP_<UPPER(server)>_<UPPER(key)>; имена уже валидируются вверху
// (UPPER_SNAKE_CASE), поэтому конкатенация безопасна.
func hermesMCPEnvName(server, key string) string {
	return "HERMES_MCP_" + strings.ToUpper(toEnvSafe(server)) + "_" + strings.ToUpper(key)
}

func toEnvSafe(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		case c == '_' || c == '-':
			out = append(out, '_')
		}
	}
	return string(out)
}

// buildHermesSkillsFiles — пока MVP: для каждого skill оставляем плейсхолдер
// SKILL.md с метаданными (имя/source). Реальная подгрузка содержимого из
// AgentSkillRepository / agentskills.io — отдельная задача (16.C.2);
// здесь главное — корректно работать с путями (path traversal) и стабильно
// генерировать карту key→content для runner'а.
func buildHermesSkillsFiles(skills []HermesSkillRef) (map[string][]byte, error) {
	if len(skills) == 0 {
		return nil, nil
	}
	out := map[string][]byte{}
	const baseDir = "/home/sandbox/.hermes/skills"
	for _, sk := range skills {
		if sk.Name == "" {
			return nil, errors.New("skill: empty name")
		}
		// Сначала валидируем по «безопасному» алфавиту имени, потом — путь.
		if !codeBackendSkillNameRE.MatchString(sk.Name) {
			return nil, fmt.Errorf("skill %q: invalid name", sk.Name)
		}
		rel := path.Join(sk.Name, "SKILL.md")
		if err := assertSafeRelativePath(baseDir, rel); err != nil {
			return nil, fmt.Errorf("skill %q: %w", sk.Name, err)
		}
		body := fmt.Sprintf("# Skill: %s\nsource: %s\n", sk.Name, sk.Source)
		out[rel] = []byte(body)
	}
	return out, nil
}

// assertSafeRelativePath — Sprint 16.C path-traversal guard.
//
// Принимает фиксированный baseDir и относительный путь rel; проверяет, что
// после path.Clean(filepath.Join(base, rel)) итог остаётся под baseDir
// (с завершающим разделителем — иначе HasPrefix считает /home/sandbox/.hermes/skills2
// «внутри» /home/sandbox/.hermes/skills).
//
// Отдельно отбиваем абсолютные пути, шорткат "~", null-байты — даже если они
// после Clean окажутся «внутри» (на разных файловых системах поведение разное).
func assertSafeRelativePath(baseDir, rel string) error {
	if rel == "" {
		return errors.New("empty path")
	}
	if strings.ContainsRune(rel, 0) {
		return errors.New("null byte in path")
	}
	if path.IsAbs(rel) || strings.HasPrefix(rel, "~") {
		return fmt.Errorf("path %q: absolute or home shortcut not allowed", rel)
	}
	cleaned := path.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("path %q: parent-traversal not allowed", rel)
	}
	full := path.Clean(path.Join(baseDir, cleaned))
	prefix := path.Clean(baseDir) + "/"
	if !strings.HasPrefix(full, prefix) {
		return fmt.Errorf("path %q: escapes %q", rel, baseDir)
	}
	return nil
}
