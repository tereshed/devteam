package events

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	publishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_eventbus_published_total",
		Help: "Total number of events published to the event bus",
	}, []string{"event_type"})

	droppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_eventbus_dropped_total",
		Help: "Total number of events dropped by the event bus due to slow subscribers",
	}, []string{"subscriber", "event_type"})

	subscribersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ws_eventbus_subscribers",
		Help: "Current number of active subscribers in the event bus",
	})
)

type prometheusMetrics struct{}

func NewPrometheusMetrics() Metrics {
	return &prometheusMetrics{}
}

func (m *prometheusMetrics) IncPublished(eventType string) {
	publishedTotal.WithLabelValues(eventType).Inc()
}

func (m *prometheusMetrics) IncDropped(subscriber string, eventType string) {
	droppedTotal.WithLabelValues(subscriber, eventType).Inc()
}

func (m *prometheusMetrics) SetSubscribers(count int) {
	subscribersGauge.Set(float64(count))
}
