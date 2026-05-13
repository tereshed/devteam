#!/usr/bin/env bash
# Sprint 15.M4 — CI guard: блокирует production-сборку free-claude-proxy если
# FREE_CLAUDE_PROXY_REF не закреплён на конкретный commit SHA.
#
# Использование в CI:
#   bash deployment/free-claude-proxy/check_pinned_ref.sh
#
# Принимает SHA из stdin/env:
#   FREE_CLAUDE_PROXY_REF=<sha>
# или из Dockerfile (ARG ... main → провал).
#
# В local-dev контексте (BUILD_ENV != production) гард пропускает, чтобы не блокировать
# `docker compose --profile free-claude-proxy up`.

set -euo pipefail

env_name="${BUILD_ENV:-local}"
ref="${FREE_CLAUDE_PROXY_REF:-}"

if [[ -z "${ref}" ]]; then
    # Читаем дефолт из Dockerfile.
    here="$(cd "$(dirname "$0")" && pwd)"
    ref="$(awk -F'=' '/^ARG FREE_CLAUDE_PROXY_REF=/ {print $2; exit}' "${here}/Dockerfile" || true)"
fi

# SHA = 40 hex chars или 7+ short hex.
if [[ "${ref}" =~ ^[0-9a-f]{7,40}$ ]]; then
    echo "free-claude-proxy: REF pinned to ${ref}" >&2
    exit 0
fi

if [[ "${env_name}" != "production" ]]; then
    echo "free-claude-proxy: REF=${ref} (not a SHA) — allowed in BUILD_ENV=${env_name}" >&2
    exit 0
fi

cat >&2 <<EOF
ERROR: free-claude-proxy: FREE_CLAUDE_PROXY_REF=${ref} is not a commit SHA.
Pin it to a specific commit before building for production:
    docker build --build-arg FREE_CLAUDE_PROXY_REF=<40-char-sha> deployment/free-claude-proxy
EOF
exit 1
