package sandbox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamLineWriter_Write_manyShortLinesLinearScan(t *testing.T) {
	ch := make(chan LogEntry, 10000)
	w := newStreamLineWriter(ch, "id", false)

	var b strings.Builder
	for i := 0; i < 5000; i++ {
		b.WriteByte('a')
		b.WriteByte('\n')
	}
	payload := b.String()
	n, err := w.Write([]byte(payload))
	require.NoError(t, err)
	assert.Equal(t, len(payload), n)
	w.flush()
	close(ch)

	var lines int
	for e := range ch {
		if e.Line != "" {
			lines++
		}
	}
	assert.Equal(t, 5000, lines)
}
