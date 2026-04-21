package repository

import (
	"strings"
)

// escapeILIKEWildcards экранирует \, % и _ для ILIKE с ESCAPE '\', чтобы ввод не работал как шаблон.
func escapeILIKEWildcards(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func sanitizeOrderDir(dir string) string {
	if strings.ToUpper(dir) == "ASC" {
		return "ASC"
	}
	return "DESC"
}

func normalizeLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
