#!/usr/bin/env bash
# DevTeam sandbox entrypoint (Hermes Agent) — clone → branch → agent → diff → status.json
#
# Sprint 16: симметричный аналог deployment/sandbox/claude/entrypoint.sh для
# code_backend=hermes. Контракт env, путей артефактов и status.json совпадает
# (оркестратор парсит их одинаково).
#
# Контракт env:
#   REPO_URL           — URL для git clone (обязательно)
#   BRANCH_NAME        — рабочая ветка задачи (обязательно)
#   BASE_REF           — база для diff; default = GIT_DEFAULT_BRANCH или main
#   START_REF          — точка старта локальной ветки; default = BASE_REF
#   BACKEND            — должно быть "hermes"
#   DEVTEAM_AGENT_MODEL — каноничный provider/model для Hermes (например
#                         "openrouter/anthropic/claude-haiku-4.5"). Если задан,
#                         entrypoint вызывает `hermes config set model ...` перед задачей.
#   Аутентификация: должен быть хотя бы один из <PROVIDER>_API_KEY (резолвер кладёт
#   нужный в env по agent.provider_kind, см. AgentProviderKind.HermesEnvVar):
#     OPENROUTER_API_KEY, ANTHROPIC_API_KEY, OPENAI_API_KEY, и др.
#   GIT_TOKEN          — опц., PAT для push (https://). Маскируется в логах.
#   TASK_INSTRUCTION / TASK_CONTEXT — fallback'и для локальной отладки (раннер
#                                     обычно кладёт файлы в /workspace/*.txt).
#
# Артефакты (стабильные пути для оркестратора — те же, что у claude):
#   /workspace/repo/       — клон
#   /workspace/prompt.txt  — инструкция
#   /workspace/context.txt — контекст
#   /workspace/agent.log   — stdout/stderr Hermes
#   /workspace/full.diff   — unified diff (индекс vs origin/<BASE_REF>)
#   /workspace/changes.txt — --stat
#   /workspace/status.json — итог (JSON), единая точка записи через finalize()

set -euo pipefail

HERMES_PID=""
CANCELLED=0
LAST_EXIT_CODE=0
PHASE="init"
AGENT_EXIT_CODE=""
AGENT_FAILED=0
COMMIT_HASH=""
MESSAGE=""
FINALIZED=0

REPO_DIR="/workspace/repo"
PROMPT_FILE="/workspace/prompt.txt"
CONTEXT_FILE="/workspace/context.txt"
AGENT_LOG="/workspace/agent.log"
FULL_DIFF="/workspace/full.diff"
CHANGES_TXT="/workspace/changes.txt"
STATUS_JSON="/workspace/status.json"

is_blank() {
  local v="${1:-}"
  [[ -z "${v//[[:space:]]/}" ]]
}

