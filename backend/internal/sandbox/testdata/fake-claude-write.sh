#!/bin/sh
# Integration test: имитирует Claude Code, который создаёт файл в текущем
# репозитории и завершается с 0. Используется в sandbox_real_test.go,
# чтобы убедиться, что entrypoint собирает diff, делает commit и push в bare-remote
# без реального вызова Anthropic API.
set -eu
# Текущая директория задаётся `cd "$REPO_DIR"` в entrypoint перед запуском claude.
printf '# DevTeam Fake Agent\n' > FAKE_AGENT.md
exit 0
