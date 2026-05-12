#!/bin/sh
# Integration test (14.5 cancel): эмулирует «долго работающий» Claude.
# Тест зовёт DockerSandboxRunner.StopTask и ожидает, что контейнер будет
# принудительно остановлен и удалён.
exec sleep 3600
