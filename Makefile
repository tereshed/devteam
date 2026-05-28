COMPOSE_FILE := docker-compose.yml
# Тестовый overlay — добавляется только в test-features* таргетах.
# Gitea (порт 3001) + всё что нужно для PR-gate smoke-тестов.
COMPOSE_TEST_FILE := docker-compose.test.yml
COMPOSE_TEST := docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST_FILE)

# Поддерживаемые stem для sandbox-build-<stem> (deployment/sandbox/<stem>/).
# Sprint 16: добавлен hermes — Hermes Agent (Nous Research, MIT).
# Sprint 17/18: добавлен base — базовый образ с рантаймами.
SANDBOX_BUILDABLE_STEMS := base claude hermes
SANDBOX_BUILD_TARGETS := $(addprefix sandbox-build-,$(SANDBOX_BUILDABLE_STEMS))

.PHONY: help build up down logs test test-unit test-integration test-all validate-agent-prompts \
	check-docker sandbox-build $(SANDBOX_BUILD_TARGETS) \
	free-claude-proxy-build free-claude-proxy-check-ref \
	migrate-create migrate-up migrate-down migrate-status \
	frontend-test frontend-test-unit frontend-test-widget frontend-test-integration frontend-test-ws \
	frontend-analyze frontend-codegen frontend-codegen-watch frontend-l10n-check \
	frontend-run-web frontend-run-android frontend-run-ios \
	frontend-build-web frontend-build-android frontend-build-ios \
	swagger rules \
	test-features test-features-up test-features-backend test-features-frontend test-features-real test-features-e2e-real test-features-down \
	feature-report

# === Управление сервисами ===
build:
	docker-compose -f $(COMPOSE_FILE) build

up:
	docker-compose -f $(COMPOSE_FILE) up -d

down:
	docker-compose -f $(COMPOSE_FILE) down

logs:
	docker-compose -f $(COMPOSE_FILE) logs -f app

# === Тестирование (Backend) ===
# test: полный прогон всех пакетов; -tags=integration подключает файлы с //go:build integration
# (обычные *_test.go без тега тоже выполняются в этом же проходе).
test: test-integration

# Быстрый прогон без тестов с //go:build integration (без БД и прочих интеграций).
test-unit:
	cd backend && go test -race ./... -v

test-integration:
	cd backend && go test -race -tags=integration ./... -v

# === Feature-smoke (интеграционные пирамида, см. docs/integration-tests-plan.md) ===
# Самодостаточные таргеты: сами поднимают yugabyte + gitea через docker-compose.test.yml,
# ждут healthcheck, прогоняют тесты. Tenant-изоляция (UUID user/project), а не DROP SCHEMA.
#
# test-features          — backend + frontend (web) smoke в mock-режиме.
# test-features-backend  — только Go featuresmoke-пакет.
# test-features-frontend — только Flutter integration_test (web).
# test-features-real     — real LLM/Git (FEATURESMOKE_MODE=real, требует ключей в .env).
# test-features-down     — стоп контейнеров + удаление volumes (очистка БД).

test-features-up: check-docker
	@echo ">> Поднимаем тестовый stack (yugabytedb + gitea)..."
	$(COMPOSE_TEST) up -d yugabytedb gitea
	@echo ">> Ждём healthcheck (до 120s)..."
	@deadline=$$(( $$(date +%s) + 120 )); \
	while true; do \
		yb=$$($(COMPOSE_TEST) ps --format json yugabytedb 2>/dev/null | grep -o '"Health":"healthy"' | head -1); \
		gt=$$($(COMPOSE_TEST) ps --format json gitea 2>/dev/null | grep -o '"Health":"healthy"' | head -1); \
		if [ -n "$$yb" ] && [ -n "$$gt" ]; then echo ">> healthy"; break; fi; \
		if [ $$(date +%s) -gt $$deadline ]; then echo ">> ERROR: timeout waiting healthcheck" >&2; $(COMPOSE_TEST) ps; exit 1; fi; \
		sleep 2; \
	done

test-features-backend: test-features-up
	# КРИТИЧНО: env -u снимает реальные LLM-ключи из родительского окружения.
	# Защита от cost-leak: harness.go в mock-режиме поднимает FakeLLM и подсовывает
	# dummy-ключи в child-process; но если он сломается, без -u backend подхватит
	# твой настоящий ANTHROPIC_API_KEY из shell/.env и пойдёт жечь токены
	# (см. инцидент Phase 2: 5,271 calls / 15M tokens утекло так за день).
	cd backend && env \
		-u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u DEEPSEEK_API_KEY \
		-u GEMINI_API_KEY -u QWEN_API_KEY -u OPENROUTER_API_KEY \
		-u CLAUDE_CODE_OAUTH_ACCESS_TOKEN \
		FEATURESMOKE_ENABLED=1 \
		DB_HOST=localhost DB_PORT=5433 DB_USER=yugabyte DB_PASSWORD=yugabyte DB_NAME=yugabyte \
		FEATURESMOKE_GITEA_URL=http://localhost:3001 \
		go test -tags featuresmoke -race -timeout 600s ./test/featuresmoke/... -count=1 -v

