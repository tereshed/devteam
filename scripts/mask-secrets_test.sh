#!/usr/bin/env bash
# scripts/mask-secrets_test.sh — sanity-тесты для mask-secrets.sh.
#
# Проверяет регрессы из review Phase 5 #1:
#   1) Bash-globs в значении секрета НЕ должны интерпретироваться как pattern
#      (символы `*`, `?`, `[`). До рефакторинга `${text//$value/***}` это
#      делал и маскировал что попало.
#   2) Stdin > 100MB должен скрабиться без OOM (потоковый python-helper).
#   3) url-encoding покрывает оба варианта: %-encoded (quote) и + (quote_plus,
#      совпадает с Go url.QueryEscape).
#
# Запуск:
#   bash scripts/mask-secrets_test.sh
#
# Exit 0 — все ок; ≠0 — какой-то тест упал.

set -euo pipefail

cd "$(dirname "$0")/.."
SCRIPT="scripts/mask-secrets.sh"

failed=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n  %s\n' "$1" "$2" >&2; failed=$((failed + 1)); }

# ── Test 1: glob-pattern в секрете не должен матчить лишнего ────────────────
#
# До фикса `${text//$value/***}` развернул `*` в значении как glob и в строке
# `aaaaaaaaaaaaaaaaaaa bbbbbbbbbbbbbbbbbbbbbb` маскировал бы любое слово.
#
# Используем заведомо длинный (>16 байт) секрет с `*` внутри. После фикса
# python str.replace должен заменить ровно подстроку.
test_glob_safety() {
  local secret='aaaaaa*bbbbbbcccccc'  # 19 байт, содержит `*`
  # Если bash-version интерпретирует `*` как glob, заменится «всё что между
  # `aaaaaa` и `bbbbbbcccccc`» — то есть в `aaaaaaXXXXXbbbbbbcccccc` всё
  # сожрётся целиком. Python str.replace такого не сделает.
  local input='before aaaaaaXXXXXbbbbbbcccccc after | aaaaaa*bbbbbbcccccc'
  local expected='before aaaaaaXXXXXbbbbbbcccccc after | ***'
  local got
  got=$(GLOB_SECRET="$secret" bash -c "
    # shellcheck source=/dev/null
    source $SCRIPT
    # Подменяем SECRET_ENV_NAMES — нам нужно проверить ровно этот ключ.
    SECRET_ENV_NAMES=(GLOB_SECRET)
    printf '%s' '$input' | mask_secrets_scrub_stdin
  ")
  if [ "$got" = "$expected" ]; then
    pass "glob-safety: '*' в секрете не раскрывается"
  else
    fail "glob-safety" "expected=$expected; got=$got"
  fi
}

# ── Test 2: stdin > 100 MB не вызывает OOM ───────────────────────────────────
#
# `$(cat)` загрузил бы все 100 МБ в bash-строку. Python через `for line in
# stdin` стримит по строке — пик RSS должен оставаться в десятках МБ.
#
# 100 МБ строк по 80 байт = ~1.3M строк. Замер: с прошлой реализацией bash
# вылетал по сегфолту/OOM на ~50 МБ.
test_stdin_streaming() {
  # КРИТИЧНО: тест проверяет ровно «100 МБ stdin не вызывает OOM и завершается».
  # Размер вывода НЕ равен 100 МБ если секрет встречается в каждой строке —
  # python str.replace заменит его на «***» (короче, выход усыхает). Поэтому
  # используем секрет, которого в потоке НЕ ВСТРЕЧАЕТСЯ, — тогда output == input.
  local absent_secret='ZZZZZZZZZZZZZZZZ-not-in-stream-ZZZZZZ'  # 37 байт, ≥16
  local out_file
  out_file=$(mktemp)
  # На macOS coreutils-`timeout` отсутствует по умолчанию (есть только gtimeout).
  # Эмулируем через `&`+kill-after-deadline в фоне, чтобы тест работал и на CI
  # ubuntu-runner'е, и локально на mac без brew install coreutils.
  SECRET_TEST="$absent_secret" bash -c "
    # shellcheck source=/dev/null
    source $SCRIPT
    SECRET_ENV_NAMES=(SECRET_TEST)
    # 100 МБ потока (yes | head -c). Скраббинг через потоковый python -
    # пик RSS должен быть в десятках МБ, а не растущим линейно от объёма входа.
    yes 'random line of log content here' | head -c 104857600 \
      | mask_secrets_scrub_stdin \
      | wc -c
  " > "$out_file" &
  local pid=$!
  # 90с deadline; если зависло — kill и fail.
  ( sleep 90 && kill -9 "$pid" 2>/dev/null ) &
  local guard=$!
  wait "$pid" 2>/dev/null
  kill -9 "$guard" 2>/dev/null || true
  wait "$guard" 2>/dev/null || true
  local got
  got=$(tr -d '[:space:]' < "$out_file")
  rm -f "$out_file"
  # Секрет НЕ во входе → выход == вход (по байту в байт). 100 МБ ± округление.
  if [ -n "$got" ] && [ "$got" -ge 99000000 ] && [ "$got" -le 110000000 ]; then
    pass "stdin streaming: 100MB обработан без OOM (out=$got bytes)"
  else
    fail "stdin streaming" "expected ~100MB out, got=$got"
  fi
}

# ── Test 3: оба url-encoding варианта (quote и quote_plus) ──────────────────
#
# Секрет с пробелом → %-encoding `with%20space` и +-encoding `with+space`.
# Оба должны маскироваться, иначе расхождение с Go url.QueryEscape (см.
# e2e_real_test.go) даст ложный pass либо ложный leak.
test_url_encoding_both() {
  # 17 байт ≥16 — пройдёт length-gate.
  local secret='hello world 12345'
  local input='raw=hello world 12345 pct=hello%20world%2012345 plus=hello+world+12345'
  local expected='raw=*** pct=*** plus=***'
  local got
  got=$(URL_SECRET="$secret" bash -c "
    # shellcheck source=/dev/null
    source $SCRIPT
    SECRET_ENV_NAMES=(URL_SECRET)
    printf '%s' '$input' | mask_secrets_scrub_stdin
  ")
  if [ "$got" = "$expected" ]; then
    pass "url-encoding: маскируется raw + %-encoded + +-encoded"
  else
    fail "url-encoding" "expected=$expected; got=$got"
  fi
}

# ── Test 4: короткие секреты (<16 байт) НЕ маскируются ─────────────────────
#
# Это явное design-решение в скрипте (защита от ложных срабатываний на
# легитимных подстроках вроде `password=test`). Тест-канарейка чтобы кто-то
# случайно не уронил гейт.
test_short_secret_skipped() {
  local secret='short'  # 5 байт
  local input='leak=short something'
  local got
  got=$(SHORT_SECRET="$secret" bash -c "
    # shellcheck source=/dev/null
    source $SCRIPT
    SECRET_ENV_NAMES=(SHORT_SECRET)
    printf '%s' '$input' | mask_secrets_scrub_stdin
  ")
  if [ "$got" = "$input" ]; then
    pass "short secret skipped: <16 байт не маскируется"
  else
    fail "short secret skipped" "expected unchanged, got=$got"
  fi
}

test_glob_safety
test_stdin_streaming
test_url_encoding_both
test_short_secret_skipped

if [ "$failed" -gt 0 ]; then
  printf '\n%d test(s) failed\n' "$failed" >&2
  exit 1
fi
echo
echo "all good"
