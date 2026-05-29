# devteam/sandbox-antigravity — изолированный runtime для Antigravity CLI (agy).
# Наследуется от общего базового образа devteam/sandbox-base.

FROM devteam/sandbox-base:local

LABEL org.opencontainers.image.title="devteam/sandbox-antigravity" \
      org.opencontainers.image.description="Sandbox image: Antigravity CLI (agy) on top of sandbox-base"

# Install official Antigravity CLI (agy) as sandbox user
USER sandbox
RUN curl -fsSL https://antigravity.google/cli/install.sh | bash

USER root
# entrypoint после установки CLI: правки скрипта не инвалидируют кэш
COPY entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh && chown sandbox:sandbox /entrypoint.sh

USER sandbox

ENV HOME=/home/sandbox
ENV PATH="/home/sandbox/.local/bin:${PATH}"

ENTRYPOINT ["/entrypoint.sh"]
