# free-claude-proxy (Sprint 15.16)
#
# Sidecar, который выставляет Anthropic-совместимый API (/v1/messages) поверх произвольных
# LLM-провайдеров (OpenRouter, DeepSeek, Moonshot, Ollama, Zhipu). Используется sandbox-агентами
# с code_backend=claude-code-via-proxy: вместо ANTHROPIC_API_KEY оркестратор подаёт
# ANTHROPIC_BASE_URL=http://free-claude-proxy:8787 и ANTHROPIC_AUTH_TOKEN=<service-token>.
#
# Образ собирается из апстрима https://github.com/Alishahryar1/free-claude-code.
# Версия пинуется через FREE_CLAUDE_PROXY_REF (тег/коммит). По умолчанию main.
#
# Сборка локально:
#   docker compose --profile free-claude-proxy build free-claude-proxy
# или
#   docker build -t devteam/free-claude-proxy:local deployment/free-claude-proxy
#
# Конфиг проксируется через volume; см. deployment/free-claude-proxy/config.example.yaml.
ARG FREE_CLAUDE_PROXY_REPO=https://github.com/Alishahryar1/free-claude-code.git
ARG FREE_CLAUDE_PROXY_REF=main

FROM alpine/git:latest AS source
ARG FREE_CLAUDE_PROXY_REPO
ARG FREE_CLAUDE_PROXY_REF
WORKDIR /src
RUN git clone --depth=1 --branch "${FREE_CLAUDE_PROXY_REF}" "${FREE_CLAUDE_PROXY_REPO}" . \
    || (git clone "${FREE_CLAUDE_PROXY_REPO}" . && git checkout "${FREE_CLAUDE_PROXY_REF}")

FROM node:20-alpine AS runtime
WORKDIR /app
COPY --from=source /src/ /app/

# Установка зависимостей. Реальная команда зависит от структуры апстрима;
# package.json/Cargo.toml/requirements.txt — поддерживаем три самых частых варианта.
RUN if [ -f package.json ]; then \
        npm ci --omit=dev || npm install --omit=dev; \
    elif [ -f requirements.txt ]; then \
        apk add --no-cache python3 py3-pip && pip3 install --break-system-packages -r requirements.txt; \
    fi

ENV PORT=8787 \
    FREE_CLAUDE_PROXY_CONFIG=/etc/free-claude-proxy/config.yaml \
    NODE_ENV=production

EXPOSE 8787

# Команда старта тоже зависит от апстрима. Пробуем стандартные варианты в порядке убывания вероятности.
CMD if [ -f package.json ] && grep -q '"start"' package.json; then \
        npm start; \
    elif [ -f server.js ]; then \
        node server.js; \
    elif [ -f main.py ]; then \
        python3 main.py; \
    else \
        echo "free-claude-proxy: cannot determine start command; override CMD in compose" >&2 && exit 1; \
    fi
