# devteam/sandbox-claude — изолированный runtime для Claude Code CLI.
# Наследуется от общего базового образа devteam/sandbox-base.

FROM devteam/sandbox-base:local

LABEL org.opencontainers.image.title="devteam/sandbox-claude" \
      org.opencontainers.image.description="Sandbox image: Claude Code CLI on top of sandbox-base"

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
