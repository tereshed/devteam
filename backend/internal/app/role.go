package app

import (
	"os"
	"strings"
)

// Role — роль процесса при горизонтальном масштабировании с разделением concern'ов.
//
// Связанность по событиям учтена так: Hub/HubBridge/ClusterBridge, EventBus и индексаторы
// строятся и работают на ВСЕХ ролях. Это нужно, чтобы worker-нода доставляла свои доменные
// события (артефакты, router-решения) в WS-клиентов api-нод через Redis fan-out, а индексаторы
// обрабатывали локально-рождённые события (без дублей — события не пересекают границу процесса).
// Роль гейтит лишь ТОЧКИ СТАРТА: HTTP/MCP, пулы воркеров, leader-tasks.
type Role string

const (
	// RoleAll — всё в одном процессе (дефолт; одноинстансный/dev режим — поведение как прежде).
	RoleAll Role = "all"
	// RoleAPI — HTTP API + WebSocket + MCP. Без пулов воркеров и leader-tasks.
	RoleAPI Role = "api"
	// RoleWorker — пулы step/agent воркеров (claim-safe, task_events + SKIP LOCKED). Без HTTP.
	RoleWorker Role = "worker"
	// RoleScheduler — leader-tasks (cron/refreshers/retention/workflow/model-sync). Без HTTP и пулов.
	RoleScheduler Role = "scheduler"
)

// RoleFromEnv читает APP_ROLE (api|worker|scheduler|all). Пусто/неизвестно → RoleAll.
func RoleFromEnv() Role {
	switch Role(strings.ToLower(strings.TrimSpace(os.Getenv("APP_ROLE")))) {
	case RoleAPI:
		return RoleAPI
	case RoleWorker:
		return RoleWorker
	case RoleScheduler:
		return RoleScheduler
	default:
		return RoleAll
	}
}

// RunsHTTP — обслуживать ли HTTP API/WS/MCP (принимать клиентский трафик).
func (r Role) RunsHTTP() bool { return r == RoleAll || r == RoleAPI }

// RunsWorkers — запускать ли пулы step/agent воркеров.
func (r Role) RunsWorkers() bool { return r == RoleAll || r == RoleWorker }

// RunsLeaderTasks — участвовать ли в leader election и исполнять процессы-синглтоны.
func (r Role) RunsLeaderTasks() bool { return r == RoleAll || r == RoleScheduler }
