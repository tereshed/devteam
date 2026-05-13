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
#   Аутентификация Claude Code (Sprint 15.14): обязателен ровно один из вариантов —
#     1) ANTHROPIC_API_KEY        — классический API-ключ Anthropic
#     2) CLAUDE_CODE_OAUTH_TOKEN  — OAuth-токен от подписки Claude Code (приоритет, если задан)
#     3) ANTHROPIC_AUTH_TOKEN     — Bearer-токен для free-claude-proxy (вместе с ANTHROPIC_BASE_URL,
#        Sprint 15.18); прокси сам ходит к нужному LLM-провайдеру.
#   ANTHROPIC_BASE_URL — опционально, переопределяет endpoint Anthropic API (для free-claude-proxy).
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

# Маскирование userinfo в URL для безопасного лога (одна строка в stderr).
# Жадный (.*@): иначе пароль с символом @ (например P@ssw@rd) режется и часть пароля попадает в «хост».
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

if [[ "$BACKEND" == "claude-code" ]]; then
  # Sprint 15.14: принимаем любую из трёх форм аутентификации:
  #   - CLAUDE_CODE_OAUTH_TOKEN (подписка Claude Code)
  #   - ANTHROPIC_AUTH_TOKEN    (Bearer для free-claude-proxy, обычно с ANTHROPIC_BASE_URL)
  #   - ANTHROPIC_API_KEY       (API-ключ)
  if is_blank "${CLAUDE_CODE_OAUTH_TOKEN:-}" \
    && is_blank "${ANTHROPIC_AUTH_TOKEN:-}" \
    && is_blank "${ANTHROPIC_API_KEY:-}"; then
    echo "entrypoint: claude-code requires one of CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_API_KEY" >&2
    LAST_EXIT_CODE=1
    PHASE="validation"
    MESSAGE="claude-code authentication is required"
    exit 1
  fi
fi

BASE_REF_RESOLVED="${BASE_REF:-${GIT_DEFAULT_BRANCH:-main}}"
if is_blank "$BASE_REF_RESOLVED"; then
  BASE_REF_RESOLVED="main"
fi
export BASE_REF_RESOLVED

# START_REF — с какой ветки начинать локально. Developer стартует с BASE_REF
# (обычно main) и строит feature-ветку; reviewer/tester получают START_REF =
# имя ветки задачи и видят уже пушнутый код developer'а. Если START_REF не
# задан — fallback на BASE_REF (старое поведение).
START_REF="${START_REF:-${BASE_REF_RESOLVED}}"
if is_blank "$START_REF"; then
  START_REF="$BASE_REF_RESOLVED"
fi
export START_REF

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

# Sprint 15.22 — per-agent settings.json и .mcp.json (если поставлены раннером).
# Раннер кладёт их в /workspace/.claude/settings.json и /workspace/.mcp.json.
# Перенесём их в ожидаемые claude-code локации.
# Sprint 15.M4 — defense-in-depth:
#   - 0700 на ~/.claude (только owner-rwx);
#   - 0600 на settings.json (mode сохраняется и при mv/cp);
#   - cp с проверкой exit code: если копирование провалилось, fail-fast (entrypoint не запускает claude
#     с дефолтными настройками, тем самым не маскируя ошибку конфигурации).
PHASE="prepare_agent_settings"
if [[ -f /workspace/.claude/settings.json ]]; then
  if ! mkdir -p "$HOME/.claude"; then
    echo "entrypoint: mkdir -p $HOME/.claude failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: mkdir failed"
    exit 1
  fi
  chmod 0700 "$HOME/.claude"
  if ! cp /workspace/.claude/settings.json "$HOME/.claude/settings.json"; then
    echo "entrypoint: cp settings.json failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: cp failed"
    exit 1
  fi
  chmod 0600 "$HOME/.claude/settings.json"
fi

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

# Обеспечиваем объекты для базы (shallow): догоняем BASE_REF (база для diff)
# и START_REF (точка старта локальной ветки). Если START_REF == BASE_REF —
# второй fetch почти бесплатен (Git вернёт «Already up to date»).
PHASE="fetch_base"
if ! git fetch origin --depth=50 -- "${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git fetch failed for BASE_REF=${BASE_REF_RESOLVED} (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git fetch failed"
  exit 1
fi

START_REF_RESOLVED="$START_REF"
if [[ "$START_REF_RESOLVED" != "$BASE_REF_RESOLVED" ]]; then
  PHASE="fetch_start"
  # Явный refspec — иначе `git fetch origin -- <branch>` обновляет только FETCH_HEAD
  # и refs/remotes/origin/<branch> не создаётся (последующий switch -C origin/<branch> падает).
  # Если ветки на remote нет (developer ещё не пушнул) — мягко падаем на BASE_REF.
  if ! git fetch origin --depth=50 "+refs/heads/${START_REF_RESOLVED}:refs/remotes/origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: START_REF=${START_REF_RESOLVED} not found on origin, falling back to BASE_REF=${BASE_REF_RESOLVED}" >>"$AGENT_LOG"
    START_REF_RESOLVED="$BASE_REF_RESOLVED"
  fi
fi
export START_REF_RESOLVED

# --- branch: рабочая ветка от origin/<START_REF> ---
# git switch -C: создать ветку или переключиться с reset на ref (если имя совпадает с дефолтной после clone — не падаем).
# «--» перед start-point: не истолковать пользовательский ref как опцию (5.4).
PHASE="branch"
if ! git switch -C "$BRANCH_NAME" -- "origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: could not create/switch to branch ${BRANCH_NAME} at origin/${START_REF_RESOLVED}" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git switch -C failed"
  exit 1
