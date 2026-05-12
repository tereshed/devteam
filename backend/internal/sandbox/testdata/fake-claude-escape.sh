#!/bin/sh
# Integration test (14.4 sandbox isolation): пытается прикоснуться к файлам
# вне /workspace. Sandbox-пользователь uid=1001 без sudo не должен иметь
# доступ к записи в /etc/, /usr/, /, и т.п. Скрипт всегда возвращает 0:
# тест проверяет, что write действительно НЕ произошёл, через docker exec stat.
exec 1>/tmp/escape.log 2>&1
set +e
echo "compromised" > /etc/passwd
echo "compromised" > /etc/devteam_pwned
echo "compromised" > /usr/local/bin/escape
echo "compromised" > /escape_at_root
echo "exit codes: $?"
exit 0
