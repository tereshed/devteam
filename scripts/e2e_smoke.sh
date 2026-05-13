#!/usr/bin/env bash
# Full-stack smoke test (Sprint 14.7 + Sprint 15.e2e per-agent backends).
#
# Один прогон через единственный pipeline проверяет ТРИ способа подключения
# LLM/Claude Code разом — каждый агент в команде сконфигурирован по-своему
# (это и есть основной смысл Sprint 15: per-agent customization).
#
# Матрица агентов:
#   orchestrator → LLM (глобальный ANTHROPIC_API_KEY из backend/.env)
#   planner      → LLM (глобальный ANTHROPIC_API_KEY)
#   developer    → sandbox claude-code, provider_kind=anthropic_oauth        (Sprint 15.B)
#   reviewer     → sandbox claude-code, provider_kind=deepseek               (Sprint 15.e2e: native endpoint)
#   tester       → sandbox claude-code, provider_kind=anthropic_oauth        (Sprint 15.B)
#
# SandboxAuthEnvResolver выбирает по provider_kind:
#   anthropic_oauth  → claude_code_subscriptions(owner) → CLAUDE_CODE_OAUTH_TOKEN
#   deepseek         → user_llm_credentials(owner, deepseek)
#                      → ANTHROPIC_BASE_URL=api.deepseek.com/anthropic + ANTHROPIC_AUTH_TOKEN
# Никакого shared proxy — каждый юзер шлёт со своим ключом напрямую.
#
# Требования (env / backend/.env):
#   GITHUB_PAT                            — PAT, может открывать PR в REPO_URL
#   ANTHROPIC_API_KEY                     — для orchestrator/planner (LLMExecutor)
#   CLAUDE_CODE_OAUTH_ACCESS_TOKEN        — для developer и tester (Sprint 15.B)
#   DEEPSEEK_API_KEY                      — для reviewer (Sprint 15.e2e: per-user creds)
#   ENCRYPTION_KEY                        — в backend/.env, 32 байта hex
#
# Использование:
#   GITHUB_PAT=ghp_xxx ./scripts/e2e_smoke.sh \
#        [--api http://localhost:8080] \
#        [--repo https://github.com/owner/repo] \
#        [--timeout 900]
set -euo pipefail

# ──── параметры ────────────────────────────────────────────────────────────
API="${API:-http://localhost:8080}"
REPO_URL="${REPO_URL:-https://github.com/tereshed/kt-test-repo}"
TIMEOUT="${TIMEOUT:-900}"
PASS="${PASS:-Password123!}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --api)     API="$2"; shift 2 ;;
    --repo)    REPO_URL="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help) sed -n '2,33p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

# ──── требования ──────────────────────────────────────────────────────────
require() { command -v "$1" >/dev/null 2>&1 || { echo "missing dep: $1" >&2; exit 2; }; }
require curl
require jq
require uuidgen
require docker

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "[smoke FAIL] env $name is required" >&2; exit 2
  fi
}

require_env GITHUB_PAT
require_env CLAUDE_CODE_OAUTH_ACCESS_TOKEN
require_env DEEPSEEK_API_KEY

OWNER_REPO=$(echo "$REPO_URL" | sed -E 's|^https?://github\.com/||; s|\.git$||')

