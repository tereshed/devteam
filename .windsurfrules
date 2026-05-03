# DevTeam — AI Agent Orchestrator

Стек: **Go (Gin)**, **Flutter**, **YugabyteDB**, **Weaviate**, **Claude Code CLI / Aider**.

## ⚠️ Главные правила (SSOT)
1. **Правила в `docs/rules/`**: Всегда читай детальные правила перед работой. При изменении правил запускай `make rules`.
2. **Никаких лишних файлов**: ЗАПРЕЩЕНО создавать итоговые `.md` файлы с результатами. Пиши только в `README.md`.
3. **Изоляция**: Задачи выполняются в изолированных Docker-контейнерах.

## 🔴 Критичные правила по стеку
**Backend (Go):**
- **Clean Architecture**: Строго `handler` -> `service` -> `repository`.
- **Глобальные переменные**: ЗАПРЕЩЕНЫ. Используй Dependency Injection.
- **Миграции**: Только Goose (`make migrate-create`). ЗАПРЕЩЕНО `gorm.AutoMigrate`.
- **MCP**: Для новых публичных ручек обязательно создавай MCP-инструмент в `internal/mcp/`.
- **Swagger**: Обновляй аннотации и запускай `make swagger`.

**Frontend (Flutter):**
- **Архитектура**: Feature-First + Riverpod 3.x (`flutter_riverpod` ^3).
- **Freezed**: Обязательно используй `abstract class` (напр. `abstract class UserModel`).
- **Импорты**: Только абсолютные (`package:frontend/...`), НИКАКИХ `../`.
- **Локализация**: ЗАПРЕЩЕН хардкод строк в UI. Используй `.arb` и `flutter gen-l10n`.

## ✅ Чек-лист перед коммитом
- [ ] Прочитан ли профильный файл из `docs/rules/`?
- [ ] Написаны ли тесты (Unit/Integration)?
- [ ] (Backend) Выполнен ли `make swagger` и `make test-all`?
- [ ] (Frontend) Выполнен ли `make frontend-codegen` и `make frontend-analyze`?
- [ ] Нет ли захардкоженных строк или секретов?

## 📚 Детальные правила (Читай через Read tool!)
- **backend**: `docs/rules/backend.md`
- **deploy**: `docs/rules/deploy.md`
- **frontend**: `docs/rules/frontend.md`
- **idias**: `docs/rules/idias.md`
- **main**: `docs/rules/main.md`
- **review**: `docs/rules/review.md`
