package ws

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dispatchedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_hubbridge_dispatched_total",
		Help: "Total number of messages dispatched by the hub bridge to the hub",
	}, []string{"message_type"})

	dispatchErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_hubbridge_dispatch_errors_total",
		Help: "Total number of errors encountered by the hub bridge during dispatch",
	}, []string{"error_kind"})
)

type prometheusBridgeMetrics struct{}

func NewPrometheusBridgeMetrics() BridgeMetrics {
	return &prometheusBridgeMetrics{}
}

func (m *prometheusBridgeMetrics) IncDispatched(msgType string) {
	dispatchedTotal.WithLabelValues(msgType).Inc()
}

func (m *prometheusBridgeMetrics) IncDispatchError(kind string) {
	dispatchErrorsTotal.WithLabelValues(kind).Inc()
}
