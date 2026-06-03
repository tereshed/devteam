package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// leader_elector.go — lease-based leader election поверх БД для процессов-синглтонов
// при горизонтальном масштабировании.
//
// Зачем: cron-планировщик, токен-рефрешеры, retention, stale-recovery и workflow-worker
// не являются claim-safe (в отличие от step/agent-воркеров на task_events + SKIP LOCKED).
// Если их запустить на N репликах, они продублируются: cron сработает N раз, два рефрешера
// устроят гонку на обновлении токена, workflow-worker выполнит один execution дважды.
//
// Механизм: строка-лиз в таблице leader_leases. Лидерство = владение непросроченным лизом.
// Acquire/renew — атомарный CAS (INSERT ... ON CONFLICT DO UPDATE ... WHERE holder=me OR
// expires_at<now()). Источник времени — БД (now()), поэтому рассинхрон часов между
// инстансами не ломает выбор лидера. YugabyteDB 2.20 не поддерживает pg advisory locks —
// отсюда табличный лиз, а не pg_try_advisory_lock.
//
// Использование: задачи-синглтоны регистрируются через OnLeader ДО Run. Run в фоне
// держит лиз; при получении лидерства запускает все задачи в дочернем контексте, при
// потере — отменяет его (задачи обязаны уважать ctx.Done()).

const (
	defaultLeaseTTL      = 30 * time.Second
	defaultRenewInterval = 10 * time.Second
)

// LeaseStore — хранилище лиза (CAS-acquire + release). Вынесено за интерфейс для
// юнит-тестирования supervisor-логики LeaderElector без реальной БД.
type LeaseStore interface {
	// Acquire атомарно захватывает или продлевает лиз name для holder на ttl.
	// Возвращает true, если holder теперь владеет непросроченным лизом.
	Acquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error)
	// Release освобождает лиз, если им владеет holder (best-effort, ускоряет failover).
	Release(ctx context.Context, name, holder string) error
}

// dbLeaseStore — реализация LeaseStore поверх YugabyteDB/Postgres.
type dbLeaseStore struct {
	db *gorm.DB
}

// NewDBLeaseStore создаёт LeaseStore поверх таблицы leader_leases (миграция 070).
func NewDBLeaseStore(db *gorm.DB) LeaseStore {
	return &dbLeaseStore{db: db}
}

// CAS: вставляем лиз; при конфликте перезаписываем ТОЛЬКО если им владеем мы сами
// (renew) ИЛИ он просрочен (takeover). Иначе ON CONFLICT-UPDATE не срабатывает,
// RETURNING не вернёт строк — значит лидер кто-то другой. Интервал считаем в БД,
// чтобы привязать срок к единым серверным часам.
const acquireLeaseSQL = `
INSERT INTO leader_leases (name, holder, acquired_at, expires_at)
VALUES (?, ?, now(), now() + (interval '1 second' * ?))
ON CONFLICT (name) DO UPDATE
SET holder = EXCLUDED.holder,
    acquired_at = now(),
    expires_at = EXCLUDED.expires_at
WHERE leader_leases.holder = EXCLUDED.holder
   OR leader_leases.expires_at < now()
RETURNING holder`

func (s *dbLeaseStore) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (bool, error) {
	ttlSeconds := int(ttl / time.Second)
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	var got string
	// При неудачном CAS (лиз держит другой и он не просрочен) RETURNING вернёт 0 строк —
	// got останется пустым, ошибки нет.
	if err := s.db.WithContext(ctx).Raw(acquireLeaseSQL, name, holder, ttlSeconds).Scan(&got).Error; err != nil {
		return false, err
	}
	return got == holder, nil
}

func (s *dbLeaseStore) Release(ctx context.Context, name, holder string) error {
	return s.db.WithContext(ctx).
		Exec(`DELETE FROM leader_leases WHERE name = ? AND holder = ?`, name, holder).Error
}

// leaderTask — именованная задача-синглтон, исполняемая только лидером.
type leaderTask struct {
	name string
	fn   func(context.Context)
}

