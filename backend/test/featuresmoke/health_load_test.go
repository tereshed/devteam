//go:build featuresmoke

package featuresmoke

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// health_load_test.go — P3 «/health под нагрузкой».
//
// Контракт: /health должен выдерживать burst запросов (например, при
// re-scheduling kubernetes-pod'а под нагрузкой) без 5xx и без таймаутов.
// Мы не нагружаем продакшн-чарт — это smoke на «handler цепляет sql.Ping
// дёшево и не блокируется».
//
// Сценарий: N goroutines × M запросов. Каждый запрос должен:
//   - вернуться за < perReqTimeout;
//   - status = 200;
//   - тело прочитано до конца (иначе keep-alive не работает и измеряем
//     TCP-хендшейки вместо самого /health — см. review §1).
//
// Числа подобраны так, чтобы не задушить YugabyteDB в CI (4 conn limit
// для тестового пользователя): 20 параллельных воркеров × 25 запросов =
// 500 HTTP запросов, шиплем за ~10s. На локалке YBDB 5433 спокойно
// держит, в GHA — тоже (один контейнер, hostNetwork).
//
// Если /health под нагрузкой ловит timeout/5xx — это симптом, что backend
// блокирует sql connection pool на длинных запросах, или handler делает
// что-то тяжёлое. Любой fail здесь — баг в проде.
//
// Транспорт ОДИН на тест:
//   - http.Transport потокобезопасен и сам параллелит соединения через
//     MaxIdleConnsPerHost. Заводить транспорт per-goroutine не имеет смысла:
//     те, что без явного Transport, всё равно шарят http.DefaultTransport
//     (см. review §1, claim про «сериализацию пула» был ошибочным).
//   - Явный Transport нужен только чтобы поднять MaxIdleConnsPerHost
//     (default=2) до нужной для теста ширины: иначе при 20 воркерах большую
//     часть запросов клиент будет открывать заново, и мы вернёмся к
//     бенчмаркингу TCP-хендшейков.

func TestHealth_UnderLoad(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	const (
		workers           = 20
		requestsPerWorker = 25
		perReqTimeout     = 2 * time.Second
		// Хвост запросов может занять чуть больше из-за гранулярности
		// connection-пула YBDB, поэтому общий timeout 60s.
		overallTimeout = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// Один shared client — Transport потокобезопасен, не клонируем default'ом
	// http.DefaultTransport, чтобы не задеть остальные тесты, гоняющие через
	// Harness.hc (он живёт со своим DefaultTransport).
	transport := &http.Transport{
		MaxIdleConns:        workers * 2,
		MaxIdleConnsPerHost: workers * 2,
		IdleConnTimeout:     30 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: perReqTimeout}

	var (
		ok   atomic.Int64
		slow atomic.Int64
		fail atomic.Int64
		wg   sync.WaitGroup
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < requestsPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				start := time.Now()
				req, err := http.NewRequestWithContext(ctx, "GET", h.BaseURL+"/health", nil)
				if err != nil {
					fail.Add(1)
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					fail.Add(1)
					continue
				}
				// Полностью читаем тело — иначе keep-alive не может вернуть
				// соединение в пул, и при следующем запросе клиент откроет
				// новое TCP-соединение (см. review §1).
				if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
					t.Logf("worker %d: copy body: %v", w, copyErr)
				}
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Logf("worker %d: close body: %v", w, closeErr)
				}
				dur := time.Since(start)
				if resp.StatusCode != http.StatusOK {
					fail.Add(1)
					continue
				}
				if dur > perReqTimeout {
					slow.Add(1)
					continue
				}
				ok.Add(1)
			}
		}()
	}

	wg.Wait()
	cancel()

	total := int64(workers * requestsPerWorker)
	if fail.Load() > 0 {
		t.Fatalf("/health под нагрузкой: %d/%d запросов завершились ошибкой/non-200",
			fail.Load(), total)
	}
	if slow.Load() > 0 {
		// «Медленный» — это > perReqTimeout (2s). Любой такой случай уже плохо.
		t.Fatalf("/health под нагрузкой: %d/%d запросов выполнились дольше %s",
			slow.Load(), total, perReqTimeout)
	}
	if ok.Load() != total {
		t.Fatalf("/health под нагрузкой: ok=%d/%d (баг в подсчёте)", ok.Load(), total)
	}
}
