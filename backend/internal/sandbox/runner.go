package sandbox

import "context"

// SandboxRunner — абстракция изолированного выполнения задачи (Docker / иной рантайм).
// Реализация по умолчанию: DockerSandboxRunner (задача 5.5).
//
// context.Context в каждом методе задаёт дедлайн/отмену для самого вызова API раннера
// (обращение к Docker Engine, ожидание ответа). Это не то же самое, что бизнес-таймаут задачи:
// см. SandboxOptions.Timeout и комментарии к Wait / RunTask.
//
// Потокобезопасность: все методы реализации обязаны быть безопасны при конкурентных вызовах,
// в том числе для одного и того же sandboxID (например, Stop и Cleanup с разных горутин).
//
// Состояние и рестарт процесса: реализация обязана быть stateless относительно жизненного цикла инстанса
// в смысле crash recovery — источником правды о контейнере остаётся Docker Engine. После полного
// перезапуска Go-процесса Wait, GetStatus, StreamLogs, Stop и Cleanup с валидным sandboxID (или
// повторный attach по имени из TaskID — см. RunTask) должны корректно работать, опрашивая API движка;
// in-memory кэш — опционален и не должен заменять ответы движка.
type SandboxRunner interface {
	// RunTask создаёт и запускает изолированный runtime для одной задачи.
	// opts: образ, RepoURL, ветка, backend, Timeout (бизнес-таймаут), лимиты, EnvVars для entrypoint.
	// Не дожидается завершения процесса в контейнере — используйте Wait.
	// В SandboxInstance.ID возвращается только доверенный ID рантайма (см. ValidateSandboxID).
	// Первая строка тела RunTask: opts = opts.Clone() — снимок полей и глубокая копия EnvVars (гонки с вызывающим кодом).
	// Далее: opts.Validate(ctx), ValidateRepoURL(ctx, opts.RepoURL), ValidateBranchName(opts.Branch), ValidateEnvKeys(opts.EnvVars).
	//
	// Сироты: если Docker уже выдал ID контейнера (create успешен), а последующий шаг (start, pull и т.д.)
	// падает из-за ошибки или отмены ctx, реализация обязана best-effort удалить этот контейнер до возврата
	// ошибки, чтобы не оставлять запись в статусе Created без известного вызывающему ID в ответе RunTask.
	//
	// Идемпотентность по TaskID: имя контейнера детерминированно от opts.TaskID (ориентир TaskContainerNamePrefix
	// + нормализованный идентификатор задачи). Если контейнер с таким именем уже существует: либо вернуть
	// SandboxInstance существующего инстанса без второго clone/push, либо ErrSandboxRunConflict — выбранная
	// политика фиксируется в реализации 5.5 и не должна порождать два параллельных контейнера на одну задачу.
	RunTask(ctx context.Context, opts SandboxOptions) (*SandboxInstance, error)

	// Wait блокирует выполнение до exited/stopped контейнера, отмены/дедлайна ctx этого вызова
	// либо до срабатывания бизнес-таймаута opts.Timeout из RunTask (принудительная остановка по 5.5/5.8).
	// Не использует поллинг GetStatus в цикле — только нативное ожидание рантайма (например ContainerWait).
	//
	// Два уровня таймаутов: отмена только ctx возвращает context.Canceled / DeadlineExceeded;
	// контейнер при этом не обязан исчезнуть немедленно — его ограничивает opts.Timeout и/или Stop/Cleanup.
	// Срабатывание opts.Timeout останавливает контейнер независимо от короткого HTTP-ctx у Wait (нет «сирот» по лимиту задачи).
	// При остановке по бизнес-таймауту Status в SandboxStatus — SandboxStatusTimedOut (не failed/stopped).
	//
	// Несколько горутин могут вызывать Wait для одного sandboxID: все должны получить согласованный
	// финальный статус (одинаковые поля статуса по смыслу; допускается различие неизменяемых деталей вроде
	// RunningFor на снимке). Wait не должен неявно вызывать Cleanup и не «съедает» инстанс для других Wait.
	Wait(ctx context.Context, sandboxID string) (*SandboxStatus, error)

	// GetStatus возвращает снимок статуса (UI, диагностика).
	// Антипаттерн: цикл GetStatus как замена Wait — запрещён (нагрузка на CPU и Docker API).
	GetStatus(ctx context.Context, sandboxID string) (*SandboxStatus, error)

	// StreamLogs открывает поток логов stdout/stderr в виде канала.
	//
	// Вариант А (MVP): не более одного активного стрима на sandboxID; повторный вызов —
	// ErrStreamAlreadyActive. Fan-out (БД + WebSocket) — ответственность оркестратора (tee одного канала).
	// Канал закрывается при завершении стрима, отмене ctx или Cleanup (детали реализации — 5.6).
	// Ошибка чтения логов (обрыв Docker API и т.д.) передаётся в LogEntry.Error; потребитель обязан проверять.
	//
	// Контракт потребителя канала: вызывающая сторона обязана либо вычитывать канал до закрытия,
	// либо отменить ctx, переданный в StreamLogs, если чтение прервано (клиент отвалился и т.д.).
	//
	// Буферизация и backpressure: канал <-chan LogEntry обязан быть буферизованным (ориентир StreamLogsDefaultBuffer,
	// допустимо порядка 1e3–5e3). Чтение из Docker API не должно блокироваться на отправке в канал из-за
	// медленного потребителя: при переполнении буфера — либо дроп старых записей с явной пометкой о потере
	// в потоке (договорённость 5.6), либо завершение стрима с LogEntry.Error, но не оставлять процесс в
	// контейнере зависшим на write в stdout/stderr.
	//
	// Чанкирование строк: см. LogEntry / LogEntryMaxLineBytes — длинные логические строки без '\n'
	// или длиннее лимита обязаны резаться на несколько LogEntry.
	StreamLogs(ctx context.Context, sandboxID string) (<-chan LogEntry, error)

	// Stop инициирует остановку контейнера: сначала graceful (SIGTERM / docker stop с таймаутом,
	// например 10 с), затем при отсутствии остановки — SIGKILL (аналог docker stop -t 10).
	// Контейнер должен быть остановлен гарантированно; ctx ограничивает только сам RPC к движку.
	Stop(ctx context.Context, sandboxID string) error

	// StopTask останавливает задачу по TaskID: до появления containerID отменяет фазу creating (5.8),
	// после — то же, что Stop по sandboxID (поиск в реестре или inspect по детерминированному имени контейнера).
	StopTask(ctx context.Context, taskID string) error

	// Cleanup удаляет контейнер и связанные ресурсы; желательна идемпотентность.
	// Обязанность: завершить активные стримы StreamLogs для этого sandboxID, закрыть канал,
	// разблокировать ожидающий Wait (ошибка состояния / sentinel), не оставлять горутины после удаления.
	// Также удалить выделенные для инстанса Docker volumes, временные каталоги/bind-mount на хосте
	// и изолированные bridge-сети, созданные только под этот sandbox (best-effort, без утечки диска).
	//
	// Важно: не передавайте сюда отменённый HTTP- или задачный ctx из defer без обёртки — Docker API
	// вернёт context.Canceled и контейнер может остаться. Используйте context.WithoutCancel(parent)
	// (Go 1.21+) или context.WithTimeout(context.Background(), …) с отдельным дедлайном на уборку.
	Cleanup(ctx context.Context, sandboxID string) error
}
