#!/usr/bin/env bash
# DevTeam sandbox entrypoint (Claude Code) — clone → branch → agent → diff → status.json
#
# Контракт env (согласовать с backend/internal/sandbox/types.go):
#   REPO_URL           — URL для git clone (обязательно, не пусто)
#   BRANCH_NAME        — рабочая ветка задачи (обязательно)
#   Инструкция/контекст: не через большие ENV (ARG_MAX). Раннер кладёт prompt/context в
#   /workspace/prompt.txt и /workspace/context.txt до старта (CopyToContainer). Для локальной
#   отладки при маленьком объёме допустимы TASK_INSTRUCTION / TASK_CONTEXT — см. prepare_files.
#   BASE_REF           — база для diff (ветка на origin), по умолчанию GIT_DEFAULT_BRANCH или main
#   GIT_DEFAULT_BRANCH — fallback, если BASE_REF не задан
#   BACKEND            — ожидается claude-code (по умолчанию claude-code)
#   ANTHROPIC_API_KEY  — обязательно для claude-code (проверка до clone — fast fail)
#   MAX_TURNS          — зарезервировано (CLI 0.2.37 не поддерживает --max-turns; игнорируется)
#
# Артефакты (стабильные пути для оркестратора):
#   /workspace/repo/       — клон
#   /workspace/prompt.txt  — инструкция (файл от раннера или из TASK_* при отладке)
#   /workspace/context.txt — контекст (аналогично)
#   /workspace/agent.log   — stdout/stderr Claude Code
#   /workspace/full.diff   — unified diff (индекс vs origin/<BASE_REF>)
#   /workspace/changes.txt — --stat для краткой сводки
#   /workspace/status.json — итог (JSON), единая точка записи через finalize()

set -euo pipefail

# shellcheck disable=SC2034
CLAUDE_PID=""
CANCELLED=0
LAST_EXIT_CODE=0
PHASE="init"
AGENT_EXIT_CODE=""
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

