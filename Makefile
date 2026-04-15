COMPOSE_FILE := docker-compose.yml

# Поддерживаемые stem для sandbox-build-<stem> (deployment/sandbox/<stem>/).
SANDBOX_BUILDABLE_STEMS := claude
SANDBOX_BUILD_TARGETS := $(addprefix sandbox-build-,$(SANDBOX_BUILDABLE_STEMS))

.PHONY: help build up down logs test test-unit test-integration \
	check-docker sandbox-build $(SANDBOX_BUILD_TARGETS) \
	migrate-create migrate-up migrate-down migrate-status \
	frontend-test frontend-test-unit frontend-test-widget frontend-test-integration \
	frontend-analyze frontend-codegen frontend-codegen-watch \
	frontend-run-web frontend-run-android frontend-run-ios \
	frontend-build-web frontend-build-android frontend-build-ios \
	swagger

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
test: test-unit test-integration

test-unit:
	cd backend && go test -race ./internal/handler/... ./internal/service/... ./internal/mcp/... ./internal/config/... ./pkg/crypto/... ./pkg/gitprovider/... -v

test-integration:
	cd backend && go test -race -tags=integration ./internal/repository/... ./internal/sandbox/... ./pkg/gitprovider/... -v

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
	docker build -t "devteam/sandbox-$*:local" -f "deployment/sandbox/$*/Dockerfile" "deployment/sandbox/$*"

sandbox-build: sandbox-build-claude

test-all:
	cd backend && go test ./... -v

# === Тестирование (Frontend) ===
frontend-test:
	cd frontend && flutter pub get && flutter gen-l10n && flutter pub run build_runner build --delete-conflicting-outputs && flutter test

# Упростим - все тесты в test/ считаются unit/widget
frontend-test-unit: frontend-test
frontend-test-widget: frontend-test

frontend-test-integration:
	cd frontend && flutter pub get && flutter gen-l10n && flutter test integration_test/

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
	cd frontend && flutter pub get && flutter pub run build_runner build --delete-conflicting-outputs && flutter gen-l10n

# === Frontend: Анализ и Проверки ===
frontend-analyze:
	cd frontend && flutter analyze .

# === Frontend: Кодогенерация ===
frontend-codegen:
	cd frontend && flutter pub get && flutter pub run build_runner build --delete-conflicting-outputs && flutter gen-l10n 

frontend-codegen-watch:
	cd frontend && flutter pub run build_runner watch --delete-conflicting-outputs

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

# === Помощь ===
help:
	@echo "Available commands:"
	@echo ""
	@echo "=== Backend ==="
	@echo "  make build           - Build Docker images"
	@echo "  make up              - Start services"
	@echo "  make down            - Stop services"
	@echo "  make logs            - Show application logs"
	@echo "  make test            - Run all backend tests"
	@echo "  make test-unit       - Run backend unit tests (handler, service, mcp)"
	@echo "  make test-integration - Run backend integration tests"
	@echo "  make sandbox-build    - Build default sandbox image (Claude, devteam/sandbox-claude:local)"
	@echo "  make sandbox-build-<stem> - Build a specific sandbox image (e.g. sandbox-build-claude)"
	@echo "  make migrate-create  - Create new migration"
	@echo "  make migrate-up      - Apply migrations"
	@echo "  make migrate-down    - Rollback last migration"
	@echo "  make migrate-status  - Show migration status"
	@echo "  make swagger         - Generate Swagger documentation"
	@echo ""
	@echo "=== Frontend ==="
	@echo "  make frontend-setup           - Setup frontend (pub get, gen-l10n, codegen)"
	@echo "  make frontend-test            - Run all frontend tests"
	@echo "  make frontend-test-integration - Run frontend integration tests"
	@echo "  make frontend-analyze        - Run Flutter analyze"
	@echo "  make frontend-codegen        - Run code generation (build_runner)"
	@echo "  make frontend-codegen-watch   - Watch mode for code generation"
	@echo "  make frontend-run-web        - Run frontend on Chrome (with auto-setup)"
	@echo "  make frontend-run-android    - Run frontend on Android (with auto-setup)"
	@echo "  make frontend-run-ios        - Run frontend on iOS (with auto-setup)"
	@echo "  make frontend-build-web      - Build web release"
	@echo "  make frontend-build-android  - Build Android APK release"
	@echo "  make frontend-build-ios      - Build iOS release"