test-features-frontend: test-features-up
	# Phase 3 Flutter integration_test/ прогоняется через Go-обёртку
	# (`backend/test/featuresmoke/frontend_e2e_test.go`): она поднимает backend
	# на ПОРТУ 8080 (зашитом в `lib/core/api/dio_providers.dart`), редиректит LLM
	# на FakeLLM, спавнит `flutter test` и проверяет, что `llm_logs` не выросла —
	# cost-leak guard на этом уровне обязателен (Phase 3 не должна жечь токены).
	#
	# Codegen + l10n запускаем СНАРУЖИ обёртки: Go-тест Stream'ит flutter test, но
	# build_runner — это отдельный длительный шаг, лучше держать его прозрачным.
	cd frontend && flutter pub get && \
		dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n
	# КРИТИЧНО (cost-leak prevention, как в test-features-backend): env -u
	# снимает реальные LLM-ключи родительского окружения. Если harness в mock-
	# режиме сломается, backend упадёт с "provider not configured", а не пойдёт
	# на api.anthropic.com с настоящим ключом.
	cd backend && env \
		-u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u DEEPSEEK_API_KEY \
		-u GEMINI_API_KEY -u QWEN_API_KEY -u OPENROUTER_API_KEY \
		-u CLAUDE_CODE_OAUTH_ACCESS_TOKEN \
		FEATURESMOKE_ENABLED=1 FEATURESMOKE_FORCE_PORT=8080 \
		DB_HOST=localhost DB_PORT=5433 DB_USER=yugabyte DB_PASSWORD=yugabyte DB_NAME=yugabyte \
		FEATURESMOKE_GITEA_URL=http://localhost:3001 \
		go test -tags "featuresmoke featuresmoke_frontend" -timeout 1800s \
			./test/featuresmoke/... -run TestFrontendIntegration_Phase3 -count=1 -v

test-features: test-features-backend test-features-frontend

test-features-real: test-features-up
	cd backend && FEATURESMOKE_ENABLED=1 FEATURESMOKE_MODE=real \
		DB_HOST=localhost DB_PORT=5433 DB_USER=yugabyte DB_PASSWORD=yugabyte DB_NAME=yugabyte \
		go test -tags featuresmoke -race -timeout 1800s ./test/featuresmoke/... -count=1 -v

# test-features-e2e-real (Task 5.2, Phase 5): полный pipeline orchestrator →
# planner → developer → reviewer → tester с реальным PR на kt-test-repo.
# Тег `e2ereal` нужен в дополнение к `featuresmoke` — он gate'ит файл
# `e2e_real_test.go`. Так как тест открывает реальный PR в github и тратит
# заметные деньги на LLM (~10-15 центов за прогон), запускается ТОЛЬКО
# nightly + on-demand (см. feature-e2e-real.yml). Локально — для проверки
# регрессий в самом тесте перед merge'ом workflow.
#
# Требует в env: ANTHROPIC_API_KEY (обычно через backend/.env) +
# CLAUDE_CODE_OAUTH_ACCESS_TOKEN, DEEPSEEK_API_KEY, OPENROUTER_API_KEY,
# GITHUB_PAT, ENCRYPTION_KEY. Без них тест делает t.Skip с понятным сообщением.
test-features-e2e-real: test-features-up
	cd backend && FEATURESMOKE_ENABLED=1 FEATURESMOKE_MODE=real \
		DB_HOST=localhost DB_PORT=5433 DB_USER=yugabyte DB_PASSWORD=yugabyte DB_NAME=yugabyte \
		go test -tags "featuresmoke e2ereal" -timeout 1800s \
			./test/featuresmoke/... -run TestE2EReal_MixedAgentsPipeline -count=1 -v

# feature-report (Task 5.3, Phase 5): локальный прогон дашборда поверх
# артефактов go test -json / flutter test --machine. CI вызывает аналогично
# (см. .github/workflows/feature-smoke.yml). Удобно для локальной отладки
# самого репортера: положи свои JSON-файлы в ./artifacts и запусти.
#
# Пример:
#   make feature-report BACKEND_JSON=./artifacts/backend.json FRONTEND_JSON=./artifacts/frontend.json
BACKEND_JSON  ?= ./artifacts/backend-tests.json
FRONTEND_JSON ?= ./artifacts/frontend-tests.json
feature-report:
	mkdir -p artifacts
	cd backend && go run ./cmd/feature_report \
		--backend  ../$(BACKEND_JSON) \
		--frontend ../$(FRONTEND_JSON) \
		--out-md   ../artifacts/feature-report.md \
		--out-html ../artifacts/feature-report.html \
		--title    "Feature smoke (local)" \
		--commit   "$(shell git rev-parse --short HEAD)"

