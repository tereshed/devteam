package gitprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// pipelineTraceTailBytes — сколько байт хвоста лога упавшего джоба сохранять.
const pipelineTraceTailBytes = 4 * 1024

// pipelineMaxFailedJobsWithTrace — для скольких упавших джобов тянуть trace (защита от длинных пайплайнов).
const pipelineMaxFailedJobsWithTrace = 3

var _ PipelineStatusReader = (*GitLabProvider)(nil)

// GetLatestPipelineStatus возвращает статус последнего пайплайна ветки ref.
// При Failed догружает упавшие джобы (этап/имя) + хвост их trace-лога.
func (g *GitLabProvider) GetLatestPipelineStatus(ctx context.Context, repoURL, ref string) (*PipelineResult, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	base, path, err := gitlabBaseAndPath(repoURL)
	if err != nil {
		return nil, err
	}
	projPrefix := base + "/api/v4/projects/" + url.PathEscape(path)

	endpoint := projPrefix + "/pipelines?order_by=id&sort=desc&per_page=1&ref=" + url.QueryEscape(ref)
	resp, err := g.apiRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab list pipelines: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrAuthFailed
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrRepoNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("gitlab list pipelines HTTP %d", resp.StatusCode)
	}
	var pipelines []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Ref    string `json:"ref"`
		SHA    string `json:"sha"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&pipelines); err != nil {
		return nil, fmt.Errorf("gitlab decode pipelines: %w", err)
	}
	if len(pipelines) == 0 {
		// Ветка без пайплайна — CI не настроен/не запустился: гейт не блокирует.
		return &PipelineResult{Status: PipelineStatusNone}, nil
	}
	p := pipelines[0]
	res := &PipelineResult{
		Status: mapGitlabPipelineStatus(p.Status),
		WebURL: p.WebURL,
		SHA:    p.SHA,
	}
	if res.Status == PipelineStatusFailed {
		res.FailedJobs = g.failedPipelineJobs(ctx, projPrefix, p.ID)
	}
	return res, nil
}

// mapGitlabPipelineStatus нормализует статус пайплайна GitLab.
func mapGitlabPipelineStatus(s string) PipelineStatus {
	switch s {
	case "success":
		return PipelineStatusSuccess
	case "failed":
		return PipelineStatusFailed
	case "canceled", "canceling":
		return PipelineStatusCanceled
	case "skipped":
		return PipelineStatusSkipped
	case "created", "waiting_for_resource", "preparing", "pending", "running", "scheduled", "manual":
		return PipelineStatusPending
	default:
		return PipelineStatusPending
	}
}

// failedPipelineJobs возвращает упавшие джобы пайплайна (этап/имя + хвост лога).
// Ошибки догрузки не фатальны — возвращаем что есть.
func (g *GitLabProvider) failedPipelineJobs(ctx context.Context, projPrefix string, pipelineID int) []PipelineFailedJob {
	endpoint := projPrefix + "/pipelines/" + strconv.Itoa(pipelineID) + "/jobs?per_page=100"
	resp, err := g.apiRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var jobs []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Stage  string `json:"stage"`
		Status string `json:"status"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&jobs); err != nil {
		return nil
	}
	var out []PipelineFailedJob
	withTrace := 0
	for _, j := range jobs {
		if j.Status != "failed" {
			continue
		}
		fj := PipelineFailedJob{Name: j.Name, Stage: j.Stage, WebURL: j.WebURL}
		if withTrace < pipelineMaxFailedJobsWithTrace {
			fj.LogTail = g.jobTraceTail(ctx, projPrefix, j.ID)
			withTrace++
		}
		out = append(out, fj)
	}
	return out
}

// jobTraceTail тянет trace-лог джоба и возвращает его хвост (последние pipelineTraceTailBytes).
func (g *GitLabProvider) jobTraceTail(ctx context.Context, projPrefix string, jobID int) string {
	endpoint := projPrefix + "/jobs/" + strconv.Itoa(jobID) + "/trace"
	resp, err := g.apiRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	// Берём ИМЕННО хвост лога (там сама ошибка), а не начало: логи джоба часто на
	// мегабайты (docker pull progress и т.п.), а падение — в самом конце. Читаем поток
	// в кольцевой буфер фиксированного размера → в памяти максимум tailKeep байт.
	const tailKeep = 8 * 1024
	buf := make([]byte, 0, tailKeep)
	chunk := make([]byte, 16*1024)
	for {
		n, rerr := resp.Body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > tailKeep {
				buf = buf[len(buf)-tailKeep:]
			}
		}
		if rerr != nil {
			break
		}
	}
	s := strings.TrimSpace(string(buf))
	if len(s) > pipelineTraceTailBytes {
		s = "…(truncated)…\n" + s[len(s)-pipelineTraceTailBytes:]
	}
	return s
}
