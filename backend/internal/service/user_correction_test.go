package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAndSanitizeUserCorrection(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		s, err := ValidateAndSanitizeUserCorrection("  Fix API \n\t ")
		require.NoError(t, err)
		require.Equal(t, "Fix API", s)
	})
	t.Run("too large", func(t *testing.T) {
		_, err := ValidateAndSanitizeUserCorrection(strings.Repeat("a", UserCorrectionMaxBytes+1))
		require.ErrorIs(t, err, ErrUserCorrectionTooLarge)
	})
	t.Run("empty after trim", func(t *testing.T) {
		_, err := ValidateAndSanitizeUserCorrection("   \n\t  ")
		require.ErrorIs(t, err, ErrUserCorrectionEmpty)
	})
}

func TestRedactCorrectionForLog(t *testing.T) {
	s := strings.Repeat("x", 300)
	out := RedactCorrectionForLog(s)
	require.LessOrEqual(t, len(out), 210)
	require.True(t, strings.HasSuffix(out, "…"))
}
