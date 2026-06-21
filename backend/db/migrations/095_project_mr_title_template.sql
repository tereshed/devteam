-- +goose Up
-- +goose StatementBegin

-- mr_title_template — per-project шаблон тайтла MR/PR (см. branch_template.go:
-- RenderMRTitle). Плейсхолдеры {title},{ticket},{slug},{branch},{repo},{short_id},
-- {id},{date}. NULL/'' ⇒ дефолт «PolyMaths: {title}» (legacy-поведение публикатора PR).
ALTER TABLE projects ADD COLUMN IF NOT EXISTS mr_title_template TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS mr_title_template;

-- +goose StatementEnd
