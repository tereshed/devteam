-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / 6.10 — добавляем 'paused' в state-machine задач.
--
-- Cancel уже мигрировал на v2 (миграция 037 + TaskLifecycleService). Pause/Resume
-- оставались legacy. Теперь Pause = task.state='paused' (отдельное состояние,
-- не needs_human, чтобы UI мог различать «нужен оператор» и «пользователь
-- сам остановил»). Resume возвращает в active. Воркеры при pickup проверяют
-- state=='active' и тихо пропускают шаг, если задача на паузе.
--
-- Переходы (см. allowedTransitions в task_service.go):
--   active  → paused | done | failed | cancelled | needs_human
--   paused  → active | cancelled
--
-- cancel_requested при паузе НЕ выставляем (это самостоятельный сентинель отмены),
-- pause использует только тот факт что state!='active'.

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_state;
ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_state
        CHECK (state IN ('active', 'done', 'failed', 'cancelled', 'needs_human', 'paused'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Перед откатом нужно убрать все paused-строки — иначе CHECK прежней версии не сядет.
-- Маппим обратно в needs_human (наиболее близкая legacy-семантика «требуется внимание»).
UPDATE tasks SET state = 'needs_human' WHERE state = 'paused';

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_state;
ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_state
        CHECK (state IN ('active', 'done', 'failed', 'cancelled', 'needs_human'));

-- +goose StatementEnd
