-- +goose Up
-- +goose StatementBegin

-- scheduled_tasks — регулярные (повторяющиеся по cron-расписанию) задачи на уровне
-- проекта. Leader-gated раннер (scheduled-tasks) тикает раз в минуту, выбирает строки
-- где is_active AND next_run_at <= now() и создаёт обычную task'у в проекте/команде с
-- заданными name/description/priority, после чего пересчитывает next_run_at по cron.
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id         UUID         REFERENCES teams(id) ON DELETE SET NULL,
    created_by      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(500) NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    cron_expression VARCHAR(255) NOT NULL,
    priority        VARCHAR(50)  NOT NULL DEFAULT 'medium',
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_project_id ON scheduled_tasks(project_id);
-- Покрывает выборку «что пора запустить» в раннере.
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_due ON scheduled_tasks(next_run_at) WHERE is_active;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS scheduled_tasks;

-- +goose StatementEnd
