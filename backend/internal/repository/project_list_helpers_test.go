package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeILIKEWildcards(t *testing.T) {
	t.Parallel()
	assert.Equal(t, `a\%b\_c\\`, escapeILIKEWildcards(`a%b_c\`))
	assert.Equal(t, `100\%\_done\\`, escapeILIKEWildcards(`100%_done\`))
}

func TestNormalizeProjectListLimit(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 20, normalizeProjectListLimit(0))
	assert.Equal(t, 20, normalizeProjectListLimit(-1))
	assert.Equal(t, 5, normalizeProjectListLimit(5))
	assert.Equal(t, 20, normalizeProjectListLimit(20))
	assert.Equal(t, 100, normalizeProjectListLimit(100))
	assert.Equal(t, 100, normalizeProjectListLimit(101))
	assert.Equal(t, 100, normalizeProjectListLimit(500))
}
