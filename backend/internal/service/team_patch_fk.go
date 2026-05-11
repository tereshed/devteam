package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Имя ограничения FK по умолчанию для REFERENCES tool_definitions(id) на колонке
// agent_tool_bindings.tool_definition_id (см. backend/db/migrations/016_alter_agents.sql).
// 13.3.1 A.8: нарушение этого FK → 400 (ErrTeamAgentInvalidToolBindings), не 409.
const fkAgentToolBindingsToolDefinitionID = "agent_tool_bindings_tool_definition_id_fkey"

// FK prompt_id → prompts (миграции agents / 016): единственный известный случай 409 для PatchAgent.
const fkAgentsPromptID = "agents_prompt_id_fkey"

// mapAgentPatchPostgresFK мапит известные 23503 после Save/SaveAgentWithToolBindings в доменные ошибки PatchAgent.
// Неизвестный ConstraintName не маскируется под 409 — пусть уходит в 500 для диагностики (см. ревью: mcp_bindings и др.).
// Возвращает (mapped, true) только для allowlist; иначе (nil, false).
func mapAgentPatchPostgresFK(err error) (error, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return nil, false
	}
	switch pgErr.ConstraintName {
	case fkAgentToolBindingsToolDefinitionID:
		return ErrTeamAgentInvalidToolBindings, true
	case fkAgentsPromptID:
		return ErrTeamAgentConflict, true
	default:
		return nil, false
	}
}
