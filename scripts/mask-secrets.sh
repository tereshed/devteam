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

# URL-encoding через python — без него секрет, попавший в URL (`?token=…`),
# не будет совпадать с raw-значением и утечёт в логи.
_mask_secrets_url_encode() {
  python3 -c 'import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

mask_secrets_addmask() {
  for name in "${SECRET_ENV_NAMES[@]}"; do
    local value="${!name:-}"
    if [ -n "$value" ] && [ "${#value}" -ge 16 ]; then
      echo "::add-mask::$value"
      local encoded
      encoded=$(_mask_secrets_url_encode "$value")
      if [ "$encoded" != "$value" ]; then
        echo "::add-mask::$encoded"
      fi
    fi
  done
}

mask_secrets_scrub_text() {
  local text="$1"
  for name in "${SECRET_ENV_NAMES[@]}"; do
    local value="${!name:-}"
    if [ -n "$value" ] && [ "${#value}" -ge 16 ]; then
      text="${text//$value/***}"
      local encoded
      encoded=$(_mask_secrets_url_encode "$value")
      if [ "$encoded" != "$value" ]; then
        text="${text//$encoded/***}"
      fi
    fi
  done
  printf '%s' "$text"
}

mask_secrets_scrub_stdin() {
  local input
  input=$(cat)
  mask_secrets_scrub_text "$input"
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
