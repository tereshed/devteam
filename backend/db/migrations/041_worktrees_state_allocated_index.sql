-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / 6.2 — глобальный GET /worktrees (admin debug screen).
--
-- Запрос: `[WHERE task_id = ? | WHERE state = ?] ORDER BY allocated_at DESC LIMIT 200`.
-- Дефолт UI (worktrees_list_screen, фильтр "All") — БЕЗ state-предиката,
-- только ORDER BY + LIMIT. Поэтому композит (state, allocated_at) НЕ годится:
-- его leading-column `state` отсутствует в WHERE → планировщик не сможет
-- использовать индекс для сортировки и упадёт в Seq Scan + Top-N heapsort
-- по всей таблице. Это ровно та катастрофа, от которой защищаемся.
--
-- Решение: одноколоночный индекс на (allocated_at DESC). Для дефолтного
-- запроса PG идёт по индексу в обратном порядке и останавливается после
-- 200 совпадений (top-N short-circuit через index scan). Для запросов с
-- state-фильтром — планировщик прогоняет index scan по allocated_at DESC и
-- отбрасывает не подходящие строки на лету; с LIMIT 200 это всё ещё дешевле
-- чем Full Table Scan + sort, потому что worktrees с одинаковым state в
-- среднем сосредоточены равномерно по времени (см. EXPLAIN ANALYZE в PR).
--
-- idx_worktrees_state (036) остаётся — используется для count'ов в админке
-- и для запросов БЕЗ сортировки.

CREATE INDEX IF NOT EXISTS idx_worktrees_allocated_at
    ON worktrees(allocated_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_worktrees_allocated_at;

-- +goose StatementEnd
