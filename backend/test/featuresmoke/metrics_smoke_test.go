//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"strings"
	"testing"
)

// metrics_smoke_test.go — P3 экспозиция Prometheus-метрик на /metrics.
//
// Что покрываем:
//   - GET /metrics — 200, Content-Type начинается с text/plain (Prometheus
//     exposition format не v2-protobuf, а человекочитаемый text;
//     именно его scrape'ит Prometheus + Grafana).
//   - Тело содержит как минимум один из наших custom-counter'ов
//     (`async_task_total`, `ws_eventbus_published_total`, …) — гарантирует,
//     что в /metrics реально подцеплен наш реестр, а не дефолтный пустой.
//   - Тело содержит стандартный Go-runtime metric (`go_goroutines`) —
//     promauto регистрирует свои в DefaultRegisterer, и promhttp.Handler
//     отдаёт всё из неё.
//
// /metrics открыт без auth — это интенциональный выбор (см. server.go).
// В проде закрывается сетевым уровнем (отдельный listener / firewall).

func TestMetrics_ExposesPrometheusFormat(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	resp := h.Do(t, "GET", "/metrics", nil, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("/metrics: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	ct := resp.Header.Get("Content-Type")
	// Prometheus client-go отдаёт что-то вроде:
	//   text/plain; version=0.0.4; charset=utf-8; escaping=values
	// либо OpenMetrics при Accept'е на это. Достаточно проверки на text/plain.
	if !strings.HasPrefix(ct, "text/plain") &&
		!strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Fatalf("/metrics: Content-Type=%q (ожидали text/plain или openmetrics)", ct)
	}

	body := string(resp.Body)
	// 1. Стандартный Go-runtime — гарантирует, что promhttp.Handler цепляется
	//    к нашему DefaultRegisterer (и значит promauto-метрики тоже будут).
	if !strings.Contains(body, "go_goroutines") {
		t.Fatalf("/metrics: тело не содержит go_goroutines — promhttp.Handler не подцеплен. "+
			"Первые 500 байт: %s", truncBody(resp.Body))
	}

	// 2. Хоть один из наших custom-counter'ов. Имена брал из:
	//    internal/metrics/async_metrics.go и internal/domain/events/metrics.go.
	//    Если хотя бы один зарегистрирован — значит наш код проинициализирован
	//    и promauto.NewCounterVec не упал. На свежем backend счётчики могут
	//    быть в нулях, но имя в ответе появится (Prometheus экспортирует
	//    зарегистрированные метрики даже с нулевым значением).
	expectedSomeOf := []string{
		"async_task_total",
		"ws_eventbus_subscribers",
	}
	foundSomeCustom := false
	for _, name := range expectedSomeOf {
		if strings.Contains(body, name) {
			foundSomeCustom = true
			break
		}
	}
	if !foundSomeCustom {
		head := resp.Body
		if len(head) > 800 {
			head = head[:800]
		}
		t.Fatalf("/metrics: ни одна из custom-метрик %v не найдена. "+
			"Первые 800 байт: %s", expectedSomeOf, truncBody(head))
	}
}

// TestMetrics_NoAuthRequired — открытый endpoint, без Authorization.
// Дублирует часть TestMetrics_ExposesPrometheusFormat (мы там уже шлём без
// token'а), но фиксирует контракт отдельным тестом для регрессии:
// если кто-то навесит authMW — этот тест моментально упадёт.
func TestMetrics_NoAuthRequired(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	resp := h.Do(t, "GET", "/metrics", nil, "")
	if resp.Status == http.StatusUnauthorized || resp.Status == http.StatusForbidden {
		t.Fatalf("/metrics: status=%d — auth был случайно навешен; "+
			"prometheus scrape без token'а сломается", resp.Status)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("/metrics no token: status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
}
