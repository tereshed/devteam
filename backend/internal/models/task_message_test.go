package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSenderType_IsValid(t *testing.T) {
	assert.True(t, SenderTypeUser.IsValid())
	assert.True(t, SenderTypeAgent.IsValid())
	assert.False(t, SenderType("system").IsValid())
}

func TestMessageType_IsValid(t *testing.T) {
	for _, m := range []MessageType{
		MessageTypeInstruction, MessageTypeResult, MessageTypeQuestion,
		MessageTypeFeedback, MessageTypeError,
	} {
		assert.True(t, m.IsValid(), "expected valid: %s", m)
	}
	assert.False(t, MessageType("chat").IsValid())
}
