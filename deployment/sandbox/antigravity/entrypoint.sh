#!/usr/bin/env bash
# DevTeam sandbox entrypoint (Antigravity CLI) — clone → branch → agent → diff → status.json
#
# Контракт env (согласовать с backend/internal/sandbox/types.go):
#   REPO_URL           — URL для git clone (обязательно, не пусто)
#   BRANCH_NAME        — рабочая ветка задачи (обязательно)
#   Инструкция/контекст: не через большие ENV (ARG_MAX). Раннер кладёт prompt/context в
#   /workspace/prompt.txt и /workspace/context.txt до старта (CopyToContainer). Для локальной
#   отладки при маленьком объёме допустимы TASK_INSTRUCTION / TASK_CONTEXT — см. prepare_files.
#   BASE_REF           — база для diff (ветка на origin), по умолчанию GIT_DEFAULT_BRANCH или main
#   GIT_DEFAULT_BRANCH — fallback, если BASE_REF не задан
#   BACKEND            — ожидается antigravity (по умолчанию antigravity)
#   Аутентификация Antigravity CLI: обязателен ровно один из вариантов —
#     1) ANTIGRAVITY_API_KEY        — API-ключ Antigravity
#     2) ANTIGRAVITY_OAUTH_TOKEN     — OAuth-токен от подписки Antigravity
#   ANTIGRAVITY_BASE_URL — опционально, переопределяет endpoint Antigravity API
#   MAX_TURNS          — зарезервировано (игнорируется)
#
# Артефакты (стабильные пути для оркестратора):
#   /workspace/repo/       — клон
#   /workspace/prompt.txt  — инструкция (файл от раннера или из TASK_* при отладке)
#   /workspace/context.txt — контекст (аналогично)
#   /workspace/agent.log   — stdout/stderr Antigravity CLI
#   /workspace/full.diff   — unified diff (индекс vs origin/<BASE_REF>)
#   /workspace/changes.txt — --stat для краткой сводки
#   /workspace/status.json — итог (JSON), единая точка записи через finalize()

set -euo pipefail

export GOMAXPROCS=1

# shellcheck disable=SC2034
AGY_PID=""
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

  # Node есть в образе. Пути — из тех же переменных, что и в начале скрипта.
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
  if [[ -n "${AGY_PID:-}" ]] && kill -0 "$AGY_PID" 2>/dev/null; then
    kill -TERM "$AGY_PID" 2>/dev/null || true
    local waited=0
    while kill -0 "$AGY_PID" 2>/dev/null && [[ "$waited" -lt 5 ]]; do
      sleep 1
      waited=$((waited + 1))
    done
    if kill -0 "$AGY_PID" 2>/dev/null; then
      kill -KILL "$AGY_PID" 2>/dev/null || true
    fi
    wait "$AGY_PID" 2>/dev/null || true
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

BACKEND="${BACKEND:-antigravity}"
if [[ "$BACKEND" != "antigravity" ]]; then
  echo "entrypoint: BACKEND must be antigravity for this image (got: ${BACKEND})" >&2
  LAST_EXIT_CODE=1
  PHASE="validation"
  MESSAGE="unsupported BACKEND"
  exit 1
fi

