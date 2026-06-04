#!/usr/bin/env bash
# backup.sh — дамп YugabyteDB (YSQL) → gzip → S3-совместимое Object Storage.
#
# «Вариант 0» = всё на одной VM, поэтому единственная защита данных — регулярный дамп НА
# ВНЕШНЕЕ хранилище. Volume на диске VM сам по себе бэкапом НЕ является: смерть/пересоздание
# инстанса = потеря всего.
#
# Зависимости на хосте: docker, aws-cli (v2).
# Конфиг через окружение (положи в /etc/polymaths-backup.env, root-only chmod 600):
#   YB_CONTAINER   — имя контейнера БД           (default: wibe_yugabytedb)
#   DB_NAME        — имя базы                     (default: yugabyte)
#   DB_USER        — пользователь                 (default: yugabyte)
#   S3_BUCKET      — бакет назначения             (например s3://polymaths-backups)
#   S3_ENDPOINT    — endpoint Object Storage      (Yandex: https://storage.yandexcloud.net)
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY     — статический ключ сервис-аккаунта
#   RETENTION_DAYS — сколько хранить локально     (default: 7)
#   BACKUP_DIR     — локальная папка дампов        (default: /var/backups/polymaths)
#
# Cron (ежедневно в 03:30, конфиг из env-файла):
#   30 3 * * *  set -a; . /etc/polymaths-backup.env; set +a; /opt/devteam/deployment/prod/backup.sh >> /var/log/polymaths-backup.log 2>&1
set -euo pipefail

YB_CONTAINER="${YB_CONTAINER:-wibe_yugabytedb}"
DB_NAME="${DB_NAME:-yugabyte}"
DB_USER="${DB_USER:-yugabyte}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/polymaths}"

: "${S3_BUCKET:?S3_BUCKET is required (e.g. s3://polymaths-backups)}"
: "${S3_ENDPOINT:?S3_ENDPOINT is required (Yandex: https://storage.yandexcloud.net)}"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
file="${BACKUP_DIR}/polymaths-${DB_NAME}-${ts}.sql.gz"
mkdir -p "${BACKUP_DIR}"

# ysql_dump лежит в образе yugabyte (обычно в postgres/bin). Находим его на всякий случай,
# чтобы не зависеть от точного пути между версиями.
dump_bin="$(docker exec "${YB_CONTAINER}" sh -c \
  'command -v ysql_dump || ls /home/yugabyte/postgres/bin/ysql_dump 2>/dev/null || ls /home/yugabyte/bin/ysql_dump 2>/dev/null' \
  | head -n1)"
if [ -z "${dump_bin}" ]; then
  echo "[backup] FATAL: ysql_dump не найден в контейнере ${YB_CONTAINER}" >&2
  exit 1
fi

echo "[backup] $(date -u) dumping ${DB_NAME} via ${dump_bin}"
# Стримим дамп из контейнера и жмём на хосте — не упираемся в диск контейнера.
docker exec "${YB_CONTAINER}" "${dump_bin}" -h 127.0.0.1 -U "${DB_USER}" "${DB_NAME}" \
  | gzip -9 > "${file}"

size="$(du -h "${file}" | cut -f1)"
echo "[backup] wrote ${file} (${size})"

echo "[backup] uploading to ${S3_BUCKET}/"
aws --endpoint-url "${S3_ENDPOINT}" s3 cp "${file}" "${S3_BUCKET}/$(basename "${file}")"

# Локальная ретенция (на удалённой стороне настрой lifecycle-правило бакета).
find "${BACKUP_DIR}" -name 'polymaths-*.sql.gz' -mtime "+${RETENTION_DAYS}" -delete
echo "[backup] done; local retention ${RETENTION_DAYS}d"
