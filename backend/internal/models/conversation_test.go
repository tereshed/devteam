package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConversationStatus_IsValid(t *testing.T) {
	for _, s := range []ConversationStatus{
		ConversationStatusActive,
		ConversationStatusCompleted,
		ConversationStatusArchived,
	} {
		assert.True(t, s.IsValid(), "expected valid: %s", s)
	}
	assert.False(t, ConversationStatus("draft").IsValid())
}

func TestConversationRole_IsValid(t *testing.T) {
	for _, r := range []ConversationRole{
		ConversationRoleUser,
		ConversationRoleAssistant,
		ConversationRoleSystem,
	} {
		assert.True(t, r.IsValid(), "expected valid: %s", r)
	}
	assert.False(t, ConversationRole("tool").IsValid())
}
