-- +goose Up
-- +goose StatementBegin

-- enhancer_configs — per-project конфиг агента-улучшайзера (enhancer). Энхансер
-- анализирует историю выполнения задач проекта (router_decisions, artifacts,
-- task_events, фидбек пользователя) и формирует предложения изменений
-- (enhancer_changes): правки промптов агентов проекта, описания проекта и т.п.
-- Фаза 1: только propose-режим — предложения копятся и ждут решения человека.
-- Leader-gated раннер (enhancer-runs) тикает раз в минуту, выбирает строки где
-- is_active AND next_run_at <= now() и запускает прогон, после чего пересчитывает
-- next_run_at по cron. Ручной запуск — POST /projects/:id/enhancer/run.
CREATE TABLE IF NOT EXISTS enhancer_configs (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID         NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    -- created_by — владелец конфига; от его имени резолвится enhancer-агент и
    -- LLM-ключи (user_llm_credentials), как у scheduled_tasks.created_by.
    created_by           UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active            BOOLEAN      NOT NULL DEFAULT false,
    -- autonomy: propose — только предложения (дефолт); auto_apply — зарезервировано
    -- под фазу 3 (автоприменение с замером эффекта), в фазе 1 не принимается API.
    autonomy             VARCHAR(16)  NOT NULL DEFAULT 'propose',
    -- cron_expression — расписание автозапуска (5-польный cron); NULL/пусто —
    -- только ручной запуск.
    cron_expression      VARCHAR(255),
    -- analysis_window_days — окно истории задач, которое анализирует прогон.
    analysis_window_days INTEGER      NOT NULL DEFAULT 7,
    -- max_changes_per_run — гардрейл: жёсткий лимит предложений за один прогон,
    -- enforced в Go (инструмент enhancer_propose_change), не в промпте.
    max_changes_per_run  INTEGER      NOT NULL DEFAULT 5,
    last_run_at          TIMESTAMPTZ,
    next_run_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_enhancer_autonomy CHECK (autonomy IN ('propose', 'auto_apply')),
    CONSTRAINT chk_enhancer_window CHECK (analysis_window_days BETWEEN 1 AND 90),
    CONSTRAINT chk_enhancer_max_changes CHECK (max_changes_per_run BETWEEN 1 AND 20)
);

-- Покрывает выборку «что пора запустить» в раннере.
CREATE INDEX IF NOT EXISTS idx_enhancer_configs_due ON enhancer_configs(next_run_at) WHERE is_active;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS enhancer_configs;

-- +goose StatementEnd
