package sandbox

import (
	"fmt"
	"log/slog"
)

var _ slog.LogValuer = (*CodeResult)(nil)

const (
	codeResultLogMaxPaths     = 5
	codeResultLogMaxPathRunes = 64
)

// LogValue реализует slog.LogValuer: при логировании структуры с полем CodeResult
// Diff и Output не утекают целиком (секреты, .env). См. задачу 5.7, комментарий к CodeResult в types.go.
//
// fmt.Printf("%+v", cr) и %#v по-прежнему обходят LogValue — для отладки не используйте их на сырых данных.
func (cr *CodeResult) LogValue() slog.Value {
	if cr == nil {
		return slog.GroupValue(slog.String("code_result", "<nil>"))
	}
	attrs := []slog.Attr{
		slog.Bool("success", cr.Success),
		slog.Int("tokens_used", cr.TokensUsed),
		slog.String("duration", cr.Duration.String()),
		slog.String("branch_name", cr.BranchName),
		slog.String("commit_hash", cr.CommitHash),
		slog.Int("files_changed_count", len(cr.FilesChanged)),
		slog.String("diff", redactArtifactForLog(cr.Diff)),
		slog.String("output", redactArtifactForLog(cr.Output)),
	}
	if n := len(cr.FilesChanged); n > 0 {
		attrs = append(attrs, slog.String("files_changed_sample", summarizePathsForLog(cr.FilesChanged)))
	}
	return slog.GroupValue(attrs...)
}

func redactArtifactForLog(s string) string {
	if s == "" {
		return ""
	}
	// Только длина — без префикса текста, чтобы короткие секреты не попали в ELK даже частично.
	return fmt.Sprintf("<redacted len=%d>", len(s))
}

func summarizePathsForLog(paths []string) string {
	var b []byte
	n := len(paths)
	if n > codeResultLogMaxPaths {
		n = codeResultLogMaxPaths
	}
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		p := paths[i]
		r := []rune(p)
		if len(r) > codeResultLogMaxPathRunes {
			p = string(r[:codeResultLogMaxPathRunes]) + "…"
		}
		b = append(b, p...)
	}
	if len(paths) > codeResultLogMaxPaths {
		b = append(b, fmt.Sprintf(" …(+%d)", len(paths)-codeResultLogMaxPaths)...)
	}
	return string(b)
}