fi

git config --global user.name "DevTeam Agent"
git config --global user.email "agent@devteam.local"

# Sprint 15.22 / 15.M4: .mcp.json в корне репозитория, exit-code check + mode 0600.
if [[ -f /workspace/.mcp.json ]]; then
  if ! cp /workspace/.mcp.json "$REPO_DIR/.mcp.json"; then
    echo "entrypoint: cp .mcp.json failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: cp mcp.json failed"
    exit 1
  fi
  chmod 0600 "$REPO_DIR/.mcp.json"
fi

# --- agent: stdin = prompt + разделитель + context; короткий -p (без больших argv) ---
PHASE="agent"
# Headless: без TTY не ждём интерактива и телеметрию-приглашения (зависания по таймауту оркестратора).
export CLAUDE_INTERACTIVE=0
export ANTHROPIC_TELEMETRY_DISABLED=1
# Claude Code 2.x: флаг --cwd удалён, рабочая директория задаётся через cd;
# --bare снимает hooks/keychain/CLAUDE.md auto-discovery (sandbox изолирован).
# --dangerously-skip-permissions: допустимо в изолированном контейнере (сеть к LLM — политика хоста).
CLAUDE_MODEL_ARGS=()
if [[ -n "${DEVTEAM_AGENT_MODEL:-}" ]]; then
  CLAUDE_MODEL_ARGS+=("--model" "${DEVTEAM_AGENT_MODEL}")
fi

# Sprint 15.22 — per-agent permission mode. Если задан через CLAUDE_CODE_PERMISSION_MODE,
# используем его и НЕ передаём --dangerously-skip-permissions (mode уже описывает поведение).
# Без env остаётся прежнее поведение (--dangerously-skip-permissions, обратная совместимость).
CLAUDE_PERMS_ARGS=()
case "${CLAUDE_CODE_PERMISSION_MODE:-}" in
  "")
    CLAUDE_PERMS_ARGS+=("--dangerously-skip-permissions")
    ;;
  bypassPermissions)
    CLAUDE_PERMS_ARGS+=("--dangerously-skip-permissions")
    ;;
  acceptEdits|plan|default)
    CLAUDE_PERMS_ARGS+=("--permission-mode" "${CLAUDE_CODE_PERMISSION_MODE}")
    ;;
  *)
    echo "entrypoint: invalid CLAUDE_CODE_PERMISSION_MODE=${CLAUDE_CODE_PERMISSION_MODE}; falling back to --dangerously-skip-permissions" >&2
    CLAUDE_PERMS_ARGS+=("--dangerously-skip-permissions")
    ;;
esac

(
  cd "$REPO_DIR"
  {
    cat "$PROMPT_FILE"
    printf '\n---\n'
    cat "$CONTEXT_FILE"
  } | claude -p "DevTeam sandbox: полные инструкции и контекст переданы через stdin; работай только в этом репозитории." \
    --bare \
    "${CLAUDE_PERMS_ARGS[@]}" \
    "${CLAUDE_MODEL_ARGS[@]}" \
    --allowedTools "Bash,Edit,Read,Write,Glob,NotebookEdit" \
    >>"$AGENT_LOG" 2>&1
) &
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
  AGENT_FAILED=1
  MESSAGE="claude exited with code ${AGENT_EXIT_CODE}"
  PHASE="agent"
  # Не выходим: ниже собираем diff/артефакты, чтобы не потерять частичную работу агента.
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

# --- commit: если агент что-то изменил (есть staged-изменения), коммитим ---
PHASE="commit"
COMMITTED=0
if ! git diff --cached --quiet; then
  if ! git commit -m "DevTeam agent: ${BRANCH_NAME}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: git commit failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git commit failed"
    exit 1
  fi
  COMMITTED=1
fi

COMMIT_HASH="$(git rev-parse HEAD)"

# --- push: пушим ветку на origin только если был свой коммит.
# Tester/Reviewer ничего не меняют → push'ить нечего (и было бы non-fast-forward
# поверх ветки, уже пушнутой Developer'ом).
# https:// требует GIT_TOKEN (PAT); file:// / ssh:// — без авторизации (локальные интеграционные тесты).
# Токен НИКОГДА не уходит в agent.log: подменяем remote во временной переменной и стрипаем stderr через sed.
PHASE="push"
PUSHED=0
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
  # Маскируем токен в логе перед добавлением в AGENT_LOG (на случай, если git напечатал часть URL).
  sed -E "s|x-access-token:[^@]+@|x-access-token:***@|g" "$PUSH_LOG" >>"$AGENT_LOG"
  rm -f "$PUSH_LOG"
  if [[ "$PUSH_EXIT" -ne 0 ]]; then
    echo "entrypoint: git push failed (exit=${PUSH_EXIT})" >&2
    LAST_EXIT_CODE="$PUSH_EXIT"
    MESSAGE="git push failed"
    exit "$PUSH_EXIT"
  fi
  PUSHED=1
fi

PHASE="done"
if [[ "${AGENT_FAILED:-0}" -eq 1 ]]; then
  LAST_EXIT_CODE="$AGENT_EXIT_CODE"
  # MESSAGE уже задан при падении агента
else
  MESSAGE="completed"
  LAST_EXIT_CODE=0
fi
exit "$LAST_EXIT_CODE"
