# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git

# Копируем go mod файлы
COPY go.mod go.sum ./

# Загружаем зависимости (кэшируется если go.mod/go.sum не изменились)
RUN go mod download

# Копируем исходный код
COPY . .

# Устанавливаем swag для генерации документации
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Генерируем документацию
RUN swag init -g cmd/api/main.go

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Собираем одноразовый бинарь миграций (для multi-instance: накат отдельным job, см. compose)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migrate ./cmd/migrate

# Выделенные бинари ролей для горизонтального масштабирования с разделением concern'ов.
# (Альтернатива: тот же ./main с APP_ROLE=worker|scheduler — оба способа работают.)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o scheduler ./cmd/scheduler

# Устанавливаем goose для миграций
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata postgresql-client git
WORKDIR /root/

# Копируем бинарники из builder stage:
#   main      — основной (роль из APP_ROLE, по умолчанию all),
#   migrate   — one-shot миграции,
#   worker    — роль worker (пулы),
#   scheduler — роль scheduler (leader-tasks).
COPY --from=builder /app/main .
COPY --from=builder /app/migrate .
COPY --from=builder /app/worker .
COPY --from=builder /app/scheduler .

# Копируем goose из builder stage
COPY --from=builder /go/bin/goose /usr/local/bin/goose

# Копируем директорию с миграциями
COPY --from=builder /app/db/migrations /root/db/migrations

# Копируем директорию с промптами
COPY --from=builder /app/prompts /root/prompts

# Копируем воркфлоу
COPY --from=builder /app/workflows /root/workflows
COPY --from=builder /app/schedules /root/schedules

# Экспонируем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]
