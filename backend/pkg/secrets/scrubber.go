package secrets

import (
	"regexp"
)

// Scrubber — интерфейс для маскирования секретов.
type Scrubber interface {
	Scrub(s string) string
}

type defaultScrubber struct {
	regexps []*regexp.Regexp
}

const Redacted = "***REDACTED***"

var defaultPatterns = []string{
	`sk-[a-zA-Z0-9]{20,}`,             // OpenAI
	`ghp_[a-zA-Z0-9]{36,}`,            // GitHub
	`xoxb-[a-zA-Z0-9-]{10,}`,          // Slack
	`eyJ[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+`, // JWT
	`(?i)Authorization:\s*Basic\s*[a-zA-Z0-9+/=]+`,      // Basic Auth
	`AKIA[0-9A-Z]{16}`,                // AWS Access Key
	`-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]+?-----END [A-Z ]+ PRIVATE KEY-----`, // PEM Private Key
}

func NewScrubber() Scrubber {
	s := &defaultScrubber{
		regexps: make([]*regexp.Regexp, 0, len(defaultPatterns)),
	}
	for _, p := range defaultPatterns {
		s.regexps = append(s.regexps, regexp.MustCompile(p))
	}
	return s
}

func (s *defaultScrubber) Scrub(input string) string {
	if input == "" {
		return ""
	}
	res := input
	for _, r := range s.regexps {
		res = r.ReplaceAllString(res, Redacted)
	}
	return res
}

// GlobalScrubber — синглтон для простого использования.
var GlobalScrubber = NewScrubber()

// Scrub — обертка над GlobalScrubber.Scrub.
func Scrub(s string) string {
	return GlobalScrubber.Scrub(s)
}
