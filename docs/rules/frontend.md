---
alwaysApply: true
---

## Руководство по разработке и поддержке Flutter-приложения (Web/Android/iOS)

**Цель:** Поддержка и развитие кроссплатформенного приложения на Flutter с бэкендом на FastAPI.

**Платформы:** Web, Android, iOS.
**Backend API:** Gin (Go).

-----

## 1\. Базовые принципы и правила (Code Style)

Для обеспечения долгосрочной поддержки проекта, код должен соответствовать следующим требованиям:

### 1.1. Читаемая структура

  * **Архитектура:** Мы используем **Feature-First (модульную)** архитектуру. Код группируется по функциональным модулям (например, `auth`, `profile`, `feed`), а не по типам файлов (например, `screens`, `widgets`, `controllers`).
  * **Именование:** Соблюдайте стандартный Dart naming conventions (`camelCase` для переменных/функций, `PascalCase` для классов/виджетов, `snake_case` для имен файлов).
  * **Линтинг:** Обязательное использование встроенного `flutter analyze` и кастомного набора правил в `analysis_options.yaml` (например, `effective_dart` или `flutter_lints` со строгими правилами).
  * **Импорты:** **ЗАПРЕЩЕНО** использовать относительные импорты с `../` или `../../`. **ВСЕГДА** используйте абсолютные импорты с префиксом `package:` и именем пакета из `pubspec.yaml`. Например: `package:frontend/core/utils/responsive.dart` вместо `../../../../core/utils/responsive.dart`.

### 1.2. Принцип единственной ответственности (Small Files)

  * **Маленькие файлы:** Каждый файл (виджет, сервис, контроллер) должен иметь одну четкую зону ответственности.
  * **Декомпозиция виджетов:** Избегайте "ада виджетов" (глубокой вложенности). Любой логически обособленный или переиспользуемый блок UI должен быть вынесен в отдельный `StatelessWidget` или `ConsumerWidget` (в зависимости от выбранного state management).
  * **Разделение логики:** UI-код (виджеты) должен быть отделен от бизнес-логики (state management) и логики данных (репозитории, сервисы).

-----

## 2\. Ключевые фокусные области

При разработке нового функционала или рефакторинге существующего, необходимо уделить особое внимание следующим аспектам:

### 2.1. UI и Компоненты (Design System)

  * **UI Kit:** Все базовые элементы (кнопки, поля ввода, карточки, индикаторы загрузки, диалоги) должны быть вынесены в отдельную директорию `shared/widgets` (или `common/ui`).
  * **Темизация:** Цвета, шрифты, отступы и стили должны быть определены в `ThemeData` (Material 3). Избегайте хардкода цветов (например, `Colors.blue`) или размеров шрифта (`fontSize: 16`) в виджетах. Используйте `Theme.of(context).colorScheme.primary` или `Theme.of(context).textTheme.bodyMedium`.

### 2.2. Адативная верстка (Mobile | Tablet | Desktop)

  * **Обязательная поддержка:** Приложение должно корректно отображаться на всех трех форм-факторах.
  * **Инструменты:** Используйте `LayoutBuilder`, `MediaQuery.of(context).size` или специализированные пакеты (например, `responsive_builder`) для определения текущего размера экрана.
  * **Сетки (Grids):** Для сложных экранов (особенно на Desktop/Tablet) используйте адаптивные сетки (например, `GridView` с `SliverGridDelegateWithMaxCrossAxisExtent` или кастомные решения на `Row`/`Column` с `Expanded`/`Flexible`).
  * **Breakpoints:** Четко определите точки перехода (breakpoints), например:
      * Mobile: \< 600 dp
      * Tablet: 600 dp - 1200 dp
      * Desktop: \> 1200 dp

### 2.3. Мультиязычность (i18n)

  * **Обязательно:** Весь текст, видимый пользователю, должен поддерживать локализацию.
  * **Инструменты:** Используйте стандартный подход `flutter_localizations` с `.arb` файлами или проверенные пакеты (например, `easy_localization` или `slang`).
  * **Запрет хардкода (CRITICAL):** В коде виджетов **СТРОГО ЗАПРЕЩЕНО** использовать строковые литералы (например, `Text("Login")`). Весь текст **ОБЯЗАН** быть вынесен в файлы локализации. Получайте `AppLocalizations` через **`requireAppLocalizations(context, where: '…')`** из **`package:frontend/core/l10n/require.dart`**, затем вызывайте геттеры ключей ARB (например `final l10n = requireAppLocalizations(context, where: 'loginScreen');` … `l10n.login`). Параметр **`where`** указывайте для диагностики (попадает в текст ошибки, если под деревом нет `AppLocalizations.delegate`). Extension **`context.l10n`** в проекте **не** используется. **`AppLocalizations.of(context)!`** допустим в легаси до точечной миграции.