// LeaderElector держит лиз и супервизирует задачи-синглтоны (см. описание файла).
type LeaderElector struct {
	store      LeaseStore
	name       string
	instanceID string
	ttl        time.Duration
	interval   time.Duration
	log        *slog.Logger

	leader atomic.Bool

	mu         sync.Mutex
	tasks      []leaderTask
	leaderStop context.CancelFunc // отменяет дочерний ctx задач при потере лидерства
}

// NewLeaderElector создаёт elector с дефолтными TTL (30s) и интервалом renew (10s).
func NewLeaderElector(store LeaseStore, name, instanceID string, log *slog.Logger) *LeaderElector {
	return NewLeaderElectorWithConfig(store, name, instanceID, defaultLeaseTTL, defaultRenewInterval, log)
}

// NewLeaderElectorWithConfig — конструктор с явными TTL/интервалом (для тестов).
func NewLeaderElectorWithConfig(store LeaseStore, name, instanceID string, ttl, interval time.Duration, log *slog.Logger) *LeaderElector {
	if log == nil {
		log = slog.Default()
	}
	return &LeaderElector{
		store:      store,
		name:       name,
		instanceID: instanceID,
		ttl:        ttl,
		interval:   interval,
		log:        log,
	}
}

// OnLeader регистрирует задачу-синглтон. Вызывать ДО Run. fn получает контекст, живущий
// пока инстанс лидер; fn обязана завершаться при ctx.Done() (Run(ctx)-loop или
// <-ctx.Done() + cleanup). Разовые задачи (например, прогрев кэша) просто выполняются
// и выходят — повторно запустятся при следующем получении лидерства.
func (e *LeaderElector) OnLeader(name string, fn func(context.Context)) {
	if fn == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tasks = append(e.tasks, leaderTask{name: name, fn: fn})
}

// IsLeader сообщает, владеет ли инстанс лизом сейчас (для job-level гейтинга, если нужно).
func (e *LeaderElector) IsLeader() bool { return e.leader.Load() }

// Run держит лиз до закрытия ctx. Блокируется. Первая попытка — сразу, далее раз в interval.
func (e *LeaderElector) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	e.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			e.stepDown()
			// best-effort release: ускоряет переезд лидерства на другой инстанс
			// (иначе ждать до TTL). Отдельный короткий ctx — родительский уже отменён.
			relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if err := e.store.Release(relCtx, e.name, e.instanceID); err != nil {
				e.log.Warn("leader election: release failed", "error", err, "lease", e.name)
			}
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *LeaderElector) tick(ctx context.Context) {
	won, err := e.store.Acquire(ctx, e.name, e.instanceID, e.ttl)
	if err != nil {
		e.log.Warn("leader election: acquire failed", "error", err, "lease", e.name)
		// Не понижаем лидерство по разовой ошибке БД: лиз ещё мог быть нашим.
		// Если ошибки продолжатся, лиз протухнет по TTL и его заберёт другой инстанс.
		return
	}
	was := e.leader.Load()
	switch {
	case won && !was:
		e.leader.Store(true)
		e.startTasks(ctx)
		e.log.Info("leader election: acquired leadership", "lease", e.name, "instance", e.instanceID)
	case !won && was:
		e.leader.Store(false)
		e.stepDown()
		e.log.Warn("leader election: lost leadership", "lease", e.name, "instance", e.instanceID)
	}
}

// startTasks запускает все зарегистрированные задачи в свежем дочернем контексте.
func (e *LeaderElector) startTasks(parent context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leaderStop != nil { // подстраховка от двойного запуска
		e.leaderStop()
	}
	lctx, cancel := context.WithCancel(parent)
	e.leaderStop = cancel
	for _, t := range e.tasks {
		t := t
		go func() {
			e.log.Info("leader task started", "task", t.name)
			t.fn(lctx)
			e.log.Info("leader task exited", "task", t.name)
		}()
	}
}

// stepDown отменяет контекст задач лидера (останавливает их).
func (e *LeaderElector) stepDown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leaderStop != nil {
		e.leaderStop()
		e.leaderStop = nil
	}
}
