// cmd/worker — выделенный бинарь роли worker: пулы step/agent воркеров (claim-safe,
// task_events + SKIP LOCKED). Без HTTP/MCP и без leader-tasks. Масштабируется независимо
// от API. Доменные события, рождённые воркерами, долетают до WS-клиентов api-нод через
// Redis fan-out (Hub/HubBridge/ClusterBridge работают и в этой роли).
package main

import "github.com/devteam/backend/internal/app"

func main() {
	app.Run(app.RoleWorker)
}
