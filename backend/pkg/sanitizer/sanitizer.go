package sanitizer

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[-_]?key|secret|password|token|auth|credential)(["']?\s*[:=]\s*["']?)([a-zA-Z0-9\-_.~]{4,})`),
		regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_.~]{15,})`),
	}
)

// MaskSecrets маскирует чувствительные данные в тексте.
func MaskSecrets(text string) string {
	if text == "" {
		return ""
	}

	// Пытаемся декодировать URL-encoded строку для поиска скрытых секретов
	decoded, err := url.PathUnescape(text)
	if err == nil {
		text = decoded
	}

	result := text
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			groups := pattern.FindStringSubmatch(match)
			if len(groups) >= 3 {
				if len(groups) >= 4 {
					// Группа 1: ключ, Группа 2: разделитель, Группа 3: значение
					return groups[1] + groups[2] + "********"
				}
				// Группа 1: префикс (например, bearer), Группа 2: значение
				return groups[1] + "********"
			}
			return "********"
		})
	}
	return strings.TrimSpace(result)
}