# Decode go-keyring-base64 token if present
if [[ "${ANTIGRAVITY_OAUTH_TOKEN:-}" == go-keyring-base64:* ]]; then
  echo "entrypoint: ANTIGRAVITY_OAUTH_TOKEN has go-keyring-base64 prefix, decoding access_token..." >>"$AGENT_LOG"
  export DECODE_TEMP_TOKEN="${ANTIGRAVITY_OAUTH_TOKEN#go-keyring-base64:}"
  if decoded_token=$(node -e '
    try {
      const json = Buffer.from(process.env.DECODE_TEMP_TOKEN, "base64").toString("utf8");
      const obj = JSON.parse(json);
      const tok = (obj.token && obj.token.access_token) || obj.access_token || "";
      if (!tok) {
        console.error("No access_token found in JSON");
        process.exit(1);
      }
      console.log(tok);
    } catch(e) {
      console.error("Failed to parse go-keyring token:", e.message);
      process.exit(1);
    }
  ' 2>>"$AGENT_LOG"); then
    export ANTIGRAVITY_OAUTH_TOKEN="$decoded_token"
    echo "entrypoint: successfully decoded and exported access_token" >>"$AGENT_LOG"
  else
    echo "entrypoint: failed to decode go-keyring-base64 token" >&2
  fi
  unset DECODE_TEMP_TOKEN
fi

if [[ "$BACKEND" == "antigravity" ]]; then
  if is_blank "${ANTIGRAVITY_API_KEY:-}" \
    && is_blank "${ANTIGRAVITY_OAUTH_TOKEN:-}"; then
    echo "entrypoint: antigravity requires one of ANTIGRAVITY_API_KEY, ANTIGRAVITY_OAUTH_TOKEN" >&2
    LAST_EXIT_CODE=1
    PHASE="validation"
    MESSAGE="antigravity authentication is required"
    exit 1
  fi

  # Map Antigravity credentials to Jetski environment variables (agy CLI expects JETSKI_OAUTH_TOKEN)
  if [[ -n "${ANTIGRAVITY_OAUTH_TOKEN:-}" ]]; then
    export JETSKI_OAUTH_TOKEN="$ANTIGRAVITY_OAUTH_TOKEN"
    
    # Pre-populate fallback token file for agy CLI (when keyring is bypassed)
    mkdir -p "$HOME/.gemini/antigravity-cli"
    node -e '
      const fs = require("fs");
      const token = process.env.ANTIGRAVITY_OAUTH_TOKEN;
      const expiry = new Date(Date.now() + 3600 * 1000).toISOString();
      const tokenObj = {
        access_token: token,
        token_type: "Bearer",
        expiry: expiry,
        auth_method: "consumer",
        token: {
          access_token: token,
          token_type: "Bearer",
          expiry: expiry
        }
      };
      fs.writeFileSync(
        process.env.HOME + "/.gemini/antigravity-cli/antigravity-oauth-token",
        JSON.stringify(tokenObj, null, 2),
        { mode: 0o600 }
      );
    '
  fi
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

START_REF_RESOLVED="$START_REF"
if [[ "$START_REF_RESOLVED" != "$BASE_REF_RESOLVED" ]]; then
  PHASE="fetch_start"
  if ! git fetch origin --depth=50 "+refs/heads/${START_REF_RESOLVED}:refs/remotes/origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: START_REF=${START_REF_RESOLVED} not found on origin, falling back to BASE_REF=${BASE_REF_RESOLVED}" >>"$AGENT_LOG"
    START_REF_RESOLVED="$BASE_REF_RESOLVED"
  fi
fi
export START_REF_RESOLVED

# Инструкция и контекст
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

# Перенос настроек в ~/.antigravity/
PHASE="prepare_agent_settings"
if [[ -f /workspace/.claude/settings.json ]]; then
  if ! mkdir -p "$HOME/.antigravity"; then
    echo "entrypoint: mkdir -p $HOME/.antigravity failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: mkdir failed"
    exit 1
  fi
  chmod 0700 "$HOME/.antigravity"
  if ! cp /workspace/.claude/settings.json "$HOME/.antigravity/settings.json"; then
    echo "entrypoint: cp settings.json failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: cp failed"
    exit 1
  fi
  chmod 0600 "$HOME/.antigravity/settings.json"
fi

if [[ -f /workspace/.mcp.json ]]; then
  if ! mkdir -p "$HOME/.antigravity"; then
    echo "entrypoint: mkdir -p $HOME/.antigravity failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: mkdir failed"
    exit 1
  fi
  chmod 0700 "$HOME/.antigravity"
  if ! cp /workspace/.mcp.json "$HOME/.antigravity/mcp.json"; then
    echo "entrypoint: cp .mcp.json failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: cp mcp.json failed"
    exit 1
  fi
  chmod 0600 "$HOME/.antigravity/mcp.json"
fi

# --- Идемпотентная подготовка каталога клона ---
PHASE="prepare_repo_dir"
rm -rf /workspace/repo

# Configure git credentials if token is present
if [[ -n "${GIT_TOKEN:-}" && "${REPO_URL}" =~ ^https?:// ]]; then
  repo_host="$(printf '%s' "$REPO_URL" | sed -E 's|^https?://([^/]+).*|\1|')"
  echo "https://x-access-token:${GIT_TOKEN}@${repo_host}" > /tmp/git-credentials
  git config --global credential.helper 'store --file=/tmp/git-credentials'
fi

# --- clone ---
PHASE="clone"
if ! git clone --depth=50 -- "$REPO_URL" "$REPO_DIR" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git clone failed for $(mask_url_for_log "$REPO_URL") (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git clone failed"
  exit 1
fi

cd "$REPO_DIR"

# Обеспечиваем объекты для базы (shallow)
PHASE="fetch_base"
if ! git fetch origin --depth=50 -- "${BASE_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: git fetch failed for BASE_REF=${BASE_REF_RESOLVED} (see ${AGENT_LOG})" >&2
  LAST_EXIT_CODE=1
  MESSAGE="git fetch failed"
  exit 1
fi

# --- branch ---
PHASE="branch"
if git fetch origin --depth=50 "+refs/heads/${BRANCH_NAME}:refs/remotes/origin/${BRANCH_NAME}" >>"$AGENT_LOG" 2>&1; then
  echo "entrypoint: branch ${BRANCH_NAME} exists on origin, checking out" >>"$AGENT_LOG"
  if ! git switch -C "$BRANCH_NAME" -- "origin/${BRANCH_NAME}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: could not switch to existing branch ${BRANCH_NAME}" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git switch -C existing failed"
    exit 1
  fi
else
  echo "entrypoint: branch ${BRANCH_NAME} not found on origin, creating from origin/${START_REF_RESOLVED}" >>"$AGENT_LOG"
  if ! git switch -C "$BRANCH_NAME" -- "origin/${START_REF_RESOLVED}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: could not create/switch to branch ${BRANCH_NAME} at origin/${START_REF_RESOLVED}" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git switch -C failed"
    exit 1
  fi
fi

INITIAL_COMMIT_HASH="$(git rev-parse HEAD)"

git config --global user.name "DevTeam Agent"
git config --global user.email "agent@devteam.local"

# --- configure git excludes ---
mkdir -p .git/info
cat << 'EOF' >> .git/info/exclude
.antigravity*
.antigravitycli*
.agents/
.agent/
plan_*.json
review_*.json
subtask_*.json
merged_artifact.json
plan_output.json
plan_revised.json
plan_revised_v2.json
review_output.json
EOF

# --- inject env file: «инъекция env-файла» уровня репозитория ---
# Бэкенд стейджит содержимое в /workspace/.inject_env_file и передаёт имя/папку через env.
# Пишем файл в рабочую копию репо ПОСЛЕ checkout и добавляем в .git/info/exclude — он нужен
# агенту/тестам, но НЕ должен попасть в diff/commit/push (защита от утечки секретов).
PHASE="inject_env_file"
INJECT_ENV_FILE_STAGE="/workspace/.inject_env_file"
if [[ -n "${INJECT_ENV_FILE_NAME:-}" && -f "$INJECT_ENV_FILE_STAGE" ]]; then
  inj_name="$INJECT_ENV_FILE_NAME"
  inj_dir="${INJECT_ENV_FILE_DIR:-}"
  if [[ "$inj_name" == *"/"* || "$inj_name" == *".."* || "$inj_dir" == /* || "$inj_dir" == *".."* ]]; then
    echo "entrypoint: injected env file rejected (unsafe name/dir): name=$inj_name dir=$inj_dir" >&2
  else
    inj_target_dir="$REPO_DIR"
    if [[ -n "$inj_dir" ]]; then
      inj_target_dir="$REPO_DIR/$inj_dir"
      mkdir -p "$inj_target_dir"
    fi
    if cp "$INJECT_ENV_FILE_STAGE" "$inj_target_dir/$inj_name"; then
      chmod 0600 "$inj_target_dir/$inj_name"
      if [[ -n "$inj_dir" ]]; then inj_rel="$inj_dir/$inj_name"; else inj_rel="$inj_name"; fi
      mkdir -p .git/info
      printf '/%s\n' "$inj_rel" >> .git/info/exclude
      echo "entrypoint: injected env file -> $inj_rel (git-excluded)" >>"$AGENT_LOG"
    else
      echo "entrypoint: cp injected env file failed" >&2
    fi
  fi
fi

# --- skills (Sprint 22) ---
# Runner кладёт per-agent skills в глобальный каталог ~/.gemini/antigravity/skills
# ДО старта контейнера. Antigravity ищет skills и в workspace (.agents/skills) —
# зеркалим туда для надёжности обнаружения. Каталог в .git/info/exclude (выше),
# поэтому в diff/commit/push не попадает.
PHASE="prepare_skills"
GLOBAL_SKILLS_DIR="$HOME/.gemini/antigravity/skills"
if [[ -d "$GLOBAL_SKILLS_DIR" ]] && [[ -n "$(ls -A "$GLOBAL_SKILLS_DIR" 2>/dev/null)" ]]; then
  if ! mkdir -p "$REPO_DIR/.agents/skills"; then
    echo "entrypoint: mkdir -p .agents/skills failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_skills: mkdir failed"
    exit 1
  fi
  if ! cp -R "$GLOBAL_SKILLS_DIR/." "$REPO_DIR/.agents/skills/"; then
    echo "entrypoint: cp skills to workspace failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_skills: cp failed"
    exit 1
  fi
  echo "entrypoint: skills mirrored to .agents/skills: $(ls "$REPO_DIR/.agents/skills" | tr '\n' ' ')" >>"$AGENT_LOG"
fi

# Copy .mcp.json to repo root just in case
if [[ -f /workspace/.mcp.json ]]; then
  if ! cp /workspace/.mcp.json "$REPO_DIR/.mcp.json"; then
    echo "entrypoint: cp .mcp.json failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="prepare_agent_settings: cp mcp.json failed"
    exit 1
  fi
  chmod 0600 "$REPO_DIR/.mcp.json"
fi

# --- agent: run agy in print/non-interactive mode ---
PHASE="agent"

AGY_PERMS_ARGS=()
case "${CLAUDE_CODE_PERMISSION_MODE:-}" in
  "" | bypassPermissions)
    AGY_PERMS_ARGS+=("--dangerously-skip-permissions")
    ;;
  *)
    AGY_PERMS_ARGS+=("--dangerously-skip-permissions")
    ;;
esac

(
  cd "$REPO_DIR"
  {
    cat "$PROMPT_FILE"
    printf '\n---\n'
    cat "$CONTEXT_FILE"
  } | agy --print \
    "${AGY_PERMS_ARGS[@]}" \
    >>"$AGENT_LOG" 2>&1
) &
AGY_PID=$!

set +e
wait "$AGY_PID"
AGENT_EXIT_CODE=$?
set -e

if [[ "${CANCELLED:-0}" -eq 1 ]]; then
  LAST_EXIT_CODE=130
  exit 130
fi

if grep -q "Authentication required" "$AGENT_LOG" || grep -q "Error: authentication" "$AGENT_LOG" || grep -q "authentication timed out" "$AGENT_LOG"; then
  echo "entrypoint: Antigravity CLI failed to authenticate (see $AGENT_LOG)" >&2
  AGENT_EXIT_CODE=1
  AGENT_FAILED=1
  MESSAGE="Antigravity authentication failed"
  PHASE="agent"
elif [[ "$AGENT_EXIT_CODE" -ne 0 ]]; then
  AGENT_FAILED=1
  MESSAGE="agy exited with code ${AGENT_EXIT_CODE}"
  PHASE="agent"
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
  if ! git commit -m "DevTeam agent: ${BRANCH_NAME}" >>"$AGENT_LOG" 2>&1; then
    echo "entrypoint: git commit failed" >&2
    LAST_EXIT_CODE=1
    MESSAGE="git commit failed"
    exit 1
  fi
  COMMITTED=1
fi

COMMIT_HASH="$(git rev-parse HEAD)"

# --- push ---
PHASE="push"
PUSHED=0
PUSH_URL=""

HAS_UNPUSHED=0
if [[ "$COMMIT_HASH" != "$INITIAL_COMMIT_HASH" ]]; then
  HAS_UNPUSHED=1
elif ! git rev-parse --verify --quiet "origin/${BRANCH_NAME}" >/dev/null; then
  HAS_UNPUSHED=1
elif ! git diff --quiet HEAD "origin/${BRANCH_NAME}"; then
  HAS_UNPUSHED=1
fi

if [[ "$HAS_UNPUSHED" -eq 1 ]]; then
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
  
  if [[ "$PUSH_EXIT" -ne 0 ]] && grep -q -E "rejected|fetch first" "$PUSH_LOG"; then
    echo "entrypoint: git push rejected, attempting git pull --rebase..." >>"$AGENT_LOG"
    set +e
    git pull --rebase "$PUSH_URL" "$BRANCH_NAME" >>"$AGENT_LOG" 2>&1
    pull_exit=$?
    set -e
    
    if [[ "$pull_exit" -ne 0 ]]; then
      echo "entrypoint: git pull --rebase failed, checking for conflicts..." >>"$AGENT_LOG"
      if git status | grep -q -E "rebase|Rebase"; then
        conflicts=$(git diff --name-only --diff-filter=U)
        echo "entrypoint: conflicted files: $conflicts" >>"$AGENT_LOG"
        
        only_gomod_conflicts=true
        for file in $conflicts; do
          if [[ "$file" != "go.mod" && "$file" != "go.sum" ]]; then
            only_gomod_conflicts=false
          fi
        done
        
        if [ "$only_gomod_conflicts" = true ] && [ -n "$conflicts" ]; then
          echo "entrypoint: resolving go.mod/go.sum conflicts automatically..." >>"$AGENT_LOG"
          git checkout --ours go.mod go.sum >>"$AGENT_LOG" 2>&1 || true
          git add go.mod go.sum >>"$AGENT_LOG" 2>&1
          GIT_EDITOR=true git rebase --continue >>"$AGENT_LOG" 2>&1 || true
          
          echo "entrypoint: regenerating go.mod/go.sum via go mod tidy..." >>"$AGENT_LOG"
          go mod tidy >>"$AGENT_LOG" 2>&1 || true
          git add go.mod go.sum >>"$AGENT_LOG" 2>&1
          git commit --amend --no-edit >>"$AGENT_LOG" 2>&1 || true
        else
          echo "entrypoint: conflicts cannot be auto-resolved, aborting rebase" >>"$AGENT_LOG"
          git rebase --abort >>"$AGENT_LOG" 2>&1 || true
        fi
      fi
    fi
    
    git push "$PUSH_URL" "$BRANCH_NAME" >"$PUSH_LOG" 2>&1
    PUSH_EXIT=$?
  fi
  
  set -e
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
else
  MESSAGE="completed"
  LAST_EXIT_CODE=0
fi
exit "$LAST_EXIT_CODE"
