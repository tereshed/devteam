# free-claude-proxy (Sprint 15.16)
#
# Sidecar, выставляющий Anthropic-совместимый API (/v1/messages) поверх произвольных
# LLM-провайдеров (OpenRouter, DeepSeek, Moonshot, Ollama, Zhipu).
# Используется sandbox-агентами с code_backend=claude-code-via-proxy.
#
# === Supply-chain (Sprint 15.B6) ===
# Образ собирается из апстрима https://github.com/Alishahryar1/free-claude-code.
# В production ОБЯЗАНО:
#   1) Зафиксировать FREE_CLAUDE_PROXY_REF на конкретный commit SHA (не main!).
#   2) Привязать base-images по digest (FROM image@sha256:...).
#   3) Прогнать SBOM/SCA, прежде чем пушить тэг.
#
# Локальная разработка:
#   docker compose --profile free-claude-proxy build --build-arg FREE_CLAUDE_PROXY_REF=<sha> free-claude-proxy
# или
#   docker build -t devteam/free-claude-proxy:local --build-arg FREE_CLAUDE_PROXY_REF=<sha> deployment/free-claude-proxy
#
# Конфиг проксируется через volume; см. deployment/free-claude-proxy/config.example.yaml.
ARG FREE_CLAUDE_PROXY_REPO=https://github.com/Alishahryar1/free-claude-code.git
# CHANGE-ME — поставить конкретный SHA перед production-сборкой.
ARG FREE_CLAUDE_PROXY_REF=main

# Sprint 15.N2 — base images закреплены по digest. Эти значения вытащены `docker pull` + `docker inspect`
# на момент написания и зафиксированы в репозитории. Обновление образа = осознанная PR с обновлением SHA
# и SCA. ARG'и оставлены, чтобы CI/dev мог переопределить через --build-arg при необходимости.
ARG ALPINE_GIT_IMAGE=alpine/git@sha256:69eb7d50b94b8c5d0fbfb5cef76a83f81f44ba35e29c4a73aacc78f99e3a8cda
ARG NODE_ALPINE_IMAGE=node@sha256:cffed8cd39d6a380434e6d08116d188c53e70611175cd5ec7700f93f32a935a6

# --- stage: source ---
FROM $ALPINE_GIT_IMAGE AS source
ARG FREE_CLAUDE_PROXY_REPO
ARG FREE_CLAUDE_PROXY_REF
# Sprint 15.minor: fail-closed дефолт. Локальные сборки должны явно передать
# `--build-arg BUILD_ENV=local` (см. Makefile target free-claude-proxy-build),
# иначе прямой `docker build` без аргументов будет требовать SHA в REF.
ARG BUILD_ENV=production
WORKDIR /src
# Sprint 15.Major: in-Dockerfile guard. При BUILD_ENV=production REF обязан быть 7+ hex chars (SHA).
# Это работает даже при прямом `docker build` (без Makefile).
RUN set -e; \
    if [ "${BUILD_ENV}" = "production" ] && ! echo "${FREE_CLAUDE_PROXY_REF}" | grep -Eq '^[0-9a-f]{7,40}$'; then \
        echo "ERROR: FREE_CLAUDE_PROXY_REF=${FREE_CLAUDE_PROXY_REF} is not a commit SHA in production build" >&2; \
        exit 1; \
    fi
# Сначала пробуем clone --branch (быстро, если REF — ветка/тэг), затем fallback на checkout по SHA.
RUN git clone --depth=1 --branch "${FREE_CLAUDE_PROXY_REF}" "${FREE_CLAUDE_PROXY_REPO}" . \
    || (git clone "${FREE_CLAUDE_PROXY_REPO}" . && git checkout "${FREE_CLAUDE_PROXY_REF}") \
    && git --no-pager log -1 --format='free-claude-proxy upstream commit: %H %s' \
    && rm -rf .git

# --- stage: runtime ---
FROM $NODE_ALPINE_IMAGE AS runtime

# non-root user (Sprint 15.B6): процесс прокси никогда не запускается root'ом.
ARG PROXY_UID=10001
ARG PROXY_GID=10001
RUN addgroup -g ${PROXY_GID} proxy \
    && adduser -u ${PROXY_UID} -G proxy -D -s /sbin/nologin proxy

WORKDIR /app
COPY --from=source --chown=proxy:proxy /src/ /app/

# Установка зависимостей.
# - npm: предпочитаем ci (требует package-lock.json) — детерминированно и offline-friendly.
# - Python: ставим в venv пользователя, без --break-system-packages.
# - В конце удаляем build-time apk-кеши.
USER root
RUN set -eux; \
    if [ -f package-lock.json ]; then \
        npm ci --omit=dev; \
    elif [ -f package.json ]; then \
        npm install --omit=dev --no-audit --no-fund; \
    fi; \
    if [ -f requirements.txt ]; then \
        apk add --no-cache python3 py3-pip; \
        python3 -m venv /opt/proxy-venv; \
        /opt/proxy-venv/bin/pip install --no-cache-dir -r requirements.txt; \
        chown -R proxy:proxy /opt/proxy-venv; \
    fi; \
    chown -R proxy:proxy /app; \
    rm -rf /var/cache/apk/* /tmp/*

# Конфиг бэкенд-сайдкара пишет в этот каталог (см. config.example.yaml).
RUN mkdir -p /etc/free-claude-proxy && chown -R proxy:proxy /etc/free-claude-proxy

USER proxy

ENV PORT=8787 \
    FREE_CLAUDE_PROXY_CONFIG=/etc/free-claude-proxy/config.yaml \
    NODE_ENV=production \
    PATH=/opt/proxy-venv/bin:${PATH}

EXPOSE 8787

# Healthcheck-ожидание /healthz: путь конфигурируется в config.yaml апстрима.
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:${PORT:-8787}/healthz || exit 1

# Команда старта подбирается под структуру апстрима. В production-pinned образе нужно
# заменить эту «угадайку» на конкретную команду (`CMD ["node", "dist/server.js"]` или эквивалент).
CMD if [ -f package.json ] && grep -q '"start"' package.json; then \
        npm start; \
    elif [ -f dist/server.js ]; then \
        node dist/server.js; \
    elif [ -f server.js ]; then \
        node server.js; \
    elif [ -f main.py ]; then \
        python3 main.py; \
    else \
        echo "free-claude-proxy: cannot determine start command; override CMD in compose" >&2 && exit 1; \
    fi
