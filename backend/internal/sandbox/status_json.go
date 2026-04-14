package sandbox

import (
	"encoding/json"
	"fmt"
	"strings"
)

// statusJSONDoc — подмножество полей status.json из entrypoint.sh (finalize, задача 5.2).
// Дополнительные поля JSON игнорируются для forward compatibility (задача 5.7).
type statusJSONDoc struct {
	Success    bool    `json:"success"`
	ExitCode   int     `json:"exit_code"`
	BranchName *string `json:"branch_name"`
	CommitHash *string `json:"commit_hash"`
}

func normalizeStatusJSONString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// parseStatusJSON парсит status.json и возвращает поля для CodeResult.
// Пустой слайс или битый JSON — ошибка; tolerant к лишним полям через struct tags.
func parseStatusJSON(data []byte) (*statusJSONDoc, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("status.json: empty body")
	}
	var doc statusJSONDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("status.json: %w", err)
	}
	return &doc, nil
}

func applyStatusJSONToCodeResult(cr *CodeResult, doc *statusJSONDoc) {
	if cr == nil || doc == nil {
		return
	}
	cr.Success = doc.Success
	cr.BranchName = normalizeStatusJSONString(doc.BranchName)
	cr.CommitHash = normalizeStatusJSONString(doc.CommitHash)
	// exit_code из JSON не кладём в CodeResult (SandboxStatus.ExitCode — только Docker, 5.7).
}