# ──── helpers ─────────────────────────────────────────────────────────────
log()  { printf '\033[1;36m[smoke]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[smoke FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

api() {  # api METHOD PATH [JSON-BODY]
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -sS -X "$method" "$API$path" \
      -H "Authorization: Bearer $TOK" \
      -H "Content-Type: application/json" \
      -d "$body"
  else
    curl -sS -X "$method" "$API$path" -H "Authorization: Bearer $TOK"
  fi
}

ysql() {  # ysql "SQL"
  docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -c "$1"
}
ysql_scalar() {
  docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -tA -c "$1" | tr -d '[:space:]'
}

run_go_seeder() {  # run_go_seeder <cmd_subdir> env KEY=VAL ...
  local cmd_dir="$1"; shift
  local backend_dir
  backend_dir="$(dirname "$0")/../backend"
  if [[ ! -d "$backend_dir/cmd/$cmd_dir" ]]; then
    fail "missing Go seeder: backend/cmd/$cmd_dir"
  fi
  ( cd "$backend_dir" && "$@" go run "./cmd/$cmd_dir" )
}

# ──── 0. health ───────────────────────────────────────────────────────────
log "checking backend health at $API"
curl -fsS "$API/health" >/dev/null || fail "backend at $API is not healthy"

# ──── 1. user ─────────────────────────────────────────────────────────────
EMAIL="smoke-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)@example.com"
log "registering user $EMAIL"
TOK=$(curl -sS -X POST "$API/api/v1/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token')
[[ -n "$TOK" && "$TOK" != "null" ]] || fail "register failed"

# ──── 2. project ──────────────────────────────────────────────────────────
PNAME="smoke-mixed-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)"
log "creating project $PNAME pointing at $REPO_URL"
PID=$(api POST /api/v1/projects "$(jq -nc \
  --arg n "$PNAME" --arg url "$REPO_URL" \
  '{name:$n, description:"smoke (mixed agents)", git_provider:"local", git_url:$url}')" | jq -r '.id')
[[ -n "$PID" && "$PID" != "null" ]] || fail "project create failed"
log "project id: $PID"

USER_ID=$(ysql_scalar "SELECT id FROM users WHERE email='$EMAIL';")
TEAM=$(ysql_scalar "SELECT id FROM teams WHERE project_id='$PID' LIMIT 1;")
[[ -n "$TEAM" ]]    || fail "team auto-create failed"
[[ -n "$USER_ID" ]] || fail "user lookup failed"
ENCRYPTION_KEY=$(docker exec wibe_backend printenv ENCRYPTION_KEY || true)
[[ -n "$ENCRYPTION_KEY" ]] || fail "backend has no ENCRYPTION_KEY env"

# ──── 3. seed: Claude Code subscription (developer + tester) ──────────────
log "seeding claude_code_subscription for user $USER_ID"
run_go_seeder seed_claude_code_subscription env \
  USER_ID="$USER_ID" \
  CLAUDE_CODE_OAUTH_ACCESS_TOKEN="$CLAUDE_CODE_OAUTH_ACCESS_TOKEN" \
  CLAUDE_CODE_OAUTH_REFRESH_TOKEN="${CLAUDE_CODE_OAUTH_REFRESH_TOKEN:-}" \
  CLAUDE_CODE_OAUTH_EXPIRES_AT="${CLAUDE_CODE_OAUTH_EXPIRES_AT:-}" \
  ENCRYPTION_KEY="$ENCRYPTION_KEY" \
  >/dev/null

# ──── 4. seed: per-user DeepSeek credential (reviewer) ────────────────────
log "seeding user_llm_credentials.deepseek for user $USER_ID"
run_go_seeder seed_user_llm_credential env \
  USER_ID="$USER_ID" \
  PROVIDER="deepseek" \
  API_KEY="$DEEPSEEK_API_KEY" \
  ENCRYPTION_KEY="$ENCRYPTION_KEY" \
  >/dev/null

# ──── 5. seed: agents (per-agent provider_kind) ───────────────────────────
log "seeding 5 agents — каждый со своим provider_kind"
# orchestrator + planner: LLMExecutor (provider_kind=NULL → глобальный ANTHROPIC_API_KEY)
# developer / tester:   sandbox claude-code, provider_kind=anthropic_oauth
# reviewer:             sandbox claude-code, provider_kind=deepseek
ysql "
INSERT INTO agents (id, name, role, team_id, model, code_backend, provider_kind, is_active, requires_code_context, skills, settings)
VALUES
  (gen_random_uuid(),'orchestrator','orchestrator','$TEAM','claude-haiku-4-5-20251001', NULL,         NULL,              true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'planner',     'planner',     '$TEAM','claude-haiku-4-5-20251001', NULL,         NULL,              true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'developer',   'developer',   '$TEAM','claude-haiku-4-5-20251001', 'claude-code','anthropic_oauth', true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'reviewer',    'reviewer',    '$TEAM','deepseek-chat',             'claude-code','deepseek',        true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'tester',      'tester',      '$TEAM','claude-haiku-4-5-20251001', 'claude-code','anthropic_oauth', true,false,'[]'::jsonb,'{}'::jsonb);
" >/dev/null

AGT_ORCH=$(ysql_scalar "SELECT id FROM agents WHERE team_id='$TEAM' AND role='orchestrator';")
[[ -n "$AGT_ORCH" ]] || fail "orchestrator agent missing"

log "agent matrix:"
ysql "SELECT role, model, COALESCE(code_backend,'(llm)') AS backend, COALESCE(provider_kind,'(global)') AS provider FROM agents WHERE team_id='$TEAM' ORDER BY role;"

# ──── 6. git credential + project link via seed_git_credential ────────────
log "seeding git credential and attaching to project"
(
  cd "$(dirname "$0")/../backend" && \
  PAT="$GITHUB_PAT" USER_ID="$USER_ID" PROJECT_ID="$PID" ENCRYPTION_KEY="$ENCRYPTION_KEY" \
    go run ./cmd/seed_git_credential >/dev/null
)

# ──── 7. POST task ────────────────────────────────────────────────────────
BR="feature/smoke-mixed-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)"
TITLE="Smoke[mixed]: add $BR.md"
DESC="Create file $(basename "$BR").md at the repository root with three lines:
'# Smoke (mixed agents)'
'$BR'
'developer=oauth | reviewer=deepseek-native | tester=oauth'"
log "creating task '$TITLE' on branch $BR"
TID=$(api POST "/api/v1/projects/$PID/tasks" "$(jq -nc \
  --arg t "$TITLE" --arg d "$DESC" --arg a "$AGT_ORCH" --arg b "$BR" \
  '{title:$t, description:$d, assigned_agent_id:$a, branch_name:$b}')" | jq -r '.id')
[[ -n "$TID" && "$TID" != "null" ]] || fail "task create failed"
log "task id: $TID"

LOG_SINCE="$(date -u +%Y-%m-%dT%H:%M:%S)"

# ──── 8. poll task status until terminal ──────────────────────────────────
log "polling task status (timeout ${TIMEOUT}s)"
deadline=$(( $(date +%s) + TIMEOUT ))
STATUS=""
while (( $(date +%s) < deadline )); do
  STATUS=$(api GET "/api/v1/tasks/$TID" | jq -r '.status')
  printf '\r[smoke]   status=%-20s' "$STATUS"
  case "$STATUS" in
    completed|failed|cancelled) printf '\n'; break ;;
  esac
  sleep 5
done

[[ "$STATUS" == "completed" ]] || fail "task did not reach completed (last=$STATUS)"
log "task completed"

# ──── 9. verify PR on GitHub ──────────────────────────────────────────────
log "checking PR for $BR on $OWNER_REPO"
PR_JSON=$(curl -sS -H "Authorization: Bearer $GITHUB_PAT" \
  "https://api.github.com/repos/$OWNER_REPO/pulls?state=open&head=tereshed:$BR&per_page=1")
PR_NUM=$(echo "$PR_JSON" | jq -r '.[0].number // empty')
PR_URL=$(echo "$PR_JSON" | jq -r '.[0].html_url // empty')
[[ -n "$PR_NUM" ]] || fail "no PR opened for $BR"

PR_FILES=$(curl -sS -H "Authorization: Bearer $GITHUB_PAT" \
  "https://api.github.com/repos/$OWNER_REPO/pulls/$PR_NUM/files" | jq -r '.[].filename')
FNAME="$(basename "$BR").md"
echo "$PR_FILES" | grep -qx "$FNAME" || fail "PR #$PR_NUM does not include $FNAME (files: $PR_FILES)"

# ──── 10. security assertions (Sprint 15.37 — no secret leakage) ──────────
backend_logs_since() { docker logs --since "$LOG_SINCE" wibe_backend 2>&1 || true; }

assert_no_leak() {
  local name="$1" value="$2"
  [[ -z "$value" ]] && return 0
  if backend_logs_since | grep -F -q -- "$value"; then
    fail "secret '$name' leaked to backend logs"
  fi
}

log "asserting no secret leaks in backend logs"
assert_no_leak DEEPSEEK_API_KEY                "$DEEPSEEK_API_KEY"
assert_no_leak CLAUDE_CODE_OAUTH_ACCESS_TOKEN  "$CLAUDE_CODE_OAUTH_ACCESS_TOKEN"

printf '\033[1;32m[smoke OK]\033[0m mixed-agents pipeline: PR #%s opened: %s (files: %s)\n' \
  "$PR_NUM" "$PR_URL" "$PR_FILES"