# Маскирование userinfo в URL для безопасного лога (одна строка в stderr).
# Жадный (.*@): иначе пароль с символом @ (например P@ssw@rd) режется и часть пароля попадает в «хост».
mask_url_for_log() {
  local url="${1:-}"
  if [[ "$url" =~ ^([a-zA-Z][a-zA-Z0-9+.-]*://)(.*@)(.*)$ ]]; then
    printf '%s***@%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[3]}"
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

  # Node есть в образе (5.1); надёжная JSON-сериализация без jq. Пути — из тех же переменных, что и в начале скрипта.
  if ! node <<'NODE'
const fs = require('fs');
const statusPath = process.env.SJ_ARTIFACT_STATUS;
const o = {
  status: process.env.SJ_STATUS,
  success: process.env.SJ_SUCCESS === 'true',
  exit_code: Number(process.env.SJ_EXIT_CODE || 0),
  phase: process.env.SJ_PHASE || 'unknown',
  cancelled: process.env.SJ_CANCELLED === '1',
  branch_name: process.env.SJ_BRANCH_NAME || null,
  base_ref: process.env.SJ_BASE_REF || null,
  commit_hash: process.env.SJ_COMMIT_HASH || null,
  agent_exit_code: process.env.SJ_AGENT_EXIT === '' ? null : Number(process.env.SJ_AGENT_EXIT),
  message: process.env.SJ_MESSAGE || null,
  artifacts: {
    repo: process.env.SJ_ARTIFACT_REPO,
    prompt: process.env.SJ_ARTIFACT_PROMPT,
    context: process.env.SJ_ARTIFACT_CONTEXT,
    agent_log: process.env.SJ_ARTIFACT_AGENT_LOG,
    full_diff: process.env.SJ_ARTIFACT_FULL_DIFF,
    changes: process.env.SJ_ARTIFACT_CHANGES,
    status: process.env.SJ_ARTIFACT_STATUS,
  },
};
try {
  fs.writeFileSync(statusPath, JSON.stringify(o, null, 2), 'utf8');
} catch (e) {
  console.error('entrypoint: Failed to write status.json:', e && e.message ? e.message : e);
  process.exit(1);
}
NODE
  then
    echo "entrypoint: failed to generate status.json (see stderr above)" >&2
  fi
}

trap 'LAST_EXIT_CODE=$?; finalize' EXIT

handle_term() {
  CANCELLED=1
  PHASE="cancelled"
  MESSAGE="${MESSAGE:-Task cancelled (SIGTERM)}"
  if [[ -n "${CLAUDE_PID:-}" ]] && kill -0 "$CLAUDE_PID" 2>/dev/null; then
    kill -TERM "$CLAUDE_PID" 2>/dev/null || true
    local waited=0
    while kill -0 "$CLAUDE_PID" 2>/dev/null && [[ "$waited" -lt 5 ]]; do
      sleep 1
      waited=$((waited + 1))
    done
    if kill -0 "$CLAUDE_PID" 2>/dev/null; then
      kill -KILL "$CLAUDE_PID" 2>/dev/null || true
    fi
    wait "$CLAUDE_PID" 2>/dev/null || true
  fi
  LAST_EXIT_CODE=130
  exit 130
}

trap handle_term SIGTERM

# --- Валидация (до clone и внешнего ввода) ---
if [[ ! -w "/workspace" ]]; then
  echo "Error: /workspace is not writable" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="/workspace not writable"
  exit 1
fi

if is_blank "${REPO_URL:-}"; then
  echo "entrypoint: REPO_URL is required" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="REPO_URL is required"
  exit 1
fi

if is_blank "${BRANCH_NAME:-}"; then
  echo "entrypoint: BRANCH_NAME is required" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="BRANCH_NAME is required"
  exit 1
fi

if [[ "$BRANCH_NAME" == -* ]]; then
  echo "entrypoint: BRANCH_NAME cannot start with a hyphen" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="BRANCH_NAME cannot start with a hyphen"
  exit 1
fi

BACKEND="${BACKEND:-claude-code}"
if [[ "$BACKEND" != "claude-code" ]]; then
  echo "entrypoint: BACKEND must be claude-code for this image (got: ${BACKEND})" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="unsupported BACKEND"
  exit 1
fi

if [[ "$BACKEND" == "claude-code" ]] && is_blank "${ANTHROPIC_API_KEY:-}"; then
  echo "entrypoint: ANTHROPIC_API_KEY is required for claude-code" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="ANTHROPIC_API_KEY is required"
  exit 1
fi

BASE_REF_RESOLVED="${BASE_REF:-${GIT_DEFAULT_BRANCH:-main}}"
if is_blank "$BASE_REF_RESOLVED"; then
  BASE_REF_RESOLVED="main"
fi
export BASE_REF_RESOLVED

# Инструкция и контекст: приоритет файлов от раннера (CopyToContainer); иначе маленький TASK_* для отладки.
PHASE="prepare_files"
if [[ -f "$PROMPT_FILE" ]] && [[ -s "$PROMPT_FILE" ]]; then
  :
elif is_blank "${TASK_INSTRUCTION:-}"; then
  echo "entrypoint: prompt required: inject ${PROMPT_FILE} before start or set TASK_INSTRUCTION (dev only)" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
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
: > "$AGENT_LOG"

# --- Идемпотентная подготовка каталога клона ---
PHASE="prepare_repo_dir"
rm -rf /workspace/repo

# --- clone ---
PHASE="clone"
# Не логируем сырой REPO_URL с токеном в stderr
if ! git clone --depth=50 -- "$REPO_URL" "$REPO_DIR" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git clone failed for $(mask_url_for_log "$REPO_URL") (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git clone failed"
  exit 1
fi

cd "$REPO_DIR"

# Обеспечиваем объекты для базы (shallow): догоняем ветку на origin
PHASE="fetch_base"
if ! git fetch origin --depth=50 -- "${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git fetch failed for BASE_REF=${BASE_REF_RESOLVED} (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git fetch failed"
  exit 1
fi

# --- branch: рабочая ветка от origin/<BASE_REF> ---
# Имя ветки заранее отсекает ведущий «-» (Git такие ref не создаёт) — один атомарный checkout -b без лишнего if/else.
PHASE="branch"
if ! git checkout -b "$BRANCH_NAME" "origin/${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: could not create branch from origin/${BASE_REF_RESOLVED}" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git checkout -b failed"
  exit 1
fi

git config --global user.name "DevTeam Agent"
git config --global user.email "agent@devteam.local"

# --- agent: stdin = prompt + разделитель + context; короткий -p (без больших argv) ---
PHASE="agent"
# Headless: без TTY не ждём интерактива и телеметрию-приглашения (зависания по таймауту оркестратора).
export CLAUDE_INTERACTIVE=0
export ANTHROPIC_TELEMETRY_DISABLED=1
# Claude Code 0.2.37: неинтерактивный режим -p; stdin добавляется к запросу (см. документацию non-interactive).
# --dangerously-skip-permissions: допустимо в изолированном контейнере (сеть к LLM — политика хоста).
{
  cat "$PROMPT_FILE"
  printf '\n---\n'
  cat "$CONTEXT_FILE"
} | claude -p "DevTeam sandbox: полные инструкции и контекст переданы через stdin; работай только в этом репозитории." \
  --cwd "$REPO_DIR" \
  --dangerously-skip-permissions \
  --allowedTools "Bash,Edit,Replace,Read,Write,Glob,NotebookEdit" \
  >>"$AGENT_LOG" 2>&1 &
CLAUDE_PID=$!

set +e
wait "$CLAUDE_PID"
AGENT_EXIT_CODE=$?
set -e

if [[ "${CANCELLED:-0}" -eq 1 ]]; then
  LAST_EXIT_CODE=130
  exit 130
fi

if [[ "$AGENT_EXIT_CODE" -ne 0 ]]; then
  MESSAGE="claude exited with code ${AGENT_EXIT_CODE}"
  LAST_EXIT_CODE="$AGENT_EXIT_CODE"
  PHASE="agent"
  exit "$AGENT_EXIT_CODE"
fi

# --- diff: сначала индексируем всё, включая untracked ---
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

# ref до «--»: иначе после -- Git трактует origin/… как pathspec → пустой diff.
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

COMMIT_HASH="$(git rev-parse HEAD)"

PHASE="done"
MESSAGE="completed"
LAST_EXIT_CODE=0
exit 0
