# devteam/sandbox-claude — изолированный runtime для Claude Code CLI (Sprint 5).
# Общий слой с devteam/sandbox-aider пока дублируется; при появлении второго образа
# вынести в devteam/sandbox-base (apt + user + /workspace) — см. docs/tasks/5.1-dockerfile-sandbox-claude.md

ARG NODE_VERSION=20-bookworm-slim
FROM node:${NODE_VERSION}

# Статические лейблы: не привязаны к версии CLI — не инвалидируют кэш при бампе CLAUDE_CODE_VERSION
LABEL org.opencontainers.image.title="devteam/sandbox-claude" \
      org.opencontainers.image.description="Sandbox image: Node.js, git, Claude Code CLI for DevTeam agent tasks"

# Тяжёлые системные слои (кэшируются максимально долго при смене только версии claude-code)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        git \
        ca-certificates \
        curl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

# Фиксируем UID/GID для детерминизма при монтировании volume с хоста (см. задачу 5.5)
RUN groupadd --gid 1001 sandbox \
    && useradd --uid 1001 --gid 1001 --create-home --home-dir /home/sandbox --shell /bin/bash sandbox \
    && mkdir -p /workspace \
    && chown sandbox:sandbox /workspace

WORKDIR /workspace

# Версия CLI и лейбл — только перед установкой npm: смена CLAUDE_CODE_VERSION не сбрасывает apt/user слои
ARG CLAUDE_CODE_VERSION=2.1.126
LABEL devteam.sandbox.claude-code.version="${CLAUDE_CODE_VERSION}"

RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
    && npm cache clean --force \
    && bash -c 'rm -rf /tmp/* /tmp/.[!.]* /tmp/..?* 2>/dev/null || true'

# entrypoint после npm: правки скрипта не инвалидируют кэш слоя с CLI
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh && chown sandbox:sandbox /entrypoint.sh

USER sandbox

ENV HOME=/home/sandbox

ENTRYPOINT ["/entrypoint.sh"]