#### КРИТИЧНО: Порядок генерации кода

**ВАЖНО:** При использовании `flutter gen-l10n` совместно с `build_runner`, **ВСЕГДА** соблюдайте правильный порядок команд:

```bash
# ✅ ПРАВИЛЬНО:
flutter pub get
dart run build_runner build --delete-conflicting-outputs
flutter gen-l10n

# ❌ НЕПРАВИЛЬНО:
flutter pub get
flutter gen-l10n
dart run build_runner build --delete-conflicting-outputs  # Удалит файлы локализации!
```

**Причина:** Флаг `--delete-conflicting-outputs` в `build_runner` удаляет файлы, созданные `flutter gen-l10n`, считая их "конфликтующими". Запуск `build_runner` ДО `gen-l10n` решает эту проблему. Тот же порядок обязан соблюдать **`make frontend-test`** в корневом **`Makefile`** (`dart run build_runner …`, затем `flutter gen-l10n`, затем тесты).

**⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА:** `flutter run` может автоматически запускать `build_runner`, который удаляет файлы локализации, даже если они исключены в `build.yaml`. 

**Решение:** Если файлы локализации удаляются при `make frontend-run-web`, выполните после запуска:
```bash
cd frontend && flutter gen-l10n
```

**Генерация и git (важно, без двусмысленности):**
  * Исходники **`frontend/lib/l10n/*.arb`** — в git; **`template-arb-file`** в `l10n.yaml` — обычно `app_ru.arb`.
  * Файлы **`frontend/lib/l10n/app_localizations.dart`**, **`app_localizations_ru.dart`**, **`app_localizations_en.dart`** генерируются **`flutter gen-l10n`** и в этом репозитории **коммитятся в git** (в `frontend/.gitignore` исключён каталог **`lib/generated/`**, а не `lib/l10n/`). После любых правок `.arb` обязательно выполните **`make frontend-codegen`** (или `flutter gen-l10n` в корректном порядке с `build_runner`) и **закоммитьте обновлённый триплет** `app_localizations*.dart`, иначе CI и другие клоны соберутся со старыми геттерами.
  * Перед первым запуском выполнить: `make frontend-setup`
  * Автопроверка паритета ключей и зеркала плейсхолдеров (ru↔en; наличие блоков `@*.placeholders`; совпадение имён и полей `type` в placeholders — см. этап «имена и типы» в **`./scripts/check_l10n_parity.sh`**): **`make frontend-l10n-check`**.
  * **Mockito (`*.mocks.dart`):** файлы, сгенерированные `build_runner` из `@GenerateNiceMocks` рядом с тестами (например `create_project_screen_test.mocks.dart`), **коммитятся в git** вместе с тестами — как и артефакты `build_runner` для приложения (`*.g.dart`, `*.freezed.dart` в `lib/`): без них `flutter test` на чистом клоне не соберётся. Порядок регенерации — тот же, что в абзаце выше (`build_runner` → `gen-l10n`). В `analysis_options.yaml` `*.mocks.dart` **не** исключены из анализа (в отличие от `*.g.dart` / `*.freezed.dart` в `exclude`), поэтому `flutter analyze` проверяет и тесты.

### 2.4. Управление контентом (Data Flow)

  * **Источник правды:** Весь динамический контент (профили, статьи, настройки) должен загружаться с FastAPI бэкенда.
  * **Уровень данных:** Должен существовать четкий "слой репозитория" (Repository Pattern), который абстрагирует получение данных (будь то из API или кэша) от бизнес-логики.
  * **Кэширование:** Для улучшения UX (особенно на мобильных устройствах) рассмотрите стратегии кэширования (например, с использованием `dio_cache_interceptor` или локальной БД типа `hive`/`isar`).

