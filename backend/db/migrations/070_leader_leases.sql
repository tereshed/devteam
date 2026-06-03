-- +goose Up
-- +goose StatementBegin

-- Lease-based leader election для процессов-синглтонов при горизонтальном масштабировании
-- (cron-планировщик, токен-рефрешеры, retention, stale-recovery, workflow-worker).
-- YugabyteDB 2.20 не поддерживает pg advisory locks, поэтому держим строку-лиз и делаем
-- compare-and-swap по expires_at. Источник времени — БД (now()), чтобы исключить влияние
-- рассинхрона часов между инстансами.
CREATE TABLE IF NOT EXISTS leader_leases (
    name        VARCHAR(64)  PRIMARY KEY,
    holder      VARCHAR(128) NOT NULL,
    acquired_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ  NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS leader_leases;

-- +goose StatementEnd
