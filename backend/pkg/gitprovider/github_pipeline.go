package gitprovider

import (
	"context"

	"github.com/google/go-github/v67/github"
)

var _ PipelineStatusReader = (*GitHubProvider)(nil)

// GetLatestPipelineStatus агрегирует статус CI ветки ref в GitHub: combined commit
// status (legacy) + check-runs (GitHub Actions). Failed → собираем имена упавших
// чеков как FailedJobs (без trace — логи GitHub требуют отдельной выгрузки).
func (g *GitHubProvider) GetLatestPipelineStatus(ctx context.Context, repoURL, ref string) (*PipelineResult, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	res := &PipelineResult{Status: PipelineStatusNone}
	seen := false
	pending := false
	var failed []PipelineFailedJob

	// 1) Combined commit status (legacy statuses API).
	if cs, _, cerr := g.client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, nil); cerr == nil && cs != nil {
		if cs.GetTotalCount() > 0 {
			seen = true
			switch cs.GetState() {
			case "failure", "error":
				for _, st := range cs.Statuses {
					if c := st.GetState(); c == "failure" || c == "error" {
						failed = append(failed, PipelineFailedJob{Name: st.GetContext(), WebURL: st.GetTargetURL()})
					}
				}
			case "pending":
				pending = true
			}
		}
	}

	// 2) Check-runs (GitHub Actions / современные проверки).
	if crs, _, crerr := g.client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, &github.ListCheckRunsOptions{}); crerr == nil && crs != nil && crs.GetTotal() > 0 {
		seen = true
		for _, cr := range crs.CheckRuns {
			if cr.GetStatus() != "completed" {
				pending = true
				continue
			}
			switch cr.GetConclusion() {
			case "failure", "timed_out", "cancelled", "action_required":
				failed = append(failed, PipelineFailedJob{Name: cr.GetName(), WebURL: cr.GetHTMLURL()})
			}
		}
	}

	switch {
	case len(failed) > 0:
		res.Status = PipelineStatusFailed
		res.FailedJobs = failed
	case pending:
		res.Status = PipelineStatusPending
	case seen:
		res.Status = PipelineStatusSuccess
	default:
		res.Status = PipelineStatusNone
	}
	return res, nil
}
