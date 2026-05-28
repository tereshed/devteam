# devteam/sandbox-hermes — Hermes Agent (Nous Research, MIT) on top of sandbox-base.
# Наследуется от общего базового образа devteam/sandbox-base.

ARG HERMES_REPO=https://github.com/NousResearch/hermes-agent.git
# CHANGE-ME — поставить конкретный SHA перед production-сборкой (см. ниже BUILD_ENV guard).
ARG HERMES_REF=v2026.5.7

FROM devteam/sandbox-base:local

# Re-import ARGs
ARG HERMES_REPO
ARG HERMES_REF
ARG BUILD_ENV=production

# Guard: при BUILD_ENV=production HERMES_REF обязан быть SHA
RUN set -e; \
    if [ "${BUILD_ENV}" = "production" ] && ! echo "${HERMES_REF}" | grep -Eq '^[0-9a-f]{7,40}$'; then \
        echo "ERROR: HERMES_REF=${HERMES_REF} is not a commit SHA in production build" >&2; \
        exit 1; \
    fi

LABEL org.opencontainers.image.title="devteam/sandbox-hermes" \
      org.opencontainers.image.description="Sandbox image: Hermes Agent CLI on top of sandbox-base"

# Install Mockery for Go testing (since mockery is not a standard debian package)
RUN ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "amd64" ]; then M_ARCH="x86_64"; else M_ARCH="$ARCH"; fi && \
    curl -L "https://github.com/vektra/mockery/releases/download/v2.43.0/mockery_2.43.0_Linux_${M_ARCH}.tar.gz" | tar -xz -C /usr/local/bin mockery

# uv — быстрый Python package manager, ставим pip'ом и используем для установки Hermes.
# Используем --break-system-packages, так как системный Python в Debian помечен как externally-managed.
RUN pip install --no-cache-dir uv --break-system-packages

# Клонируем Hermes из конкретного ref'а и устанавливаем глобально.
# Хитрость: `uv pip install --system .` ставит пакет так, что `hermes` CLI попадает в /usr/local/bin.
# HERMES_REPO/HERMES_REF — из ARG-блока сразу после FROM выше.
RUN set -eux; \
    git clone --depth=1 --branch "${HERMES_REF}" "${HERMES_REPO}" /tmp/hermes-src \
    || (git clone "${HERMES_REPO}" /tmp/hermes-src && git -C /tmp/hermes-src checkout "${HERMES_REF}"); \
    cd /tmp/hermes-src; \
    git --no-pager log -1 --format='hermes upstream commit: %H %s'; \
    uv pip install --system --break-system-packages --no-cache .; \
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
