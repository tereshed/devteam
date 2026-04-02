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

# Устанавливаем goose для миграций
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata postgresql-client
WORKDIR /root/

# Копируем бинарник приложения из builder stage
COPY --from=builder /app/main .

# Копируем goose из builder stage
COPY --from=builder /go/bin/goose /usr/local/bin/goose

# Копируем директорию с миграциями
COPY --from=builder /app/db/migrations /root/db/migrations

# Копируем директорию с промптами
COPY --from=builder /app/prompts /root/prompts

# Копируем агентов и воркфлоу
COPY --from=builder /app/agents /root/agents
COPY --from=builder /app/workflows /root/workflows
COPY --from=builder /app/schedules /root/schedules

# Экспонируем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]

