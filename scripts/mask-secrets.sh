#!/usr/bin/env bash
# scripts/mask-secrets.sh
#
# Defense-in-depth для CI: маскирует значения известных секретов в произвольной
# строке (например, в дампе логов после `docker compose logs`).
#
# Используется в двух режимах:
#
# 1) `source scripts/mask-secrets.sh` → подгружает функции в текущий шелл:
#       - mask_secrets_addmask                  # ::add-mask:: для каждого env
#       - mask_secrets_scrub_stdin              # cat | mask_secrets_scrub_stdin
#       - mask_secrets_scrub_text "..."         # вернёт замаскированный текст
#
# 2) Прямой вызов: `./scripts/mask-secrets.sh [add-mask|scrub]`.
#       - add-mask  → печатает `::add-mask::$VALUE` для GitHub Actions.
#       - scrub     → читает stdin, отдаёт scrub'd-stdout.
#
# Этот же набор имён продублирован в `service.KnownSecretEnvNames` (Go).
# Если добавляешь переменную — добавь в оба места.
#
# КРИТИЧНО (review #1):
#  - Ранние версии использовали `${text//$value/***}` (bash parameter expansion).
#    Это интерпретирует `*`, `?`, `[…]` в $value как glob-паттерны: токен
#    `sk-*something*` маскировал бы всю строку, а не только сам секрет.
#  - `mask_secrets_scrub_stdin` ранее делал `input=$(cat)` — вычитывал весь
#    лог-файл (сотни МБ при `docker compose logs`) в одну bash-строку → OOM.
#
# Текущая реализация: scrub-логика вынесена в python-helper, который
# обрабатывает stdin **потоково** (line-by-line), и заменяет через `str.replace`
# (никакого regex/glob). bash остаётся только для add-mask (echo по одному
# значению — там OOM невозможен по построению).

# shellcheck disable=SC2034
SECRET_ENV_NAMES=(
  ANTHROPIC_API_KEY
  OPENAI_API_KEY
  OPENROUTER_API_KEY
  OPENROUTER_KEY
  DEEPSEEK_API_KEY
  GEMINI_API_KEY
  QWEN_API_KEY
  LLM_API_KEY
  GITHUB_PAT
  GITHUB_OAUTH_CLIENT_SECRET
  GITLAB_OAUTH_CLIENT_SECRET
  CLAUDE_CODE_OAUTH_ACCESS_TOKEN
  CLAUDE_CODE_OAUTH_REFRESH_TOKEN
  CLAUDE_CODE_OAUTH_CLIENT_SECRET
  ENCRYPTION_KEY
  JWT_SECRET_KEY
  DB_PASSWORD
  ADMIN_PASSWORD
)

