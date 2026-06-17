package gitprovider

import "context"

// PipelineStatus — нормализованный статус CI-пайплайна ветки/MR (Sprint 22, CI-gate).
type PipelineStatus string

const (
	// PipelineStatusNone — пайплайна нет / провайдер не поддерживает CI (гейт не блокирует).
	PipelineStatusNone PipelineStatus = "none"
	// PipelineStatusPending — created/pending/running/preparing/… (поллинг продолжается).
	PipelineStatusPending PipelineStatus = "pending"
	// PipelineStatusSuccess — пайплайн зелёный.
	PipelineStatusSuccess PipelineStatus = "success"
	// PipelineStatusFailed — пайплайн упал.
	PipelineStatusFailed PipelineStatus = "failed"
	// PipelineStatusCanceled — пайплайн отменён.
	PipelineStatusCanceled PipelineStatus = "canceled"
	// PipelineStatusSkipped — пайплайн пропущен.
	PipelineStatusSkipped PipelineStatus = "skipped"
)

// IsTerminal — статус, при котором поллинг можно прекращать.
func (s PipelineStatus) IsTerminal() bool {
	switch s {
	case PipelineStatusSuccess, PipelineStatusFailed, PipelineStatusCanceled, PipelineStatusSkipped, PipelineStatusNone:
		return true
	}
	return false
}

// PipelineFailedJob — упавший джоб пайплайна (на каком этапе и что пошло не так).
type PipelineFailedJob struct {
	Name    string
	Stage   string
	WebURL  string
	LogTail string // хвост лога упавшего джоба (усечён)
}

// PipelineResult — результат опроса последнего пайплайна ветки/MR.
type PipelineResult struct {
	Status     PipelineStatus
	WebURL     string
	SHA        string
	FailedJobs []PipelineFailedJob
}

// PipelineStatusReader — опциональная способность провайдера читать статус CI.
// Реализуется GitLab/GitHub; LocalGitProvider её не реализует (нет CI). Оркестратор
// делает type-assert: если провайдер её не поддерживает → статус трактуется как none.
type PipelineStatusReader interface {
	// GetLatestPipelineStatus возвращает статус последнего пайплайна по ref (ветке).
	// При Failed заполняет FailedJobs (этап/джоб + хвост лога), если провайдер это отдаёт.
	GetLatestPipelineStatus(ctx context.Context, repoURL string, ref string) (*PipelineResult, error)
}
