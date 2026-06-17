package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/google/uuid"
)

// ciGateNoneGrace — сколько ждём появления пайплайна, прежде чем трактовать «нет
// пайплайна» как «у ветки нет CI» (GitLab создаёт пайплайн MR не мгновенно).
const ciGateNoneGrace = 3 * time.Minute

// ciReasonTraceTailBytes — лимит хвоста лога одного джоба в тексте причины needs_human.
const ciReasonTraceTailBytes = 1500

// ciGateTarget — один открытый MR, чей пайплайн ждём.
type ciGateTarget struct {
	repo   *models.ProjectRepository // nil → одно-репо (поля проекта)
	branch string
	prURL  string
	slug   string
}

// startCIGate запускает фоновое ожидание CI-пайплайнов открытых MR (CI-gate, Sprint 22).
// Не блокирует post-commit хук: поллинг идёт в отдельной горутине со своим таймаутом.
// ВНИМАНИЕ (MVP): горутина не переживает рестарт ноды — потерянный поллинг оставит задачу
// в done без CI-проверки (надёжный leader-раннер — follow-up).
func (o *Orchestrator) startCIGate(project *models.Project, taskID uuid.UUID, targets []ciGateTarget) {
	if !o.cfg.CIGateEnabled || o.prPublisher == nil || project == nil {
		return
	}
	live := targets[:0]
	for _, t := range targets {
		if strings.TrimSpace(t.branch) != "" {
			live = append(live, t)
		}
	}
	if len(live) == 0 {
		return
	}
	go o.runCIGate(project, taskID, append([]ciGateTarget(nil), live...))
}

// runCIGate опрашивает статус пайплайнов до терминального исхода/таймаута.
// Любой failed → downgradeToNeedsHuman с деталями (этап/джоб + хвост лога). Все
// зелёные/без-CI → задача остаётся done.
func (o *Orchestrator) runCIGate(project *models.Project, taskID uuid.UUID, targets []ciGateTarget) {
	interval := o.cfg.CIGatePollInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timeout := o.cfg.CIGateTimeout
	if timeout <= 0 {
		timeout = 25 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	o.logger.InfoContext(ctx, "ci-gate: waiting for pipelines", "task_id", taskID, "targets", len(targets))

	pending := make(map[int]ciGateTarget, len(targets))
	for i, t := range targets {
		pending[i] = t
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			o.logger.WarnContext(context.Background(), "ci-gate: timeout — leaving task done without CI confirmation",
				"task_id", taskID, "unresolved", len(pending))
			return
		case <-ticker.C:
		}

		for i, t := range pending {
			res, err := o.prPublisher.LatestPipelineStatus(ctx, project, t.repo, t.branch)
			if err != nil {
				o.logger.WarnContext(ctx, "ci-gate: poll error (will retry)", "task_id", taskID, "slug", t.slug, "error", err.Error())
				continue
			}
			switch res.Status {
			case gitprovider.PipelineStatusFailed:
				reason := buildCIFailureReason(t, res)
				o.logger.WarnContext(context.Background(), "ci-gate: pipeline failed → downgrading done→needs_human",
					"task_id", taskID, "slug", t.slug, "pipeline", res.WebURL, "failed_jobs", len(res.FailedJobs))
				o.downgradeToNeedsHuman(context.Background(), taskID, reason)
				return
			case gitprovider.PipelineStatusSuccess:
				o.logger.InfoContext(ctx, "ci-gate: pipeline green", "task_id", taskID, "slug", t.slug)
				delete(pending, i)
			case gitprovider.PipelineStatusCanceled, gitprovider.PipelineStatusSkipped:
				o.logger.InfoContext(ctx, "ci-gate: pipeline inconclusive — not blocking",
					"task_id", taskID, "slug", t.slug, "status", string(res.Status))
				delete(pending, i)
			case gitprovider.PipelineStatusNone:
				if time.Since(start) >= ciGateNoneGrace {
					o.logger.InfoContext(ctx, "ci-gate: no pipeline for branch (no CI) — not blocking", "task_id", taskID, "slug", t.slug)
					delete(pending, i)
				}
			case gitprovider.PipelineStatusPending:
				// продолжаем опрашивать
			}
		}
		if len(pending) == 0 {
			o.logger.InfoContext(ctx, "ci-gate: all pipelines resolved — task stays done (CI verified)", "task_id", taskID)
			return
		}
	}
}

// buildCIFailureReason формирует читаемую причину needs_human: какой этап/джоб упал и хвост его лога.
func buildCIFailureReason(t ciGateTarget, res *gitprovider.PipelineResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Задача не завершена: CI-пайплайн упал (репозиторий %s, ветка %s).\n", t.slug, t.branch)
	if t.prURL != "" {
		fmt.Fprintf(&b, "MR: %s\n", t.prURL)
	}
	if res.WebURL != "" {
		fmt.Fprintf(&b, "Пайплайн: %s\n", res.WebURL)
	}
	if len(res.FailedJobs) == 0 {
		b.WriteString("Упавшие джобы: детали недоступны (см. пайплайн).")
		return b.String()
	}
	b.WriteString("Упавшие джобы:\n")
	for _, j := range res.FailedJobs {
		stage := j.Stage
		if stage == "" {
			stage = "-"
		}
		fmt.Fprintf(&b, "• [%s] %s", stage, j.Name)
		if j.WebURL != "" {
			fmt.Fprintf(&b, " (%s)", j.WebURL)
		}
		b.WriteByte('\n')
		if tail := strings.TrimSpace(j.LogTail); tail != "" {
			if len(tail) > ciReasonTraceTailBytes {
				tail = "…(truncated)…\n" + tail[len(tail)-ciReasonTraceTailBytes:]
			}
			b.WriteString("```\n")
			b.WriteString(tail)
			b.WriteString("\n```\n")
		}
	}
	return b.String()
}
