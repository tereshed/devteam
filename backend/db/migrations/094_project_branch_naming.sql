-- +goose Up
-- +goose StatementBegin

-- Per-project конвенция именования git-веток (см. internal/service/branch_template.go).
--
-- branch_name_template — free-form шаблон с плейсхолдерами ({ticket}, {slug}, {short_id},
--   {id}, {date}, ...). NULL/'' ⇒ дефолт task/{short_id}-{slug} (legacy-поведение).
--   Также служит источником «жёсткого формата»: из него выводится regex, валидирующий
--   ручные override'ы имени ветки.
-- branch_name_pattern — опциональный явный regex, перебивающий выведенный из шаблона
--   (escape-hatch для тонкого контроля формата). NULL ⇒ использовать выведенный.
-- branch_naming_locked — запрет ручного override имени ветки: ветка только генерируемая.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS branch_name_template TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS branch_name_pattern TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS branch_naming_locked BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS branch_naming_locked;
ALTER TABLE projects DROP COLUMN IF EXISTS branch_name_pattern;
ALTER TABLE projects DROP COLUMN IF EXISTS branch_name_template;

-- +goose StatementEnd
