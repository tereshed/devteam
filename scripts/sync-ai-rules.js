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

let mainContent = '';
let otherRulesList = [];

for (const file of files) {
    const content = fs.readFileSync(path.join(DOCS_DIR, file), 'utf-8');
    
    // Write to .cursor/rules
    const mdcName = file.replace('.md', '.mdc');
    fs.writeFileSync(path.join(CURSOR_DIR, mdcName), content);
    console.log(`✅ Synced ${file} -> .cursor/rules/${mdcName}`);

    // Collect content for global files
    const cleanContent = stripFrontmatter(content);
    if (file === 'main.md' || file === 'general.md') {
        mainContent = cleanContent;
    } else {
        otherRulesList.push(`- **${file.replace('.md', '')}**: См. файл \`docs/rules/${file}\``);
    }
}

// 2. Generate CLAUDE.md
const claudeContent = `${mainContent}

---

## 📚 Детальные правила по компонентам системы

Для работы с конкретными частями проекта, **ОБЯЗАТЕЛЬНО** прочитай соответствующие правила перед началом работы:

${otherRulesList.join('\n')}

> ⚠️ **ВНИМАНИЕ AI-АССИСТЕНТ:** Не пытайся угадать правила линтинга, архитектуры или деплоя. Всегда используй инструмент чтения файлов (Read tool), чтобы прочитать нужный файл из \`docs/rules/\` перед написанием кода!
`;

fs.writeFileSync(path.join(ROOT_DIR, 'CLAUDE.md'), claudeContent);
console.log('✅ Generated CLAUDE.md');

// 3. Generate .windsurfrules
fs.writeFileSync(path.join(ROOT_DIR, '.windsurfrules'), claudeContent);
console.log('✅ Generated .windsurfrules');

// 4. Generate .github/copilot-instructions.md
const copilotContent = `${mainContent}

---

## Детальные правила
Ознакомьтесь с файлами в директории \`docs/rules/\` для получения специфичных инструкций по backend, frontend, review и деплою.
`;
fs.writeFileSync(path.join(ROOT_DIR, '.github/copilot-instructions.md'), copilotContent);
console.log('✅ Generated .github/copilot-instructions.md');

console.log('🎉 Все AI-правила успешно синхронизированы!');
