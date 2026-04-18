const fs = require('fs');
const path = require('path');

const DOCS_DIR = path.join(__dirname, '../docs/rules');
const CURSOR_DIR = path.join(__dirname, '../.cursor/rules');
const ROOT_DIR = path.join(__dirname, '..');

// Ensure directories exist
if (!fs.existsSync(CURSOR_DIR)) {
    fs.mkdirSync(CURSOR_DIR, { recursive: true });
}
if (!fs.existsSync(path.join(ROOT_DIR, '.github'))) {
    fs.mkdirSync(path.join(ROOT_DIR, '.github'), { recursive: true });
}

// Helper to strip YAML frontmatter
function stripFrontmatter(content) {
    return content.replace(/^---\n[\s\S]*?\n---\n/, '').trim();
}

// 1. Copy all .md files from docs/rules to .cursor/rules as .mdc
const files = fs.readdirSync(DOCS_DIR).filter(f => f.endsWith('.md'));

let rulesList = [];

for (const file of files) {
    const content = fs.readFileSync(path.join(DOCS_DIR, file), 'utf-8');
    
    // Write to .cursor/rules
    const mdcName = file.replace('.md', '.mdc');
    fs.writeFileSync(path.join(CURSOR_DIR, mdcName), content);
    console.log(`✅ Synced ${file} -> .cursor/rules/${mdcName}`);

    // Collect content for global files
    rulesList.push(`- **${file.replace('.md', '')}**: См. файл \`docs/rules/${file}\``);
}

// 2. Generate CLAUDE.md
const claudeContent = `# DevTeam — AI Agent Orchestrator

Ты — ведущий архитектор и разработчик платформы **DevTeam**. Это приложение-оркестратор AI-агентов, которое автоматизирует полный цикл разработки ПО: от идеи до работающего кода.

Стек: **Go (Gin)**, **Flutter**, **YugabyteDB**, **Weaviate**, **Claude Code CLI / Aider**.

---

## ⚠️ Главные правила работы с репозиторием

1. **Единый источник правды (SSOT):** Все AI-правила хранятся в \`docs/rules/\`. Если нужно изменить правила, редактируй файлы там, а затем выполни \`make rules\`.
2. **Никаких лишних файлов:** ЗАПРЕЩЕНО создавать итоговые файлы \`.md\` с результатами работы. Можно писать только сжатую информацию в \`README.md\`.
3. **Изоляция:** Задачи выполняются в изолированных Docker-контейнерах.

---

## 📚 Детальные правила по компонентам системы

Для работы с конкретными частями проекта, **ОБЯЗАТЕЛЬНО** прочитай соответствующие правила перед началом работы:

${rulesList.join('\n')}

> ⚠️ **ВНИМАНИЕ AI-АССИСТЕНТ:** Не пытайся угадать правила линтинга, архитектуры или деплоя. Всегда используй инструмент чтения файлов (Read tool), чтобы прочитать нужный файл из \`docs/rules/\` перед написанием кода!
`;

fs.writeFileSync(path.join(ROOT_DIR, 'CLAUDE.md'), claudeContent);
console.log('✅ Generated CLAUDE.md');

// 3. Generate .windsurfrules
fs.writeFileSync(path.join(ROOT_DIR, '.windsurfrules'), claudeContent);
console.log('✅ Generated .windsurfrules');

// 4. Generate .github/copilot-instructions.md
const copilotContent = `# DevTeam — AI Agent Orchestrator

Стек: Go (Gin), Flutter, YugabyteDB, Weaviate.

## Детальные правила
Ознакомьтесь с файлами в директории \`docs/rules/\` для получения специфичных инструкций по архитектуре, backend, frontend, review и деплою:

${rulesList.join('\n')}
`;
fs.writeFileSync(path.join(ROOT_DIR, '.github/copilot-instructions.md'), copilotContent);
console.log('✅ Generated .github/copilot-instructions.md');

console.log('🎉 Все AI-правила успешно синхронизированы!');
