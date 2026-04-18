---
alwaysApply: true
---


# Окружение и Развертывание (Docker & Makefile)


1.  **Контейнеризация (Docker):**

      * Стек: **Go** (Бэкенд), **YugabyteDB** (БД), **Flutter** (Клиент API).
      * Все сервисы (Go, YugabyteDB) **ДОЛЖНЫ** запускаться в `Docker`-контейнерах.
      * Для Go-приложения **ОБЯЗАТЕЛЕН** `Dockerfile` с `multi-stage builds`.
      * Для локальной разработки **ОБЯЗАТЕЛЕН** `docker-compose.yml`.

    **YugabyteDB Порты:**
    
    | Порт | Назначение |
    |------|------------|
    | `5433` | YSQL (PostgreSQL-совместимый API) |
    | `15000` | Admin UI (веб-интерфейс) |
    | `9000` | YCQL (Cassandra-совместимый, опционально) |

    **Важно:** YugabyteDB требует ~30-60 секунд для полной инициализации при первом запуске.

2.  **Локальная Разработка (Makefile):**

      * В корне репозитория **ОБЯЗАТЕЛЬНО** должен находиться `Makefile`.
      * `Makefile` — это **единственный** интерфейс для управления локальным окружением.
      * **ЗАПРЕЩЕНО** запускать `go run main.go` или `goose up` вручную. Все команды должны быть обернуты в `make`.

3.  **Обязательные Команды Makefile (Бэкенд):**

    ```makefile
    # === Управление сервисами ===
    build:
    	docker-compose build
    up:
    	docker-compose up -d
    down:
    	docker-compose down
    logs:
    	docker-compose logs -f app

    # === Тестирование ===
    # (Запускает все типы тестов)
    test: test-unit test-integration

    test-unit:
    	docker-compose run --rm app go test -tags=unit ./...

    test-integration:
    	docker-compose run --rm app go test -tags=integration ./...

    # === Миграции Базы Данных (YugabyteDB) ===
    # Используем goose с диалектом postgres (YSQL совместим)
    # DSN: host=yugabytedb port=5433 user=yugabyte password=yugabyte dbname=yugabyte
    migrate-create:
    	@read -p "Enter migration name: " name; \
    	docker-compose run --rm app goose -dir db/migrations postgres "DSN" create $$name sql

    migrate-up:
    	docker-compose run --rm app goose -dir db/migrations postgres "DSN" up

    migrate-down:
    	docker-compose run --rm app goose -dir db/migrations postgres "DSN" down

    migrate-status:
    	docker-compose run --rm app goose -dir db/migrations postgres "DSN" status

    # === Документация (Backend) ===
    swagger:
    	cd backend && swag init -g cmd/api/main.go -o docs

    # === Frontend: Подготовка окружения ===
    # ВАЖНО: build_runner ДОЛЖЕН выполняться ДО flutter gen-l10n
    # Причина: флаг --delete-conflicting-outputs удаляет файлы, созданные gen-l10n
    frontend-setup:
    	cd frontend && flutter pub get && flutter pub run build_runner build --delete-conflicting-outputs && flutter gen-l10n

    # === Frontend: Анализ и Проверки ===
    frontend-analyze:
        cd frontend && flutter analyze .

    # === Frontend: Кодогенерация ===
    # (Запускает build_runner / riverpod_generator, затем локализацию)
    frontend-codegen:
        cd frontend && flutter pub get && flutter pub run build_runner build --delete-conflicting-outputs && flutter gen-l10n

    frontend-codegen-watch:
        cd frontend && flutter pub run build_runner watch --delete-conflicting-outputs

    # === Frontend: Запуск ===
    # ВАЖНО: build_runner НЕ запускается автоматически.
    # Если меняли аннотации - сначала запустите frontend-codegen
    frontend-run-web:
    	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d chrome

    frontend-run-android:
    	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d android

    frontend-run-ios:
    	cd frontend && flutter pub get && flutter gen-l10n && flutter run -d ios

    # === Frontend: Тестирование ===
    # (Запускает ВСЕ тесты)
    frontend-test:
        cd frontend && flutter test

    frontend-test-unit:
        cd frontend && flutter test --tags=unit

    frontend-test-widget:
        cd frontend && flutter test --tags=widget

    frontend-test-integration:
        cd frontend && flutter test integration_test/

    # === Frontend: Сборка (Build) ===
    frontend-build-web:
        cd frontend && flutter build web --release

    frontend-build-android:
        cd frontend && flutter build apk --release

    frontend-build-ios:
        cd frontend && flutter build ios --release
    ```