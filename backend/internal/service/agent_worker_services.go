package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/sandbox"
)

// attachSandboxServices добавляет в in.Services эфемерные сервис-сайдкары проекта
// (Sprint 22), если агент включил attach_sandbox_services и это sandbox-агент.
// Пароль БД генерится на каждый прогон (случайный, не хранится). Ошибки НЕ валят
// задачу — логируем и продолжаем без сервиса (тестер сам выдаст blocked, если БД
// реально нужна, как и до фичи).
func (w *AgentWorker) attachSandboxServices(ctx context.Context, task *models.Task, agentRec *models.Agent, in *agent.ExecutionInput) {
	if w.sandboxServiceRepo == nil || task == nil || agentRec == nil || in == nil {
		return
	}
	if !agentRec.AttachSandboxServices || agentRec.ExecutionKind != models.AgentExecutionKindSandbox {
		return
	}
	configs, err := w.sandboxServiceRepo.ListEnabledByProject(ctx, task.ProjectID)
	if err != nil {
		w.logger.WarnContext(ctx, "sandbox services: list failed",
			"error", err.Error(), "task_id", task.ID.String())
		return
	}
	for i := range configs {
		c := &configs[i]
		pass, perr := generateServicePassword()
		if perr != nil {
			w.logger.WarnContext(ctx, "sandbox services: password gen failed", "error", perr.Error())
			return
		}
		spec := sandbox.ServiceSpec{
			Alias:            c.Alias,
			Image:            c.Image,
			Port:             c.Port,
			ReadyTimeoutSecs: c.ReadyTimeoutSeconds,
			Env: map[string]string{
				sandbox.EnvPostgresDB:       c.DBName,
				sandbox.EnvPostgresUser:     c.DBUser,
				sandbox.EnvPostgresPassword: pass,
			},
		}
		// Только inline-сид кладётся в /docker-entrypoint-initdb.d. repo_file
		// резолвится самим тестом (репо у него на диске), none — без сида.
		if c.SeedKind == models.SandboxSeedInline {
			spec.SeedSQL = c.SeedValue
		}
		in.Services = append(in.Services, spec)
	}
	if len(in.Services) > 0 {
		w.logger.InfoContext(ctx, "sandbox services attached",
			"task_id", task.ID.String(), "agent", agentRec.Name, "count", len(in.Services))
	}
}

// generateServicePassword — случайный пароль БД сервис-контейнера (на один прогон).
func generateServicePassword() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
