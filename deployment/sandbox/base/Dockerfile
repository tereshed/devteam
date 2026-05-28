# devteam/sandbox-base — базовый образ с рантаймами для sandbox-агентов (Sprint 17/18).
# Содержит: Node.js, Python, Go, Java JDK, Rust, Git, Ripgrep.
# Используется как FROM для devteam/sandbox-claude и devteam/sandbox-hermes.

ARG NODE_VERSION=20-bookworm-slim
FROM node:${NODE_VERSION}

LABEL org.opencontainers.image.title="devteam/sandbox-base" \
      org.opencontainers.image.description="Base sandbox image with Node.js, Python, Go, Java JDK, Rust, Git, and Ripgrep"

# Устанавливаем системные зависимости и компиляторы/рантаймы:
#   - git, ca-certificates, curl, wget, build-essential (базовые инструменты сборки)
#   - ripgrep (для быстрого поиска кода)
#   - python3, python3-pip, python3-venv, python3-dev (Python 3)
#   - golang (Go)
#   - default-jdk (Java JDK)
#   - rustc, cargo (Rust)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        git \
        ripgrep \
        ca-certificates \
        curl \
        wget \
        build-essential \
        python3 \
        python3-pip \
        python3-venv \
        python3-dev \
        default-jdk \
        rustc \
        cargo \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

# Устанавливаем официальный компилятор Go 1.22.12 (так как debian apt golang слишком старый - 1.19)
RUN ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "amd64" ]; then G_ARCH="amd64"; else G_ARCH="arm64"; fi && \
    curl -L "https://go.dev/dl/go1.22.12.linux-${G_ARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Фиксируем UID/GID для детерминизма при монтировании volume с хоста (согласовано с sandbox-claude)
RUN groupadd --gid 1001 sandbox \
    && useradd --uid 1001 --gid 1001 --create-home --home-dir /home/sandbox --shell /bin/bash sandbox \
    && mkdir -p /workspace \
    && chown sandbox:sandbox /workspace

WORKDIR /workspace