### 2.5. SEO и Web-поддержка

  * **Роутинг:** Используйте пакет, поддерживающий навигацию на основе URL и глубокие ссылки, например, **`go_router`**.
  * **Meta-теги:** Для Web-версии **обязательна** возможность динамически задавать `<title>` и `<meta name="description">` для каждой публичной страницы (роута).
      * *Реализация:* Это можно сделать с помощью `go_router` (в `GoRoute` определить `pageBuilder` и обновить `title`) или используя `dart:html` / `package:flutter_web_plugins` для прямого манипулирования DOM при смене роута.
  * **Доступность:** Убедитесь, что Flutter Web генерирует семантически корректный HTML (используйте `Semantics` виджеты).

### 2.6. Public / Personal Area (Аутентификация)

  * **Разделение роутов:** Четко разделите роуты, доступные всем (Public: `/`, `/login`, `/about`), и роуты, требующие авторизации (Personal: `/dashboard`, `/profile`).
  * **Route Guards:** Используйте "защитники роутов" (например, `redirect` в `go_router`) для автоматического перенаправления неавторизованных пользователей со страниц Personal Area на страницу логина.
  * **Управление состоянием:** Состояние аутентификации (наличие токена, данные пользователя) должно быть доступно глобально (например, через `Provider`/`Notifier` верхнего уровня).

-----

## 3\. Рекомендуемая архитектура (State Management)

Для обеспечения масштабируемости, тестируемости и соблюдения вышеуказанных правил, мы выбираем следующий подход:

### Выбор: Riverpod 3.x (с `riverpod_generator`, `flutter_riverpod` ^3)

**Альтернатива:** BLoC / Cubit (если команда более знакома с ним).

### Описание и структура (Feature-First + Riverpod)

Этот подход сочетает четкое разделение по модулям (features) с гибкостью и мощью Riverpod для управления состоянием и зависимостями.

**Почему Riverpod:**

  * **DI (Внедрение зависимостей):** Легко предоставляет репозитории, сервисы и API-клиенты в бизнес-логику и UI.
  * **Управление состоянием:** `NotifierProvider` (или `AsyncNotifierProvider` для асинхронных данных) идеально подходит для инкапсуляции бизнес-логики.
  * **Тестируемость:** Позволяет легко переопределять (override) провайдеры в тестах.
  * **Compile-safe:** `riverpod_generator` устраняет ошибки времени выполнения.

**Примерная структура директорий для модуля `auth`:**

```
lib/
|
├── features/
│   │
│   ├── auth/
│   │   ├── data/
│   │   │   ├── auth_repository.dart       # (Логика API: login/logout)
│   │   │   └── auth_providers.dart        # (Provider для репозитория)
│   │   │
│   │   ├── domain/
│   │   │   └── user_model.dart            # (Модель данных)
│   │   │
│   │   ├── presentation/ (или application/)
│   │   │   ├── controllers/
│   │   │   │   └── auth_controller.dart     # (StateNotifier/AsyncNotifier - бизнес-логика)
│   │   │   │
│   │   │   ├── screens/
│   │   │   │   └── login_screen.dart        # (Экран, который "потребляет" контроллер)
│   │   │   │
│   │   │   └── widgets/
│   │   │       └── login_form.dart          # (Переиспользуемый виджет формы)
│   │
│   ├── profile/
│   │   └── ... (аналогичная структура)
│
├── core/ (или shared/)
│   ├── api/
│   │   └── dio_client.dart                # (Настройка Dio, Interceptors)
│   ├── routing/
│   │   └── app_router.dart                # (Конфигурация GoRouter)
│   ├── theme/
│   │   └── app_theme.dart                 # (ThemeData)
│   ├── widgets/
│   │   └── custom_button.dart             # (Общий UI Kit)
│
└── main.dart                          # (Инициализация, ProviderScope)
```

### 3.1. Работа с Data Models (Freezed)

Для создания неизменяемых моделей данных используем **`freezed`** с **`json_serializable`**.

#### КРИТИЧНО: Использование abstract class

При использовании `@freezed` **ОБЯЗАТЕЛЬНО** объявляйте класс как `abstract class`:

**❌ НЕПРАВИЛЬНО:**
```dart
@freezed
class UserModel with _$UserModel {  // ← Ошибка компиляции!
  const factory UserModel({
    required String id,
    required String email,
  }) = _UserModel;
  
  factory UserModel.fromJson(Map<String, dynamic> json) =>
      _$UserModelFromJson(json);
}
```

