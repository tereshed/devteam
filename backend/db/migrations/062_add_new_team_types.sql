-- +goose Up
-- +goose StatementBegin
INSERT INTO team_types (code, name, is_system) VALUES
('marketing', 'Marketing', false),
('smm', 'SMM', false),
('rd', 'R&D', false),
('hr', 'HR', false),
('legal', 'Legal', false),
('other', 'Other', false)
ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM team_types WHERE code IN ('marketing', 'smm', 'rd', 'hr', 'legal', 'other');
-- +goose StatementEnd
