-- +goose Up
-- +goose StatementBegin

-- agent_skills — связь Skills (Claude Code skills) с агентом (Sprint 15.5).
-- skill_source описывает, откуда подгружается skill:
--   builtin — встроенный (входит в дистрибутив Claude Code)
--   plugin  — поставляется плагином (плагин указан в config_json.plugin)
--   path    — локальный путь (path указан в config_json.path)
CREATE TABLE agent_skills (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id     UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_name   VARCHAR(255) NOT NULL,
    skill_source VARCHAR(16)  NOT NULL,
    config_json  JSONB        NOT NULL DEFAULT '{}',
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_skills_source CHECK (skill_source IN ('builtin', 'plugin', 'path')),
    CONSTRAINT uq_agent_skills_agent_name UNIQUE (agent_id, skill_name)
);

CREATE INDEX idx_agent_skills_agent_id  ON agent_skills(agent_id);
CREATE INDEX idx_agent_skills_is_active ON agent_skills(is_active) WHERE is_active = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_agent_skills_is_active;
DROP INDEX IF EXISTS idx_agent_skills_agent_id;
DROP TABLE IF EXISTS agent_skills;

-- +goose StatementEnd
