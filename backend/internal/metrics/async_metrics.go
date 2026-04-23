package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AsyncTaskTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "async_task_total",
		Help: "Total number of async tasks executed",
	}, []string{"action", "status"})
)

// IncAsyncTask инкрементирует счетчик асинхронных задач.
func IncAsyncTask(action, status string) {
	AsyncTaskTotal.WithLabelValues(action, status).Inc()
}
