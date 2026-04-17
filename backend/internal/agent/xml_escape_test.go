package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeXMLText(t *testing.T) {
	assert.Equal(t, "&amp;&lt;evil&gt;", EscapeXMLText("&<evil>"))
}

func TestEmbedJSONForXML_escapesAngleBracketsInStrings(t *testing.T) {
	raw := json.RawMessage(`{"x":"</task_context>"}`)
	out := EmbedJSONForXML(raw)
	assert.Contains(t, out, `\u003c`)
	assert.NotContains(t, out, "</task_context>")
}
