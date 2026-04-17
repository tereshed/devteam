package agent

import (
	"bytes"
	"encoding/json"
	"strings"
)

// EscapeXMLText escapes &, <, > so embedded text cannot close pseudo-XML boundaries (task 6.2 / 6.8).
// Order: ampersands first.
func EscapeXMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// EmbedJSONForXML re-marshals JSON with HTML-safe escaping so angle brackets in string values
// cannot break out of <task_context> / <role_context> wrappers.
func EmbedJSONForXML(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return EscapeXMLText(string(raw))
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return EscapeXMLText(string(raw))
	}
	return strings.TrimSpace(buf.String())
}