# _mask_secrets_python_scrub — потоковый scrub stdin → stdout через python.
#
# Аргументы (опционально): имена env-переменных, которые надо маскировать.
# Если не переданы — берём всё из SECRET_ENV_NAMES.
#
# Алгоритм:
#  1) Считываем из env значения для каждого имени.
#  2) Для каждого значения готовим расширенный набор форм:
#       - raw value;
#       - urllib.parse.quote(val, safe="")    # %-encoding (пробел = `%20`);
#       - urllib.parse.quote_plus(val)        # query-encoding (пробел = `+`);
#         это совпадает с поведением Go `url.QueryEscape` (см. e2e_real_test.go
#         assertNoSecretInBackendLog), поэтому здесь обязательно).
#  3) Дедуплицируем и сортируем по длине убывания — иначе короткие подстроки
#     перетрут длинные.
#  4) Стримим stdin построчно, `str.replace` каждое значение на `***`.
#
# str.replace не интерпретирует pattern (никаких glob/regex) — это и есть фикс
# review #1.
_mask_secrets_python_scrub() {
  # КРИТИЧНО: НЕ используем `python3 - "$@" <<'PY'` — `python3 -` означает
  # «читать скрипт со stdin», но stdin УЖЕ занят stdin'ом всей функции
  # (потоком логов). Heredoc украл бы stdin и наша функция вернулась бы пустой.
  # Поэтому передаём скрипт через -c как обычный строковый аргумент.
  python3 -c '
import os
import sys
import urllib.parse

names = sys.argv[1:]
if not names:
    raise SystemExit("usage: _mask_secrets_python_scrub NAME [NAME ...]")

variants = set()
for name in names:
    val = os.environ.get(name)
    if not val or len(val) < 16:
        # <16 байт = много ложных срабатываний на легитимных подстроках
        # (например, `password=test`), не маскируем. Та же гарантия применяется
        # в Go-стороне (см. assertNoSecretInBackendLog).
        continue
    variants.add(val)
    enc_pct = urllib.parse.quote(val, safe="")  # пробел -> %20
    if enc_pct != val:
        variants.add(enc_pct)
    enc_plus = urllib.parse.quote_plus(val)     # пробел -> + (Go url.QueryEscape)
    if enc_plus != val:
        variants.add(enc_plus)

# Длинные перед короткими: чтобы маскировка `secret_full` не оставила хвост
# при наличии подстроки `secret_full_prefix` где-то.
ordered = sorted(variants, key=lambda s: (-len(s), s))

# Потоковый stdin -> stdout. Никаких .read() — log может быть в сотни МБ.
for line in sys.stdin:
    for s in ordered:
        if s in line:
            line = line.replace(s, "***")
    sys.stdout.write(line)
' "$@"
}

mask_secrets_addmask() {
  for name in "${SECRET_ENV_NAMES[@]}"; do
    local value="${!name:-}"
    if [ -n "$value" ] && [ "${#value}" -ge 16 ]; then
      echo "::add-mask::$value"
      # %-encoding (urllib.parse.quote, пробел = %20).
      local enc_pct
      enc_pct=$(python3 -c 'import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$value")
      if [ "$enc_pct" != "$value" ]; then
        echo "::add-mask::$enc_pct"
      fi
      # query-encoding (urllib.parse.quote_plus, пробел = `+`). Идентично
      # Go `url.QueryEscape` — без этого assertNoSecretInBackendLog мог бы
      # обнаружить leak, который actions::add-mask не закрыл бы.
      local enc_plus
      enc_plus=$(python3 -c 'import urllib.parse, sys; print(urllib.parse.quote_plus(sys.argv[1]))' "$value")
      if [ "$enc_plus" != "$value" ] && [ "$enc_plus" != "$enc_pct" ]; then
        echo "::add-mask::$enc_plus"
      fi
    fi
  done
}

# mask_secrets_scrub_text "..." — для коротких строк, не stdin. Использует тот
# же python-helper через here-string, чтобы алгоритм был ОДНОЙ кодовой строкой.
#
# КРИТИЧНО: предполагается, что text помещается в память (≤ десятки МБ).
# Для больших объёмов используй mask_secrets_scrub_stdin (он потоковый).
mask_secrets_scrub_text() {
  local text="$1"
  printf '%s' "$text" | _mask_secrets_python_scrub "${SECRET_ENV_NAMES[@]}"
}

# mask_secrets_scrub_stdin — потоковый scrub. Безопасен для гигабайтных логов.
mask_secrets_scrub_stdin() {
  _mask_secrets_python_scrub "${SECRET_ENV_NAMES[@]}"
}

# Если скрипт запущен напрямую (не через `source`), исполняем команду.
# BASH_SOURCE[0] == $0 — стабильный детектор.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  case "${1:-}" in
    add-mask)
      mask_secrets_addmask
      ;;
    scrub)
      mask_secrets_scrub_stdin
      ;;
    *)
      echo "Usage: $0 {add-mask|scrub}" >&2
      echo "  add-mask  emit ::add-mask::… directives for known SECRET env vars" >&2
      echo "  scrub     read stdin, write scrub'd version to stdout" >&2
      exit 2
      ;;
  esac
fi
