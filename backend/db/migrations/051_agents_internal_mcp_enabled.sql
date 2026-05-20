-- +goose Up
ALTER TABLE agents ADD COLUMN internal_mcp_enabled BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE agents DROP COLUMN IF EXISTS internal_mcp_enabled;
