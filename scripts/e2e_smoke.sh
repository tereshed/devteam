#!/usr/bin/env bash
# Full-stack smoke test (Sprint 14 — C-block):
# Прогоняет реальный pipeline на поднятом стеке (docker compose up + Anthropic
# ключ в backend/.env + PAT для GitHub в env GITHUB_PAT) и проверяет, что
# в результате на GitHub появился PR с веткой, созданной задачей.
#
# Использование:
#   GITHUB_PAT=ghp_xxx ./scripts/e2e_smoke.sh \
#        [--api http://localhost:8080] \
#        [--repo https://github.com/owner/repo] \
#        [--timeout 600]
#
# Скрипт идемпотентен: создаёт нового пользователя и новый проект на каждый
# прогон, чтобы не конфликтовать с предыдущими попытками.
set -euo pipefail

# ──── параметры ────────────────────────────────────────────────────────────
API="${API:-http://localhost:8080}"
REPO_URL="${REPO_URL:-https://github.com/tereshed/kt-test-repo}"
TIMEOUT="${TIMEOUT:-600}"   # сек
PASS="${PASS:-Password123!}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --api)     API="$2"; shift 2 ;;
    --repo)    REPO_URL="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,18p' "$0"
      exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

# ──── требования ──────────────────────────────────────────────────────────
require() { command -v "$1" >/dev/null 2>&1 || { echo "missing dep: $1" >&2; exit 2; }; }
require curl
require jq
require uuidgen

if [[ -z "${GITHUB_PAT:-}" ]]; then
  echo "GITHUB_PAT env is required (token must be able to create PRs in $REPO_URL)" >&2
  exit 2
fi

OWNER_REPO=$(echo "$REPO_URL" | sed -E 's|^https?://github\.com/||; s|\.git$||')

# ──── helpers ─────────────────────────────────────────────────────────────
log() { printf '\033[1;36m[smoke]\033[0m %s\n' "$*"; }
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

# ──── 0. health ───────────────────────────────────────────────────────────
log "checking backend health at $API"
curl -fsS "$API/health" >/dev/null || fail "backend at $API is not healthy (start with: docker compose up -d)"

# ──── 1. user ─────────────────────────────────────────────────────────────
EMAIL="smoke-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)@example.com"
log "registering user $EMAIL"
TOK=$(curl -sS -X POST "$API/api/v1/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token')
[[ -n "$TOK" && "$TOK" != "null" ]] || fail "register failed"

# ──── 2. project ──────────────────────────────────────────────────────────
PNAME="smoke-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)"
log "creating project $PNAME pointing at $REPO_URL"
PID=$(api POST /api/v1/projects "$(jq -nc \
  --arg n "$PNAME" --arg url "$REPO_URL" \
  '{name:$n, description:"smoke", git_provider:"local", git_url:$url}')" | jq -r '.id')
[[ -n "$PID" && "$PID" != "null" ]] || fail "project create failed"
log "project id: $PID"

# ──── 3. team + agents (через прямую SQL вставку — отдельного API нет) ────
log "seeding 5 pipeline agents via psql inside yugabytedb container"
TEAM=$(docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -tA \
  -c "SELECT id FROM teams WHERE project_id='$PID' LIMIT 1;" | tr -d '[:space:]')
[[ -n "$TEAM" ]] || fail "team auto-create failed"

docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -c "
INSERT INTO agents (id, name, role, team_id, model, code_backend, is_active, requires_code_context, skills, settings)
VALUES
  (gen_random_uuid(),'orchestrator','orchestrator','$TEAM','claude-haiku-4-5-20251001',NULL,         true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'planner',     'planner',     '$TEAM','claude-haiku-4-5-20251001',NULL,         true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'developer',   'developer',   '$TEAM','claude-haiku-4-5-20251001','claude-code',true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'reviewer',    'reviewer',    '$TEAM','claude-haiku-4-5-20251001','claude-code',true,false,'[]'::jsonb,'{}'::jsonb),
  (gen_random_uuid(),'tester',      'tester',      '$TEAM','claude-haiku-4-5-20251001','claude-code',true,false,'[]'::jsonb,'{}'::jsonb);" >/dev/null
AGT_ORCH=$(docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -tA \
  -c "SELECT id FROM agents WHERE team_id='$TEAM' AND role='orchestrator';" | tr -d '[:space:]')
[[ -n "$AGT_ORCH" ]] || fail "orchestrator agent missing"

# ──── 4. git credential + project link via seed_git_credential ────────────
log "seeding git credential and attaching to project"
USER_ID=$(docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -tA \
  -c "SELECT id FROM users WHERE email='$EMAIL';" | tr -d '[:space:]')
ENCRYPTION_KEY=$(docker exec wibe_backend printenv ENCRYPTION_KEY || true)
[[ -n "$ENCRYPTION_KEY" ]] || fail "backend has no ENCRYPTION_KEY env"

(
  cd "$(dirname "$0")/../backend" && \
  PAT="$GITHUB_PAT" USER_ID="$USER_ID" PROJECT_ID="$PID" ENCRYPTION_KEY="$ENCRYPTION_KEY" \
    go run ./cmd/seed_git_credential >/dev/null
)

# ──── 5. POST task ────────────────────────────────────────────────────────
BR="feature/smoke-$(uuidgen | tr 'A-Z' 'a-z' | cut -c1-8)"
TITLE="Smoke: add $BR.md"
DESC="Create file $(basename "$BR").md at the repository root with two lines: '# Smoke' and '$BR'."
log "creating task '$TITLE' on branch $BR"
TID=$(api POST "/api/v1/projects/$PID/tasks" "$(jq -nc \
  --arg t "$TITLE" --arg d "$DESC" --arg a "$AGT_ORCH" --arg b "$BR" \
  '{title:$t, description:$d, assigned_agent_id:$a, branch_name:$b}')" | jq -r '.id')
[[ -n "$TID" && "$TID" != "null" ]] || fail "task create failed"
log "task id: $TID"

# ──── 6. poll task status until terminal ─────────────────────────────────
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

# ──── 7. verify PR on GitHub ──────────────────────────────────────────────
log "checking PR for $BR on $OWNER_REPO"
PR_JSON=$(curl -sS -H "Authorization: Bearer $GITHUB_PAT" \
  "https://api.github.com/repos/$OWNER_REPO/pulls?state=open&head=tereshed:$BR&per_page=1")
PR_NUM=$(echo "$PR_JSON" | jq -r '.[0].number // empty')
PR_URL=$(echo "$PR_JSON" | jq -r '.[0].html_url // empty')
[[ -n "$PR_NUM" ]] || fail "no PR opened for $BR"

# Опционально проверим, что в PR есть наш файл
PR_FILES=$(curl -sS -H "Authorization: Bearer $GITHUB_PAT" \
  "https://api.github.com/repos/$OWNER_REPO/pulls/$PR_NUM/files" | jq -r '.[].filename')
FNAME="$(basename "$BR").md"
echo "$PR_FILES" | grep -qx "$FNAME" || fail "PR #$PR_NUM does not include $FNAME (files: $PR_FILES)"

printf '\033[1;32m[smoke OK]\033[0m PR #%s opened: %s (files: %s)\n' "$PR_NUM" "$PR_URL" "$PR_FILES"
