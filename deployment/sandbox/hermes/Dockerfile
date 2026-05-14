# devteam/sandbox-hermes — Sprint 16.
# Hermes Agent (Nous Research, MIT) — open-source мультипровайдерный AI-агент.
# Используется sandbox-агентами с code_backend=hermes.
#
# Hermes требует Python ≥3.11 (см. pyproject.toml в апстриме). У него нет
# готового npm-пакета как у Claude Code — устанавливаем напрямую через uv
# из конкретного коммита/тэга.
#
# === Supply-chain ===
# - HERMES_REF: tag/commit upstream'а. Для prod обязательно SHA, не main.
# - Base image закреплён по digest (см. sprint15.N2 конвенцию).

ARG HERMES_REPO=https://github.com/NousResearch/hermes-agent.git
# CHANGE-ME — поставить конкретный SHA перед production-сборкой (см. ниже BUILD_ENV guard).
ARG HERMES_REF=v2026.5.7

# Sprint 15.N2 — base image закреплён по digest. Обновление = осознанная PR с
# обновлением SHA и SCA. ARG можно переопределить через --build-arg для dev/CI.
# python:3.13-slim digest вытащен `docker pull` + `docker inspect`.
ARG PYTHON_IMAGE=python@sha256:dc1546eefcbe8caaa1f004f16ab76b204b5e1dbd58ff81b899f21cd40541232f

FROM ${PYTHON_IMAGE}

# ARG, объявленные до FROM, не видны в стадии после FROM — Docker требует
# re-import. Делаем это ОДИН раз сразу после FROM; все RUN ниже подхватывают
# те же значения, повторно объявлять не надо.
ARG HERMES_REPO
ARG HERMES_REF
ARG BUILD_ENV=production

# Sprint 16 + 15.N2 guard: при BUILD_ENV=production HERMES_REF обязан быть SHA
# (7+ hex chars), а не tag/branch. Симметрично с deployment/free-claude-proxy/Dockerfile.
# Локальные сборки (Makefile target sandbox-build-hermes) передают BUILD_ENV=local.
RUN set -e; \
    if [ "${BUILD_ENV}" = "production" ] && ! echo "${HERMES_REF}" | grep -Eq '^[0-9a-f]{7,40}$'; then \
        echo "ERROR: HERMES_REF=${HERMES_REF} is not a commit SHA in production build" >&2; \
        exit 1; \
    fi

LABEL org.opencontainers.image.title="devteam/sandbox-hermes" \
      org.opencontainers.image.description="Sandbox image: Python 3.13 + Hermes Agent CLI for DevTeam"

# Системные зависимости:
#   git           — clone/diff/status в entrypoint
#   ripgrep       — Hermes использует `rg` для поиска в tools (см. их docs/features/tools)
#   ca-certificates — для https-запросов к OpenRouter / Anthropic / etc.
#   curl/wget     — диагностика и одноразовые pull'ы
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        git \
        ripgrep \
        ca-certificates \
        curl \
        wget \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

# Фиксируем UID/GID для детерминизма при монтировании volume (тот же контракт, что у sandbox-claude).
RUN groupadd --gid 1001 sandbox \
    && useradd --uid 1001 --gid 1001 --create-home --home-dir /home/sandbox --shell /bin/bash sandbox \
    && mkdir -p /workspace \
    && chown sandbox:sandbox /workspace

WORKDIR /workspace

# uv — быстрый Python package manager, ставим pip'ом и используем для установки Hermes.
# Параметр --break-system-packages в python:3.13-slim не нужен (slim не помечает env как managed).
RUN pip install --no-cache-dir uv

# Клонируем Hermes из конкретного ref'а и устанавливаем глобально.
# Хитрость: `uv pip install --system .` ставит пакет так, что `hermes` CLI попадает в /usr/local/bin.
# HERMES_REPO/HERMES_REF — из ARG-блока сразу после FROM выше.
RUN set -eux; \
    git clone --depth=1 --branch "${HERMES_REF}" "${HERMES_REPO}" /tmp/hermes-src \
    || (git clone "${HERMES_REPO}" /tmp/hermes-src && git -C /tmp/hermes-src checkout "${HERMES_REF}"); \
    cd /tmp/hermes-src; \
    git --no-pager log -1 --format='hermes upstream commit: %H %s'; \
    uv pip install --system --no-cache .; \
    cd /; \
    bash -c 'rm -rf /tmp/* /tmp/.[!.]* /tmp/..?* 2>/dev/null || true'

# entrypoint после установки CLI: правки скрипта не инвалидируют кэш слоя с Hermes.
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh && chown sandbox:sandbox /entrypoint.sh

USER sandbox

ENV HOME=/home/sandbox \
    # PATH уже включает /usr/local/bin где живёт hermes.
    HERMES_NONINTERACTIVE=1

ENTRYPOINT ["/entrypoint.sh"]
