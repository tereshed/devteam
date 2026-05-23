-- +goose Up
-- +goose NO TRANSACTION
CREATE TABLE IF NOT EXISTS team_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT null UNIQUE,
    name VARCHAR(255) NOT null,
    is_system BOOLEAN NOT null DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

-- Seed with initial team types
INSERT INTO team_types (code, name, is_system) VALUES 
('development', 'Development', true),
('research', 'Research', false),
('analytics', 'Analytics', false)
ON CONFLICT (code) DO NOTHING;

-- Seed any other existing team types in teams table to prevent foreign key violations
INSERT INTO team_types (code, name, is_system)
SELECT DISTINCT type, type, false
FROM teams
WHERE type IS NOT NULL AND type != ''
ON CONFLICT (code) DO NOTHING;

-- Drop constraint chk_teams_type from teams table
ALTER TABLE teams DROP CONSTRAINT IF EXISTS chk_teams_type;

-- Drop foreign key constraint if it already exists to allow re-runs
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_type;

-- Add foreign key constraint to teams
ALTER TABLE teams ADD CONSTRAINT fk_teams_type FOREIGN KEY (type) REFERENCES team_types(code) ON UPDATE CASCADE;

-- +goose Down
-- +goose NO TRANSACTION
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_type;
ALTER TABLE teams ADD CONSTRAINT chk_teams_type CHECK (type IN ('development', 'research', 'analytics'));
DROP TABLE IF EXISTS team_types;