test-features-down:
	@echo ">> Останавливаем тестовый stack и удаляем volumes..."
	$(COMPOSE_TEST) down -v --remove-orphans

# Pipeline agent prompts (task 6.8): YAML vs backend/prompts/prompt_schema.json
validate-agent-prompts:
	cd backend && go test ./pkg/agentprompts -run TestValidateAllYAMLAgainstSchema -count=1

# --- Sandbox images (task 5.12, docs/tasks/5.12-makefile-sandbox-build.md) ---
# Сборка через docker build, не сервис в docker-compose: образы — эфемерные CI/тестовые
# артефакты; compose описывает долгоживущий стек (API, БД). См. раздел Compliance в задаче 5.12.
export DOCKER_BUILDKIT := 1

check-docker:
	@docker info >/dev/null 2>&1 || (echo "Error: Docker Engine is not available (daemon not running or no permissions). Start Docker and retry." >&2 && exit 1)

# Ref -t, -f и контекст в кавычках — защита от flag injection при переопределении переменных make.
# Статическое шаблонное правило: на GNU Make 3.81 (macOS) обычное sandbox-build-% не срабатывает для
# целей из .PHONY — получается «Nothing to be done» без сборки образа.
$(SANDBOX_BUILD_TARGETS): sandbox-build-%: check-docker
	$(if $(filter $*,$(SANDBOX_BUILDABLE_STEMS)),,$(error Unknown sandbox stem '$*'. Expected one of: $(SANDBOX_BUILDABLE_STEMS)))
	# BUILD_ENV=local — отключает prod-guard на SHA-only ref (см. hermes/Dockerfile Sprint 16).
	# Production-сборка делается через CI с явной --build-arg HERMES_REF=<sha>.
	docker build --build-arg BUILD_ENV=local -t "devteam/sandbox-$*:local" -f "deployment/sandbox/$*/Dockerfile" "deployment/sandbox/$*"

sandbox-build-claude: sandbox-build-base
sandbox-build-hermes: sandbox-build-base

sandbox-build: sandbox-build-base sandbox-build-claude sandbox-build-hermes

# Sprint 15.N2 — guard для free-claude-proxy: при BUILD_ENV=production ARG ref ОБЯЗАН быть SHA.
.PHONY: free-claude-proxy-build free-claude-proxy-check-ref
free-claude-proxy-check-ref:
	bash deployment/free-claude-proxy/check_pinned_ref.sh

free-claude-proxy-build: check-docker free-claude-proxy-check-ref
	docker build \
		--build-arg FREE_CLAUDE_PROXY_REF=$${FREE_CLAUDE_PROXY_REF:-main} \
		--build-arg BUILD_ENV=$${BUILD_ENV:-local} \
		-t "devteam/free-claude-proxy:local" \
		-f "deployment/free-claude-proxy/Dockerfile" \
		"deployment/free-claude-proxy"

test-all: test-integration

# === Full-stack smoke (Sprint 14, C-block) ===
# Прогоняет реальный E2E на поднятом стеке: register → project → agents →
# task → poll до completed → проверяет, что в указанном GitHub-репо открыт PR.
# Требует: docker compose up (backend + yugabyte + sandbox-claude:local образ),
# реальный ANTHROPIC_API_KEY в backend/.env, GITHUB_PAT в env.
e2e-smoke:
	@test -n "$$GITHUB_PAT" || { echo "GITHUB_PAT env is required"; exit 2; }
	./scripts/e2e_smoke.sh

# === Тестирование (Frontend) ===

# === Тестирование (Frontend) ===
frontend-test:
	cd frontend && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n && flutter test

# Упростим - все тесты в test/ считаются unit/widget
frontend-test-unit: frontend-test
frontend-test-widget: frontend-test

frontend-test-integration:
	cd frontend && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n && flutter test integration_test/

# Задача 11.2: WebSocket unit-тесты на VM и в Chrome (subprotocol / web).
frontend-test-ws:
	cd frontend && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n && \
	flutter test test/core/api/websocket_service_test.dart test/core/api/websocket_events_test.dart test/core/api/ws_handshake_unauthorized_test.dart && \
	flutter test --platform chrome test/core/api/websocket_service_test.dart test/core/api/websocket_events_test.dart test/core/api/websocket_service_web_test.dart

