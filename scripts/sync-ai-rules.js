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
    rulesList.push(`- **${file.replace('.md', '')}**: \`docs/rules/${file}\``);
}

// 2. Generate CLAUDE.md
const claudeContent = `# DevTeam — AI Agent Orchestrator

Стек: **Go (Gin)**, **Flutter**, **YugabyteDB**, **Weaviate**, **Claude Code CLI / Aider**.

## ⚠️ Главные правила (SSOT)
1. **Правила в \`docs/rules/\`**: Всегда читай детальные правила перед работой. При изменении правил запускай \`make rules\`.
2. **Никаких лишних файлов**: ЗАПРЕЩЕНО создавать итоговые \`.md\` файлы с результатами. Пиши только в \`README.md\`.
3. **Изоляция**: Задачи выполняются в изолированных Docker-контейнерах.

## 🔴 Критичные правила по стеку
**Backend (Go):**
- **Clean Architecture**: Строго \`handler\` -> \`service\` -> \`repository\`.
- **Глобальные переменные**: ЗАПРЕЩЕНЫ. Используй Dependency Injection.
- **Миграции**: Только Goose (\`make migrate-create\`). ЗАПРЕЩЕНО \`gorm.AutoMigrate\`.
- **MCP**: Для новых публичных ручек обязательно создавай MCP-инструмент в \`internal/mcp/\`.
- **Swagger**: Обновляй аннотации и запускай \`make swagger\`.

**Frontend (Flutter):**
- **Архитектура**: Feature-First + Riverpod 2.0.
- **Freezed**: Обязательно используй \`abstract class\` (напр. \`abstract class UserModel\`).
- **Импорты**: Только абсолютные (\`package:frontend/...\`), НИКАКИХ \`../\`.
- **Локализация**: ЗАПРЕЩЕН хардкод строк в UI. Используй \`.arb\` и \`flutter gen-l10n\`.

## ✅ Чек-лист перед коммитом
- [ ] Прочитан ли профильный файл из \`docs/rules/\`?
- [ ] Написаны ли тесты (Unit/Integration)?
- [ ] (Backend) Выполнен ли \`make swagger\` и \`make test-all\`?
- [ ] (Frontend) Выполнен ли \`make frontend-codegen\` и \`make frontend-analyze\`?
- [ ] Нет ли захардкоженных строк или секретов?

## 📚 Детальные правила (Читай через Read tool!)
${rulesList.join('\n')}
`;

fs.writeFileSync(path.join(ROOT_DIR, 'CLAUDE.md'), claudeContent);
console.log('✅ Generated CLAUDE.md');

// 3. Generate .windsurfrules
fs.writeFileSync(path.join(ROOT_DIR, '.windsurfrules'), claudeContent);
console.log('✅ Generated .windsurfrules');

// 4. Generate .github/copilot-instructions.md
fs.writeFileSync(path.join(ROOT_DIR, '.github/copilot-instructions.md'), claudeContent);
console.log('✅ Generated .github/copilot-instructions.md');

console.log('🎉 Все AI-правила успешно синхронизированы!');
