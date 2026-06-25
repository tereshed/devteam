-- +goose Up
-- +goose StatementBegin

-- «Переменные проекта» (project_secrets) до этого попадали в sandbox только через
-- ${secret:NAME} в конфигах MCP-серверов. Добавляем явный opt-in на инъекцию
-- значения как ОБЫЧНОЙ переменной окружения песочницы + человекочитаемое описание,
-- которое подсовывается агенту в промпт («доступные переменные»).
--
-- inject_as_env — если TRUE: значение кладётся в env агент-контейнера (Developer/Tester и т.п.)
--   и имя переменной попадает в промпт-блок «доступные переменные окружения».
--   Дефолт FALSE — обратная совместимость: существующие секреты ведут себя как раньше.
-- description — необязательная подсказка агенту (что это за переменная). Значение секрета
--   в промпт НЕ попадает никогда — только имя и описание.
ALTER TABLE project_secrets ADD COLUMN IF NOT EXISTS inject_as_env BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE project_secrets ADD COLUMN IF NOT EXISTS description VARCHAR(255) NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE project_secrets DROP COLUMN IF EXISTS description;
ALTER TABLE project_secrets DROP COLUMN IF EXISTS inject_as_env;

-- +goose StatementEnd