**✅ ПРАВИЛЬНО:**
```dart
@freezed
abstract class UserModel with _$UserModel {  // ← abstract class!
  const factory UserModel({
    required String id,
    required String email,
  }) = _UserModel;
  
  factory UserModel.fromJson(Map<String, dynamic> json) =>
      _$UserModelFromJson(json);
}
```

**Причина:** `@freezed` генерирует mixin `_$UserModel`, который содержит геттеры и методы. Без `abstract class` компилятор требует конкретную реализацию этих членов в основном классе, что приводит к ошибкам типа:

```
Missing concrete implementations of 'getter mixin _$UserModel on Object.id', 
'getter mixin _$UserModel on Object.email', ...
```

#### Обязательные файлы

Каждая freezed-модель **ДОЛЖНА** включать:

```dart
part 'user_model.freezed.dart';  // Freezed code generation
part 'user_model.g.dart';        // JSON serialization
```

#### Генерация кода

После создания или изменения моделей **ОБЯЗАТЕЛЬНО** выполните:

```bash
make frontend-codegen
# или напрямую:
flutter pub run build_runner build --delete-conflicting-outputs
```

**ВАЖНО:** Если после генерации остаются ошибки компиляции, проверьте:
1. Используется ли `abstract class`
2. Присутствуют ли `part` директивы
3. Сгенерированы ли файлы `.freezed.dart` и `.g.dart`

-----

## 4\. Тестирование и Качество Кода

Код без тестов **не принимается**. Тестирование — это не опция, а обязательная часть процесса разработки.

### 4.1. Unit-тесты (Логика)

  * **Что тестируем:** Бизнес-логику.
  * **Где:** Все `Controllers` / `Notifiers` (в Riverpod), все методы `Repository`, любые служебные классы/функции.
  * **Правило:** Любая нетривиальная логика (циклы, условия, форматирование данных) в `Notifier` или `Repository` должна быть покрыта юнит-тестом.
  * **Инструменты:** `test`, `mockito` (или `mocktail`).

### 4.2. Widget-тесты (UI)

  * **Что тестируем:** UI-компоненты.
  * **Где:** Все виджеты из `shared/widgets` (UI Kit) и ключевые "экраны" (`screens`).
  * **Правило:**
      * **UI Kit:** Каждый виджет (например, `CustomButton`) должен быть протестирован на рендеринг и взаимодействие (например, `onTap`).
      * **Экраны:** Тестируется "золотой" путь (успешная загрузка) и альтернативные состояния (загрузка, ошибка).
  * **Инструменты:** `flutter_test`, `riverpod_test` (для `ProviderScope` при тестировании экранов).

### 4.3. Интеграционные (E2E) тесты

  * **Что тестируем:** Ключевые пользовательские сценарии (User Flows).
  * **Где:** "Login -\> Navigate to Profile -\> Logout", "Создание заказа".
  * **Правило:** Минимум 1-2 E2E теста для самых критичных "путей" приложения.
  * **Инструменты:** `integration_test` (предпочтительно) или `patrol`.

-----

## 5\. Orchestration v2 фичи (Sprint 17)

С Sprint 17 во фронтенде появилась v2-надстройка над оркестрацией задач —
admin-инструменты для управления реестром агентов и отладки worktree, плюс
обогащённый Task Detail. Полный план backend-части: [`docs/orchestration-v2-plan.md`](../orchestration-v2-plan.md).
Контекст инвариантов (flow=данные, артефакты через reviewer, `--` separator,
secrets через `agent_secrets`) — в `docs/rules/main.md` §"Orchestration v2".

### 5.1. Расположение v2-фич

| Фича | Путь |
|:---|:---|
| **Agents Management** (CRUD реестра агентов + secrets) | `frontend/lib/features/admin/agents_v2/` |
| **Worktrees debug** (список активных worktree, метаданные, force-cleanup) | `frontend/lib/features/admin/worktrees_v2/` |
| **Task Detail v2 sections** (router timeline, artifacts DAG, artifact viewer, custom timeout editor) | `frontend/lib/features/tasks/presentation/widgets/` |