# === Миграции Базы Данных (YugabyteDB) ===
migrate-create:
	@read -p "Enter migration name: " name; \
	docker-compose -f $(COMPOSE_FILE) run --rm app goose -dir /root/db/migrations postgres "host=yugabytedb port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable" create $$name sql

migrate-up:
	docker-compose -f $(COMPOSE_FILE) run --rm app goose -dir /root/db/migrations postgres "host=yugabytedb port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable" up

migrate-down:
	docker-compose -f $(COMPOSE_FILE) run --rm app goose -dir /root/db/migrations postgres "host=yugabytedb port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable" down

migrate-status:
	docker-compose -f $(COMPOSE_FILE) run --rm app goose -dir /root/db/migrations postgres "host=yugabytedb port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable" status

# === Frontend: Подготовка окружения ===
frontend-setup:
	cd frontend && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n

# === Frontend: Анализ и Проверки ===
frontend-analyze:
	cd frontend && flutter analyze .

# === Frontend: Кодогенерация ===
frontend-codegen:
	cd frontend && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter gen-l10n 

frontend-codegen-watch:
	cd frontend && dart run build_runner watch --delete-conflicting-outputs

# Паритет ключей ARB (ru/en) и зеркало плейсхолдеров; для CI / перед PR.
frontend-l10n-check:
	./scripts/check_l10n_parity.sh

# === Frontend: Сборка (Build) ===
frontend-build-web:
	cd frontend && flutter build web --release

frontend-build-android:
	cd frontend && flutter build apk --release

frontend-build-ios:
	cd frontend && flutter build ios --release

# === Frontend: Запуск (Run) ===
# ВАЖНО: Всегда генерируем l10n перед запуском
# Если изменили аннотации (@Riverpod, @Freezed), сначала запустите 'make frontend-codegen'
frontend-run-web:
	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d chrome

frontend-run-android:
	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d android

frontend-run-ios:
	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d ios

# === Документация ===
swagger:
	cd backend && ~/go/bin/swag init -g cmd/api/main.go -o docs || (go install github.com/swaggo/swag/cmd/swag@latest && ~/go/bin/swag init -g cmd/api/main.go -o docs)

# === AI Правила ===
rules:
	node scripts/sync-ai-rules.js

# === Помощь ===
help:
	@echo "Available commands:"
	@echo ""
	@echo "=== Backend ==="
	@echo "  make build           - Build Docker images"
	@echo "  make up              - Start services"
	@echo "  make down            - Stop services"
	@echo "  make logs            - Show application logs"
	@echo "  make test            - Run all backend tests (unit + integration, ./...)"
	@echo "  make test-unit       - Backend tests without //go:build integration (faster)"
	@echo "  make test-integration - Full backend test suite (-tags=integration ./...)"
	@echo "  make validate-agent-prompts - Validate backend/prompts/*.yaml against prompt_schema.json"
	@echo "  make test-all        - Same as test-integration"
	@echo "  make sandbox-build    - Build default sandbox image (Claude, devteam/sandbox-claude:local)"
	@echo "  make sandbox-build-<stem> - Build a specific sandbox image (e.g. sandbox-build-claude)"
	@echo "  make migrate-create  - Create new migration"
	@echo "  make migrate-up      - Apply migrations"
	@echo "  make migrate-down    - Rollback last migration"
	@echo "  make migrate-status  - Show migration status"
	@echo "  make swagger         - Generate Swagger documentation"
	@echo "  make rules           - Sync AI rules across IDEs (Cursor, Windsurf, Copilot)"
	@echo ""
	@echo "=== Frontend ==="
	@echo "  make frontend-setup           - Setup frontend (pub get, gen-l10n, codegen)"
	@echo "  make frontend-test            - Run all frontend tests"
	@echo "  make frontend-test-integration - Run frontend integration tests"
	@echo "  make frontend-analyze        - Run Flutter analyze"
	@echo "  make frontend-codegen        - Run code generation (build_runner)"
	@echo "  make frontend-codegen-watch   - Watch mode for code generation"
	@echo "  make frontend-l10n-check     - Verify ARB key parity (ru/en), placeholder blocks, name/type match"
	@echo "  make frontend-run-web        - Run frontend on Chrome (with auto-setup)"
	@echo "  make frontend-run-android    - Run frontend on Android (with auto-setup)"
	@echo "  make frontend-run-ios        - Run frontend on iOS (with auto-setup)"
	@echo "  make frontend-build-web      - Build web release"
	@echo "  make frontend-build-android  - Build Android APK release"
	@echo "  make frontend-build-ios      - Build iOS release"
