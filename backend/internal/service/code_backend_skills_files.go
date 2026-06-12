// Package service — доставка Claude Code / Antigravity skills в sandbox.
//
// Skill для claude-семейства — это ДИРЕКТОРИЯ (SKILL.md + опциональные
// scripts/ и прочие файлы), а не один файл. Содержимое объявляется инлайн в
// agent.code_backend_settings.skills[].config.files:
//
//	{
//	  "name": "my-skill",
//	  "source": "path",
//	  "config": {
//	    "files": {
//	      "SKILL.md": "---\nname: my-skill\ndescription: ...\n---\n...",
//	      "scripts/run.py": "#!/usr/bin/env python3\n..."
//	    }
//	  }
//	}
//
// Builder собирает map "skill-name/rel-path" → bytes; runner копирует его
// в ~/.claude/skills/ (claude-code) или ~/.gemini/antigravity/skills/
// (antigravity) ДО старта контейнера. Exec-бит файлам не нужен: CLI запускает
// скрипты через свой Bash-инструмент (`python3 ${CLAUDE_SKILL_DIR}/scripts/run.py`).
//
// Безопасность: содержимое files — произвольный код, но исполняется он в том же
// изолированном контейнере, где агент и так выполняет произвольный код репозитория;
// модель угроз не расширяется (в отличие от hooks, которые требуют whitelist).
// Пути проходят assertSafeRelativePath (builder) + assertHermesSkillRelPath
// (runner, defense-in-depth) — файл не может быть записан вне каталога skills.
package service

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// skillFileSegmentRE — один сегмент rel-пути файла skill'а: начинается с
// буквы/цифры/_ (не с точки — скрытые файлы запрещены), далее [a-zA-Z0-9._-].
var skillFileSegmentRE = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9._-]*$`)

const (
	// codeBackendSkillsValidationBase — фиктивный base для path-traversal
	// проверки rel-путей на этапе builder'а (реальный dst выбирает runner по backend'у).
	codeBackendSkillsValidationBase = "/home/sandbox/.claude/skills"

	// maxSkillFilesPerSkill — максимум файлов в одном skill'е.
	maxSkillFilesPerSkill = 64
	// maxSkillFileBytes — максимум на один файл (256 KiB).
	maxSkillFileBytes = 256 * 1024
	// maxSkillsTotalBytes — суммарный бюджет на все skills агента (1 MiB);
	// согласован с CodeResultMaxArtifactBytes по порядку величины, чтобы
	// JSONB-настройки агента не разрастались бесконтрольно.
	maxSkillsTotalBytes = 1 << 20
)

// buildCodeBackendSkillsFiles — общий хелпер Claude/Antigravity билдеров:
// конвертирует AgentSkillRef'ы в дерево файлов "skill-name/rel-path" → content.
//
// Правила:
//   - config.files задан → это полное дерево skill'а; SKILL.md обязателен
//     (без него CLI skill не обнаружит — fail loud, а не тихий no-op);
//   - config.files НЕ задан → placeholder SKILL.md с метаданными (паритет
//     с buildHermesSkillsFiles, MVP до подгрузки контента из внешних каталогов).
//
// Имена skill'ов и rel-пути валидируются здесь повторно (defense in depth) —
// первичный отказ происходит в validateCodeBackendSettingsStrict при сохранении.
func buildCodeBackendSkillsFiles(skills []AgentSkillRef) (map[string][]byte, error) {
	if len(skills) == 0 {
		return nil, nil
	}
	out := map[string][]byte{}
	total := 0
	for _, sk := range skills {
		if sk.Name == "" {
			return nil, fmt.Errorf("skill: empty name")
		}
		if !codeBackendSkillNameRE.MatchString(sk.Name) {
			return nil, fmt.Errorf("skill %q: invalid name", sk.Name)
		}
		files, err := skillConfigFiles(sk.Config)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", sk.Name, err)
		}
		if files == nil {
			// Placeholder: skill объявлен без содержимого. Генерируем минимальный
			// SKILL.md с frontmatter, чтобы CLI видел skill и его происхождение.
			rel := path.Join(sk.Name, "SKILL.md")
			if err := assertSafeRelativePath(codeBackendSkillsValidationBase, rel); err != nil {
				return nil, fmt.Errorf("skill %q: %w", sk.Name, err)
			}
			body := fmt.Sprintf("---\nname: %s\ndescription: DevTeam skill placeholder (source: %s, no files provided)\n---\n", sk.Name, sk.Source)
			out[rel] = []byte(body)
			total += len(body)
			continue
		}
		if len(files) > maxSkillFilesPerSkill {
			return nil, fmt.Errorf("skill %q: too many files (%d > %d)", sk.Name, len(files), maxSkillFilesPerSkill)
		}
		if _, ok := files["SKILL.md"]; !ok {
			return nil, fmt.Errorf("skill %q: config.files must contain SKILL.md", sk.Name)
		}
		// Стабильный порядок — детерминированные ошибки/тесты.
		rels := make([]string, 0, len(files))
		for rel := range files {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			if err := validateSkillFileRelPath(rel); err != nil {
				return nil, fmt.Errorf("skill %q: file %q: %w", sk.Name, rel, err)
			}
			content := files[rel]
			if len(content) > maxSkillFileBytes {
				return nil, fmt.Errorf("skill %q: file %q: too large (%d > %d bytes)", sk.Name, rel, len(content), maxSkillFileBytes)
			}
			full := path.Join(sk.Name, path.Clean(rel))
			// Containment строго ВНУТРИ каталога своего skill'а: после Clean путь
			// не должен выехать ни из skills-базы, ни из <base>/<skill-name>.
			if err := assertSafeRelativePath(codeBackendSkillsValidationBase+"/"+sk.Name, rel); err != nil {
				return nil, fmt.Errorf("skill %q: file %q: %w", sk.Name, rel, err)
			}
			if _, dup := out[full]; dup {
				return nil, fmt.Errorf("skill %q: file %q: duplicate path after normalization", sk.Name, rel)
			}
			out[full] = []byte(content)
			total += len(content)
		}
		if total > maxSkillsTotalBytes {
			return nil, fmt.Errorf("skills: total content size exceeds %d bytes", maxSkillsTotalBytes)
		}
	}
	return out, nil
}

// validateSkillConfigFiles — общая save-time проверка skills[].config.files
// (используется и для claude-семейства, и для hermes-секции). Возвращает
// суммарный размер файлов — caller ведёт общий бюджет maxSkillsTotalBytes.
func validateSkillConfigFiles(config map[string]any) (int, error) {
	files, err := skillConfigFiles(config)
	if err != nil {
		return 0, err
	}
	if files == nil {
		return 0, nil
	}
	if len(files) > maxSkillFilesPerSkill {
		return 0, fmt.Errorf("too many files (%d > %d)", len(files), maxSkillFilesPerSkill)
	}
	if _, ok := files["SKILL.md"]; !ok {
		return 0, fmt.Errorf("config.files must contain SKILL.md")
	}
	total := 0
	for rel, content := range files {
		if err := validateSkillFileRelPath(rel); err != nil {
			return 0, fmt.Errorf("file %q: %w", rel, err)
		}
		if len(content) > maxSkillFileBytes {
			return 0, fmt.Errorf("file %q too large (%d > %d bytes)", rel, len(content), maxSkillFileBytes)
		}
		total += len(content)
	}
	return total, nil
}

// skillConfigFiles извлекает config.files как map[string]string.
// nil-результат без ошибки — files не заданы (placeholder-режим).
func skillConfigFiles(config map[string]any) (map[string]string, error) {
	if config == nil {
		return nil, nil
	}
	raw, ok := config["files"]
	if !ok || raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config.files must be an object {path: content}")
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("config.files must not be empty (omit it for a placeholder skill)")
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("config.files[%q]: content must be a string", k)
		}
		out[k] = s
	}
	return out, nil
}

// validateSkillFileRelPath — синтаксис относительного пути файла skill'а.
// Узкий whitelist по сегментам: буквы/цифры/._-, без скрытых файлов (ведущая
// точка), без backslash (windows-пути не нормализуем — отклоняем явно).
func validateSkillFileRelPath(rel string) error {
	if rel == "" {
		return fmt.Errorf("empty path")
	}
	if strings.ContainsRune(rel, '\\') {
		return fmt.Errorf("backslash not allowed (use forward slashes)")
	}
	if strings.ContainsRune(rel, 0) {
		return fmt.Errorf("null byte in path")
	}
	if path.IsAbs(rel) || strings.HasPrefix(rel, "~") {
		return fmt.Errorf("absolute or home path not allowed")
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			return fmt.Errorf("empty path segment")
		}
		if seg == "." || seg == ".." {
			return fmt.Errorf("dot segments not allowed")
		}
		if !skillFileSegmentRE.MatchString(seg) {
			return fmt.Errorf("segment %q: only [a-zA-Z0-9._-] allowed, must not start with a dot", seg)
		}
	}
	return nil
}
