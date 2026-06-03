// cmd/scheduler — выделенный бинарь роли scheduler: процессы-синглтоны через leader election
// (cron, токен-рефрешеры, retention, stale-recovery, workflow-worker, initial model-sync).
// Без HTTP/MCP и без пулов воркеров. Можно держать 2+ реплик для отказоустойчивости —
// LeaderElector гарантирует, что синглтоны исполняет ровно одна из них.
package main

import "github.com/devteam/backend/internal/app"

func main() {
	app.Run(app.RoleScheduler)
}
