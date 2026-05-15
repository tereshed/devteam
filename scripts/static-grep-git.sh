#!/usr/bin/env bash
#
# Static-grep: enforce that every exec.Command(Context)?("git", ...) call site
# in non-test production code passes a "--" separator between flags and
# user-controlled args. Mitigates git flag-injection on baseBranch/path inputs.
#
# Multiline-aware: reads a small window (the matched line + the next 5 lines)
# because constructed git argv lists often span line boundaries, e.g.
#
#   cmd := exec.CommandContext(ctx, "git", "-C", root,
#       "worktree", "add", path, "-b", branch, "--", baseBranch)
#
# Calls that delegate the argv slice via `args...` are intentionally exempt;
# the responsibility for the "--" separator lies with the helper that builds
# args (covered by unit tests, e.g. TestBuildGitWorktreeRemoveArgs).
#
# Exits 0 on clean, 1 on violation.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

violations=0

while IFS=: read -r file lineno _rest; do
  [[ -z "$file" ]] && continue
  block=$(sed -n "${lineno},$((lineno + 5))p" "$file" 2>/dev/null || true)

  # Exempt: argv built externally as a slice and spread via `name...`
  if grep -qE '[A-Za-z_][A-Za-z0-9_]*\.\.\.' <<<"$block"; then
    continue
  fi

  if ! grep -q '"--"' <<<"$block"; then
    echo "VIOLATION: ${file}:${lineno} — exec.Command(\"git\", ...) without \"--\" separator"
    echo "----"
    echo "$block"
    echo "----"
    violations=$((violations + 1))
  fi
done < <(
  grep -rEn \
    --include='*.go' \
    --exclude='*_test.go' \
    'exec\.Command(Context)?\([^)]*"git"' \
    backend/internal backend/pkg \
  || true
)

if (( violations > 0 )); then
  echo ""
  echo "static-grep-git: $violations violation(s) found."
  echo "All inline git exec.Command calls must include a \"--\" separator before"
  echo "user-controlled paths/refs. See backend/internal/service/worktree_manager.go"
  echo "for the canonical pattern."
  exit 1
fi

echo "static-grep-git: OK (no inline git exec.Command call is missing -- separator)"
