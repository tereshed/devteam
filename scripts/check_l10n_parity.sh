#!/usr/bin/env bash
# Паритет ключей app_en.arb / app_ru.arb; для строк с {…} — наличие @*.placeholders в обе стороны и совпадение
# имён и полей type в placeholders между ru и en (построчно: name=type, сортировка по имени).
# Запуск из корня репозитория: ./scripts/check_l10n_parity.sh  или  make frontend-l10n-check
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
L10N="$ROOT/frontend/lib/l10n"

if ! command -v jq >/dev/null 2>&1; then
  echo "check_l10n_parity: требуется jq в PATH" >&2
  exit 1
fi

echo "check_l10n_parity: сравнение множеств ключей сообщений…"
if ! diff -q \
  <(jq -r 'keys[] | select(test("^@") | not)' "$L10N/app_en.arb" | sort) \
  <(jq -r 'keys[] | select(test("^@") | not)' "$L10N/app_ru.arb" | sort) \
  >/dev/null; then
  echo "ARB: множества ключей в app_en.arb и app_ru.arb различаются:" >&2
  diff \
    <(jq -r 'keys[] | select(test("^@") | not)' "$L10N/app_en.arb" | sort) \
    <(jq -r 'keys[] | select(test("^@") | not)' "$L10N/app_ru.arb" | sort) \
    >&2 || true
  exit 1
fi

cd "$L10N"
bad=0

echo "check_l10n_parity: плейсхолдеры ru → en…"
while IFS= read -r k; do
  if ! jq -e --arg key "$k" '.["@" + $key].placeholders' app_en.arb >/dev/null 2>&1; then
    echo "missing @${k}.placeholders in app_en.arb (ключ есть в app_ru.arb со строкой с {…})" >&2
    bad=1
  fi
done < <(jq -r '
  to_entries[]
  | select(.key | test("^@") | not)
  | select(.value | type == "string")
  | select(.value | test("\\{[a-zA-Z_]"))
  | .key
' app_ru.arb)

echo "check_l10n_parity: плейсхолдеры en → ru…"
while IFS= read -r k; do
  if ! jq -e --arg key "$k" '.["@" + $key].placeholders' app_ru.arb >/dev/null 2>&1; then
    echo "missing @${k}.placeholders in app_ru.arb (ключ есть в app_en.arb со строкой с {…})" >&2
    bad=1
  fi
done < <(jq -r '
  to_entries[]
  | select(.key | test("^@") | not)
  | select(.value | type == "string")
  | select(.value | test("\\{[a-zA-Z_]"))
  | .key
' app_en.arb)

echo "check_l10n_parity: имена и типы placeholders (ru ↔ en)…"
while IFS= read -r k; do
  tmpa="$(mktemp)"
  tmpb="$(mktemp)"
  jq -r --arg k "$k" '
    (.["@" + $k].placeholders // {}) | to_entries | sort_by(.key) | .[] | "\(.key)=\(.value.type // "")"
  ' app_ru.arb >"$tmpa"
  jq -r --arg k "$k" '
    (.["@" + $k].placeholders // {}) | to_entries | sort_by(.key) | .[] | "\(.key)=\(.value.type // "")"
  ' app_en.arb >"$tmpb"
  if ! cmp -s "$tmpa" "$tmpb"; then
    echo "check_l10n_parity: расхождение placeholders для «${k}» (ru vs en):" >&2
    diff -u "$tmpa" "$tmpb" >&2 || true
    bad=1
  fi
  rm -f "$tmpa" "$tmpb"
done < <(
  {
    jq -r '
  to_entries[]
  | select(.key | test("^@") | not)
  | select(.value | type == "string")
  | select(.value | test("\\{[a-zA-Z_]"))
  | .key
' app_ru.arb
    jq -r '
  to_entries[]
  | select(.key | test("^@") | not)
  | select(.value | type == "string")
  | select(.value | test("\\{[a-zA-Z_]"))
  | .key
' app_en.arb
  } | sort -u
)

exit "$bad"