mask_url_for_log() {
  local url="${1:-}"
  if [[ "$url" =~ ^([a-zA-Z][a-zA-Z0-9+.-]*://)(.*@)(.*)$ ]]; then
    printf '%s***@%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[3]}"
  elif [[ "$url" == *"@"* ]]; then
    printf '***@%s\n' "${url#*@}"
  else
    printf '%s\n' "$url"
  fi
}

finalize() {
  if [[ "$FINALIZED" -eq 1 ]]; then
    return 0
  fi
  FINALIZED=1

  local ec="${LAST_EXIT_CODE:-0}"
  local success="false"
  local status="error"

  if [[ "$CANCELLED" -eq 1 ]]; then
    status="cancelled"
    success="false"
  elif [[ "$ec" -eq 0 ]]; then
    status="ok"
    success="true"
  else
    status="error"
    success="false"
  fi

  export SJ_STATUS="$status"
  export SJ_SUCCESS="$success"
  export SJ_EXIT_CODE="$ec"
  export SJ_PHASE="${PHASE:-unknown}"
  export SJ_CANCELLED="${CANCELLED:-0}"
  export SJ_BRANCH_NAME="${BRANCH_NAME:-}"
  export SJ_BASE_REF="${BASE_REF_RESOLVED:-}"
  export SJ_COMMIT_HASH="${COMMIT_HASH:-}"
  export SJ_AGENT_EXIT="${AGENT_EXIT_CODE:-}"
  export SJ_MESSAGE="${MESSAGE:-}"

  export SJ_ARTIFACT_REPO="$REPO_DIR"
  export SJ_ARTIFACT_PROMPT="$PROMPT_FILE"
  export SJ_ARTIFACT_CONTEXT="$CONTEXT_FILE"
  export SJ_ARTIFACT_AGENT_LOG="$AGENT_LOG"
  export SJ_ARTIFACT_FULL_DIFF="$FULL_DIFF"
  export SJ_ARTIFACT_CHANGES="$CHANGES_TXT"
  export SJ_ARTIFACT_STATUS="$STATUS_JSON"

  if ! python3 - <<'PY'
import json, os
out = {
  "status": os.environ.get("SJ_STATUS"),
  "success": os.environ.get("SJ_SUCCESS") == "true",
  "exit_code": int(os.environ.get("SJ_EXIT_CODE") or 0),
  "phase": os.environ.get("SJ_PHASE") or "unknown",
  "cancelled": os.environ.get("SJ_CANCELLED") == "1",
  "branch_name": os.environ.get("SJ_BRANCH_NAME") or None,
  "base_ref": os.environ.get("SJ_BASE_REF") or None,
  "commit_hash": os.environ.get("SJ_COMMIT_HASH") or None,
  "agent_exit_code": int(os.environ["SJ_AGENT_EXIT"]) if os.environ.get("SJ_AGENT_EXIT") else None,
  "message": os.environ.get("SJ_MESSAGE") or "",
  "artifacts": {
    "repo": os.environ.get("SJ_ARTIFACT_REPO"),
    "prompt": os.environ.get("SJ_ARTIFACT_PROMPT"),
    "context": os.environ.get("SJ_ARTIFACT_CONTEXT"),
    "agent_log": os.environ.get("SJ_ARTIFACT_AGENT_LOG"),
    "full_diff": os.environ.get("SJ_ARTIFACT_FULL_DIFF"),
    "changes": os.environ.get("SJ_ARTIFACT_CHANGES"),
    "status": os.environ.get("SJ_ARTIFACT_STATUS"),
  },
}
with open(os.environ["SJ_ARTIFACT_STATUS"], "w", encoding="utf-8") as f:
  json.dump(out, f, ensure_ascii=False, indent=2)
PY
  then
    echo "entrypoint: failed to write ${STATUS_JSON} via python3" >&2
  fi
}

on_signal() {
  CANCELLED=1
  if [[ -n "${HERMES_PID:-}" ]]; then
    kill -TERM "$HERMES_PID" 2>/dev/null || true
    wait "$HERMES_PID" 2>/dev/null || true
  fi
  LAST_EXIT_CODE=130
  PHASE="cancelled"
  MESSAGE="cancelled by signal"
  finalize
  exit 130
}

trap finalize EXIT
trap on_signal INT TERM

mkdir -p /workspace
touch "$AGENT_LOG" "$FULL_DIFF" "$CHANGES_TXT"

# --- validation ---
PHASE="validation"
if is_blank "${REPO_URL:-}"; then
  echo "entrypoint: REPO_URL is required" >&2
  LAST_EXIT_CODE=1
  MESSAGE="REPO_URL is required"
  exit 1
fi

if is_blank "${BRANCH_NAME:-}"; then
  echo "entrypoint: BRANCH_NAME is required" >&2
  LAST_EXIT_CODE=1
  MESSAGE="BRANCH_NAME is required"
  exit 1
fi

if [[ "$BRANCH_NAME" == -* ]]; then
  echo "entrypoint: BRANCH_NAME cannot start with a hyphen" >&2
  LAST_EXIT_CODE=1
  MESSAGE="BRANCH_NAME cannot start with a hyphen"
  exit 1
fi

BACKEND="${BACKEND:-hermes}"
if [[ "$BACKEND" != "hermes" ]]; then
  echo "entrypoint: BACKEND must be hermes for this image (got: ${BACKEND})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="unsupported BACKEND"
  exit 1
fi

# --- auth: должен быть хотя бы один <PROVIDER>_API_KEY в env (резолвер кладёт). ---
HAS_AUTH=0
for var in OPENROUTER_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY KIMI_API_KEY MOONSHOT_API_KEY \
           NOUS_PORTAL_API_KEY NVIDIA_NIM_API_KEY HUGGINGFACE_API_KEY MINIMAX_API_KEY \
           XIAOMI_MIMO_API_KEY ZAI_API_KEY GLM_API_KEY; do
  if [[ -n "${!var:-}" ]]; then
    HAS_AUTH=1
    break
  fi
done
if [[ "$HAS_AUTH" -eq 0 ]]; then
  echo "entrypoint: hermes requires at least one *_API_KEY env (OPENROUTER_API_KEY, ANTHROPIC_API_KEY, …)" >&2
  LAST_EXIT_CODE=1
  MESSAGE="hermes authentication is required"
  exit 1
fi

BASE_REF_RESOLVED="${BASE_REF:-${GIT_DEFAULT_BRANCH:-main}}"
if is_blank "$BASE_REF_RESOLVED"; then
  BASE_REF_RESOLVED="main"
fi
export BASE_REF_RESOLVED

START_REF="${START_REF:-${BASE_REF_RESOLVED}}"
if is_blank "$START_REF"; then
  START_REF="$BASE_REF_RESOLVED"
fi
export START_REF

# --- prepare prompt/context files ---
PHASE="prepare_files"
if [[ -f "$PROMPT_FILE" ]] && [[ -s "$PROMPT_FILE" ]]; then
  :
elif is_blank "${TASK_INSTRUCTION:-}"; then
  echo "entrypoint: prompt required: inject ${PROMPT_FILE} before start or set TASK_INSTRUCTION (dev only)" >&2
  LAST_EXIT_CODE=1
  MESSAGE="prompt required"
  exit 1
else
  printf '%s' "$TASK_INSTRUCTION" > "$PROMPT_FILE"
fi

if [[ -f "$CONTEXT_FILE" ]] && [[ -s "$CONTEXT_FILE" ]]; then
  :
elif [[ -n "${TASK_CONTEXT:-}" ]]; then
  printf '%s' "$TASK_CONTEXT" > "$CONTEXT_FILE"
else
  : > "$CONTEXT_FILE"
fi

# --- clone ---
PHASE="prepare_repo_dir"
rm -rf "$REPO_DIR"

PHASE="clone"
if ! git clone --depth=50 -- "$REPO_URL" "$REPO_DIR" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git clone failed for $(mask_url_for_log "$REPO_URL") (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git clone failed"
  exit 1
fi

cd "$REPO_DIR"

PHASE="fetch_base"
if ! git fetch origin --depth=50 -- "${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git fetch failed for BASE_REF=${BASE_REF_RESOLVED}" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git fetch failed"
  exit 1
fi

START_REF_RESOLVED="$START_REF"
if [[ "$START_REF_RESOLVED" != "$BASE_REF_RESOLVED" ]]; then
  PHASE="fetch_start"
  if ! git fetch origin --depth=50 "+refs/heads/${START_REF_RESOLVED}:refs/remotes/origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: START_REF=${START_REF_RESOLVED} not found on origin, falling back to BASE_REF" >>"$AGENT_LOG"
    START_REF_RESOLVED="$BASE_REF_RESOLVED"
  fi
fi
export START_REF_RESOLVED

PHASE="branch"
if ! git switch -C "$BRANCH_NAME" -- "origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: could not create/switch to branch ${BRANCH_NAME}" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git switch -C failed"
  exit 1
fi

git config --global user.name "DevTeam Agent"
git config --global user.email "agent@devteam.local"

# --- agent: hermes chat -q "<PROMPT>" -m provider/model -Q --yolo ---
# Sprint 16: Hermes принимает prompt как значение флага -q (не stdin); собираем
# единый текст через cat, передаём в argv. ARG_MAX в Linux ≥2MB, чего хватает
# для prompt+context. Если когда-нибудь упрёмся в лимит — переключимся на
# временный файл и `hermes chat -q "$(cat /workspace/instruction.txt)"`.
#
# Флаги Hermes (см. `hermes chat --help`):
#   -q QUERY    : non-interactive single query
#   -Q          : quiet — чисто финальный ответ без spinner/banner (programmatic)
#   -m MODEL    : provider/model-name (мы кладём готовую строку через DEVTEAM_AGENT_MODEL)
#   --yolo      : пропуск всех confirm — аналог claude --dangerously-skip-permissions,
#                 безопасно в изолированном container'е (сеть к LLM — политика хоста).
PHASE="agent"
HERMES_QUERY="$(
  printf 'DevTeam sandbox: полные инструкции и контекст переданы ниже; работай только в этом репозитории (cwd=%s).\n\n--- INSTRUCTION ---\n' "$REPO_DIR"
  cat "$PROMPT_FILE"
  printf '\n\n--- CONTEXT ---\n'
  cat "$CONTEXT_FILE"
)"

HERMES_MODEL_ARGS=()
if [[ -n "${DEVTEAM_AGENT_MODEL:-}" ]]; then
  HERMES_MODEL_ARGS+=("-m" "${DEVTEAM_AGENT_MODEL}")
fi

(
  cd "$REPO_DIR"
  hermes chat \
    -q "$HERMES_QUERY" \
    -Q \
    --yolo \
    "${HERMES_MODEL_ARGS[@]}" \
    >>"$AGENT_LOG" 2>&1
) &
HERMES_PID=$!

set +e
wait "$HERMES_PID"
AGENT_EXIT_CODE=$?
set -e

if [[ "${CANCELLED:-0}" -eq 1 ]]; then
  LAST_EXIT_CODE=130
  exit 130
fi

if [[ "$AGENT_EXIT_CODE" -ne 0 ]]; then
  AGENT_FAILED=1
  MESSAGE="hermes exited with code ${AGENT_EXIT_CODE}"
  PHASE="agent"
  # Не выходим — собираем diff/артефакты, чтобы не потерять частичную работу.
fi

# --- diff ---
PHASE="diff"
git add -A

ORIGIN_BASE="origin/${BASE_REF_RESOLVED}"
if ! git rev-parse --verify --quiet -- "${ORIGIN_BASE}" >/dev/null; then
  if ! git fetch origin --depth=50 -- "${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: could not resolve ${ORIGIN_BASE}" >&2
    LAST_EXIT_CODE=1
    MESSAGE="missing origin base ref after fetch"
    exit 1
  fi
fi

if ! git diff --cached "${ORIGIN_BASE}" -- >"$FULL_DIFF" 2>>"$AGENT_LOG"; then
  echo "entrypoint: git diff --cached failed" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git diff failed"
  exit 1
fi

if ! git diff --cached --stat "${ORIGIN_BASE}" -- >"$CHANGES_TXT" 2>>"$AGENT_LOG"; then
  echo "entrypoint: git diff --stat failed" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git diff --stat failed"
  exit 1
fi

# --- commit ---
PHASE="commit"
COMMITTED=0
if ! git diff --cached --quiet; then
  if ! git commit -m "DevTeam agent (hermes): ${BRANCH_NAME}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: git commit failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git commit failed"
    exit 1
  fi
  COMMITTED=1
fi

COMMIT_HASH="$(git rev-parse HEAD)"

# --- push (только если был свой коммит) ---
PHASE="push"
PUSH_URL=""
if [[ "$COMMITTED" -eq 1 ]]; then
  if [[ -n "${GIT_TOKEN:-}" && "${REPO_URL}" =~ ^https:// ]]; then
    PUSH_URL="$(printf '%s' "${REPO_URL}" | sed -E "s|^https://([^/]*@)?|https://x-access-token:${GIT_TOKEN}@|")"
  elif [[ "${REPO_URL}" =~ ^file:// || "${REPO_URL}" =~ ^ssh:// || "${REPO_URL}" =~ ^git@ ]]; then
    PUSH_URL="$REPO_URL"
  fi
fi
if [[ -n "$PUSH_URL" ]]; then
  PUSH_LOG="$(mktemp)"
  set +e
  git push "$PUSH_URL" "$BRANCH_NAME" >"$PUSH_LOG" 2>&1
  PUSH_EXIT=$?
  set -e
  sed -E "s|x-access-token:[^@]+@|x-access-token:***@|g" "$PUSH_LOG" >>"$AGENT_LOG"
  rm -f "$PUSH_LOG"
  if [[ "$PUSH_EXIT" -ne 0 ]]; then
    echo "entrypoint: git push failed (exit ${PUSH_EXIT})" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git push failed"
    exit 1
  fi
fi

# --- success ---
PHASE="done"
if [[ "$AGENT_FAILED" -eq 1 ]]; then
  LAST_EXIT_CODE="${AGENT_EXIT_CODE:-1}"
  exit "$LAST_EXIT_CODE"
fi
LAST_EXIT_CODE=0
exit 0