Все новые v2-виджеты, скрины и провайдеры **обязаны** лежать строго в этих
директориях — не размазывайте логику по `features/admin/` корню или
`features/tasks/presentation/screens/`. Это нужно, чтобы при будущем выкатывании
v2 за feature-flag можно было оборачивать целые подпапки одним `if`-ом.

### 5.2. Локализация в v2-виджетах (ОБЯЗАТЕЛЬНО)

В **новых** v2-виджетах используйте только:

```dart
final l10n = requireAppLocalizations(context, where: 'AgentsListScreen');
// ...
Text(l10n.agentsTitle)
```

Импорт: `package:frontend/core/l10n/require.dart`.

**Запрещено в новом v2-коде:**
  * `AppLocalizations.of(context)!` — допустимо только в **легаси** до точечной миграции (см. §2.3); в `features/admin/agents_v2/`, `features/admin/worktrees_v2/`, и новых файлах в `features/tasks/presentation/widgets/` это считается багом ревью.
  * `context.l10n` — extension в проекте не используется.
  * Хардкод строк `Text("Agents")` — общий запрет из §2.3.

**Параметр `where`:** указывайте имя виджета/экрана (`'AgentsListScreen'`, `'WorktreesDebugTable'`, `'ArtifactViewerDialog'`). Он попадает в текст исключения, если под деревом нет `AppLocalizations.delegate` — без него отладка такой ошибки превращается в гадание.

### 5.3. Freezed-модели в v2 (ОБЯЗАТЕЛЬНО)

Все freezed-модели v2 (`AgentV2`, `WorktreeInfo`, `RouterDecision`, `TaskArtifact`,
`ArtifactNode` и т.д.) **обязаны** объявляться как `abstract class` с миксином
`with _$ModelName`:

```dart
@freezed
abstract class AgentV2 with _$AgentV2 {
  const factory AgentV2({
    required String id,
    required String role,
    required String systemPrompt,
    @Default(<String>[]) List<String> skills,
  }) = _AgentV2;

  factory AgentV2.fromJson(Map<String, dynamic> json) =>
      _$AgentV2FromJson(json);
}
```

**Не** `class AgentV2 with _$AgentV2` (без `abstract`) — на Freezed 3.x это
немедленная ошибка компиляции `Missing concrete implementations of 'getter mixin
_$AgentV2 on Object.id', ...`. Подробности и развёрнутый пример — в §3.1
("Использование abstract class"). Compliance-замечание из задачи 6.7 указывает
именно на эту форму как на канон.

После добавления/правки v2-моделей **обязательно**:

```bash
make frontend-codegen   # build_runner → gen-l10n (порядок критичен, см. §2.3)
```

И закоммитьте сгенерированные `*.freezed.dart` / `*.g.dart` рядом с исходником —
без них CI и чистые клоны не соберутся (§2.3, абзац про "Mockito и build_runner артефакты").

### 5.4. API-клиенты v2

Repository-слой v2-фич (`agents_v2/data/`, `worktrees_v2/data/`,
`tasks/data/orchestration_v2_repository.dart` и т.п.) ходит на новые
admin-ручки бэкенда (`/admin/agents`, `/admin/worktrees`,
`/tasks/:id/artifacts`, `/tasks/:id/router_decisions`). При расширении DTO
на стороне Go бэкенд **обязан** перегенерить swagger (см.
`docs/rules/backend.md` §6.5.1) — фронтовые Dio-клиенты строятся по
`backend/docs/swagger.json`, и stale swagger молча ломает контракт.

### 5.5. Чек-лист перед PR с v2-фичей

  * [ ] Виджет лежит в одной из трёх v2-директорий §5.1 (не в корне `features/admin/` или `screens/`)
  * [ ] Все строки UI через `requireAppLocalizations(context, where: '...')`, ключи добавлены в `app_ru.arb` **и** `app_en.arb`, `make frontend-l10n-check` зелёный
  * [ ] Все новые модели — `@freezed abstract class ModelName with _$ModelName`, `*.freezed.dart` и `*.g.dart` закоммичены
  * [ ] `make frontend-codegen` выполнен в правильном порядке (`build_runner` → `gen-l10n`)
  * [ ] `make frontend-analyze` без warnings
  * [ ] Юнит-тесты на Notifier/Repository + widget-тест на ключевой экран (см. §4)

-----