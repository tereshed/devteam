package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

// Sprint 21 — Assistant Sidebar (docs/tasks/21-assistant-sidebar.md §2).
//
// Слоевой инвариант (docs/rules/backend.md §2.1): любой SQL/GORM/`tx.Raw`/`tx.Exec`/
// `WithContext(...).Transaction(`/ FOR UPDATE живёт ТОЛЬКО в этом файле.
// AssistantService не имеет поля `db *gorm.DB`, не открывает транзакций и не знает
// про индексы/CHECK-констрейнты — он оперирует только методами этого интерфейса.

const (
	assistantMessagesDefaultLimit = 30
	assistantMessagesMaxLimit     = 100

	// Sessions и messages имеют разные паттерны вывода в UI: sidebar держит
	// «недавние сессии» (короткий список), история сообщений листается курсором.
	// Лимиты должны эволюционировать независимо.
	assistantSessionsDefaultLimit = 50
	assistantSessionsMaxLimit     = 200
)

// AssistantSessionRepository — единая точка SQL для assistant_sessions и assistant_messages.
type AssistantSessionRepository interface {
	// WithTx возвращает копию репозитория, привязанную к переданной транзакции.
	WithTx(tx *gorm.DB) AssistantSessionRepository

	// --- sessions ---

	// CreateSession создаёт пустую сессию (status='active', busy=false).
	CreateSession(ctx context.Context, session *models.AssistantSession) error

	// GetSession возвращает сессию пользователя. Проверка ownership встроена:
	// чужая сессия → ErrAssistantSessionNotFound (никакого 403 во избежание enumeration).
	GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*models.AssistantSession, error)

	// ListSessionsByUser — список активных и архивных сессий пользователя.
	// Сортировка: last_message_at DESC NULLS LAST (используется idx_assistant_sessions_user).
	ListSessionsByUser(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, includeArchived bool, limit int) ([]*models.AssistantSession, error)

	// UpdateSessionTitle меняет title (для авто-генерации после первого ответа модели).
	UpdateSessionTitle(ctx context.Context, sessionID, userID uuid.UUID, title string) error

	// ArchiveSession переводит сессию в status='archived' (soft-delete).
	// На занятой (busy=TRUE) сессии возвращает ErrAssistantSessionBusy — архивировать
	// можно только покой; иначе агент-петля будет писать в архив.
	ArchiveSession(ctx context.Context, sessionID, userID uuid.UUID) error

	// --- messages ---

	// AppendMessage добавляет сообщение и обновляет last_message_at в одной транзакции.
	// Уникальный конфликт по (session_id, client_message_id) маппится в
	// ErrAssistantMessageDuplicate — service использует это для идемпотентного 202.
	AppendMessage(ctx context.Context, msg *models.AssistantMessage) error

	// ListMessages — курсорная пагинация ORDER BY (created_at, id) DESC.
	// Пара (beforeCreatedAt, beforeID) формирует фильтр row-comparison
	// `WHERE (created_at, id) < (?, ?)`. Нулевые значения курсора → последняя страница.
	// Обоснование вторичного ключа — план §2 «нестабильный порядок».
	ListMessages(ctx context.Context, sessionID uuid.UUID, limit int, beforeCreatedAt time.Time, beforeID uuid.UUID) ([]*models.AssistantMessage, error)

	// FindMessageByClientID — лукап для идемпотентности user-сообщений (post-conflict
	// retrieve существующего message_id, чтобы вернуть фронту тот же ID).
	FindMessageByClientID(ctx context.Context, sessionID uuid.UUID, clientID string) (*models.AssistantMessage, error)

	// --- busy lifecycle (см. §3.1) ---

	// AcquireBusy — атомарный CAS: SET busy=TRUE, busy_since=NOW() WHERE busy=FALSE.
	// Также проверяет ownership и status='active' — один UPDATE закрывает все три кейса.
	// RowsAffected==0 → ErrAssistantSessionBusy (для занятой) или ErrAssistantSessionNotFound
	// (для чужой/архивной) — отличить можно повторным GetSession в service-слое.
	AcquireBusy(ctx context.Context, sessionID, userID uuid.UUID) error

	// ReleaseBusy сбрасывает busy=FALSE, busy_since=NULL, pending_tool_call_id=NULL.
	// Вызывается из `defer` в agent-loop горутине — должен быть идемпотентен, поэтому
	// RowsAffected==0 (например, повторный defer после panic-recover) НЕ возвращает ошибку.
	//
	// КОНТРАКТ ВЫЗОВА: после успешного ParkOnConfirm агент-петля ДОЛЖНА ЗАВЕРШИТЬСЯ
	// БЕЗ ВЫЗОВА ReleaseBusy (план §3.1: «при destructive-confirm флаг остаётся TRUE
	// до прихода ConfirmToolCall»). Сервисный слой обязан отменять defer (`released
	// := false`-флажок) при ветке park, иначе ReleaseBusy затрёт pending_tool_call_id
	// и confirm-resume не найдёт парк-состояние.
	ReleaseBusy(ctx context.Context, sessionID uuid.UUID) error

	// ParkOnConfirm «паркует» петлю на ожидании confirm: оставляет busy=TRUE, выставляет
	// pending_tool_call_id, чтобы между ParkOnConfirm и ConfirmAndClosePending не вклинилось
	// новое SendMessage (см. §3.1, «destructive-confirm: флаг остаётся TRUE»).
	ParkOnConfirm(ctx context.Context, sessionID uuid.UUID, toolCallID string) error

	// ResetStaleBusy — фоновый cron (см. §3.1 «Stale-recovery»). Снимает busy у сессий
	// старше staleThreshold БЕЗ pending_tool_call_id. Парк на confirm не трогаем —
	// он ждёт человека, а не процесс. Возвращает количество восстановленных строк.
	ResetStaleBusy(ctx context.Context, staleThreshold time.Duration) (int64, error)

	// --- confirm flow (см. §4.1) ---

	// GetPendingToolMessage — лукап pending tool-row по (session_id, tool_call_id)
	// для confirm-flow. Service'у нужен tool_name + tool_arguments, чтобы
	// исполнить подтверждённый MCP-вызов перед записью результата.
	// ErrMessageNotFound — нет pending row (или уже закрыт другим confirm).
	GetPendingToolMessage(ctx context.Context, sessionID uuid.UUID, toolCallID string) (*models.AssistantMessage, error)

	// ConfirmAndClosePending атомарно:
	//   1) SELECT ... FOR UPDATE по (session_id, user_id, busy=TRUE, pending_tool_call_id=toolCallID);
	//   2) UPDATE assistant_messages SET tool_result=?::jsonb WHERE tool_call_id=? AND tool_result IS NULL;
	//   3) UPDATE assistant_sessions SET pending_tool_call_id=NULL.
	// resultJSON — уже сериализованный []byte (см. план §4.1: map/struct напрямую в
	// `tx.Exec` для jsonb-колонки не работает в database/sql). Возможные ошибки:
	//   ErrAssistantNoPendingConfirmation — pending row не найден / mismatch;
	//   ErrAssistantAlreadyConfirmed     — параллельный confirm уже закрыл tool-row.
	// busy=TRUE остаётся — снимает его ReleaseBusy из defer'а runAgentLoopResume.
	ConfirmAndClosePending(ctx context.Context, sessionID, userID uuid.UUID, toolCallID string, resultJSON []byte) error
}

type assistantSessionRepository struct {
	db *gorm.DB
}

// NewAssistantSessionRepository создаёт репозиторий.
func NewAssistantSessionRepository(db *gorm.DB) AssistantSessionRepository {
	return &assistantSessionRepository{db: db}
}

func (r *assistantSessionRepository) WithTx(tx *gorm.DB) AssistantSessionRepository {
	return &assistantSessionRepository{db: tx}
}

// --- sessions ---

func (r *assistantSessionRepository) CreateSession(ctx context.Context, session *models.AssistantSession) error {
	if session == nil {
		return ErrInvalidInput
	}
	if session.UserID == uuid.Nil {
		return ErrInvalidInput
	}
	if session.Status == "" {
		session.Status = models.AssistantSessionStatusActive
	}
	if !session.Status.IsValid() {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create assistant session: %w", err)
	}
	return nil
}

func (r *assistantSessionRepository) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*models.AssistantSession, error) {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	var session models.AssistantSession
	err := db.WithContext(ctx).
		Where("id = ? AND user_id = ?", sessionID, userID).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssistantSessionNotFound
		}
		return nil, fmt.Errorf("get assistant session: %w", err)
	}
	return &session, nil
}

func (r *assistantSessionRepository) ListSessionsByUser(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, includeArchived bool, limit int) ([]*models.AssistantSession, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db).WithContext(ctx).Clauses(dbresolver.Write).
		Model(&models.AssistantSession{}).
		Where("user_id = ?", userID)
	if projectID != nil {
		db = db.Where("project_id = ?", projectID)
	} else {
		db = db.Where("project_id IS NULL")
	}
	if !includeArchived {
		db = db.Where("status = ?", models.AssistantSessionStatusActive)
	}

	// last_message_at NULLS LAST — у только что созданной сессии нет сообщений.
	limit = normalizeLimit(limit, assistantSessionsDefaultLimit, assistantSessionsMaxLimit)

	var sessions []*models.AssistantSession
	if err := db.
		Order("last_message_at DESC NULLS LAST, created_at DESC").
		Limit(limit).
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list assistant sessions: %w", err)
	}
	return sessions, nil
}

func (r *assistantSessionRepository) UpdateSessionTitle(ctx context.Context, sessionID, userID uuid.UUID, title string) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	res := db.WithContext(ctx).Model(&models.AssistantSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Updates(map[string]any{"title": title, "updated_at": gorm.Expr("NOW()")})
	if res.Error != nil {
		return fmt.Errorf("update assistant session title: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrAssistantSessionNotFound
	}
	return nil
}

func (r *assistantSessionRepository) ArchiveSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	// Дифференцируем «занята» / «не нашли» одним запросом с FOR UPDATE,
	// чтобы выдать корректный код наверх (409 vs 404). На «archived → archived»
	// возвращаем ok (идемпотентный delete).
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sess models.AssistantSession
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", sessionID, userID).
			First(&sess).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAssistantSessionNotFound
			}
			return fmt.Errorf("lock assistant session: %w", err)
		}
		if sess.Status == models.AssistantSessionStatusArchived {
			return nil
		}
		if sess.Busy {
			return ErrAssistantSessionBusy
		}
		res := tx.Model(&models.AssistantSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]any{
				"status":     models.AssistantSessionStatusArchived,
				"updated_at": gorm.Expr("NOW()"),
			})
		if res.Error != nil {
			return fmt.Errorf("archive assistant session: %w", res.Error)
		}
		return nil
	})
}

// --- messages ---

func (r *assistantSessionRepository) AppendMessage(ctx context.Context, msg *models.AssistantMessage) error {
	if msg == nil {
		return ErrInvalidInput
	}
	if msg.SessionID == uuid.Nil {
		return ErrInvalidInput
	}
	if !msg.Role.IsValid() {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			// Различаем UNIQUE-конфликты по имени констрейнта, не по «случайно
			// заполненному ClientMessageID» — иначе баг сервиса (одновременно
			// клиентский ID и дубликат tool_call_id) маскируется неверной ошибкой.
			// Имена индексов — из миграции 045.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				switch {
				case strings.Contains(pgErr.ConstraintName, "client_id"):
					return ErrAssistantMessageDuplicate
				case strings.Contains(pgErr.ConstraintName, "tool_call"):
					// Дубликат pending/closed tool-row на тот же tool_call_id —
					// баг сервиса (двойной AppendMessage на одну confirm-петлю).
					// Возвращаем «already confirmed» как ближайший по смыслу.
					return ErrAssistantAlreadyConfirmed
				}
			}
			return fmt.Errorf("append assistant message: %w", err)
		}

		// last_message_at двигаем только на user/assistant — tool-rows внутренние,
		// их появление не должно поднимать сессию в списке пользователя.
		// NOW() вместо msg.CreatedAt: msg.CreatedAt может быть zero (column DEFAULT
		// заполняется в БД через RETURNING, и не всегда успевает попасть в struct
		// до этого UPDATE), а нам важно atomic-консистентное время на сессии и
		// сообщении — оба идут одним txn, оба видят один NOW() транзакции.
		if msg.Role == models.AssistantMessageRoleUser || msg.Role == models.AssistantMessageRoleAssistant {
			res := tx.Model(&models.AssistantSession{}).
				Where("id = ?", msg.SessionID).
				Updates(map[string]any{
					"last_message_at": gorm.Expr("NOW()"),
					"updated_at":      gorm.Expr("NOW()"),
				})
			if res.Error != nil {
				return fmt.Errorf("touch last_message_at: %w", res.Error)
			}
		}
		return nil
	})
}

func (r *assistantSessionRepository) ListMessages(
	ctx context.Context,
	sessionID uuid.UUID,
	limit int,
	beforeCreatedAt time.Time,
	beforeID uuid.UUID,
) ([]*models.AssistantMessage, error) {
	if sessionID == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db).WithContext(ctx).
		Model(&models.AssistantMessage{}).
		Where("session_id = ?", sessionID)

	// Курсорная пагинация: row-comparison `(created_at, id) < (?, ?)`.
	// Нулевая пара — «с конца истории» (первая страница).
	if !beforeCreatedAt.IsZero() && beforeID != uuid.Nil {
		db = db.Where("(created_at, id) < (?, ?)", beforeCreatedAt, beforeID)
	}

	limit = normalizeLimit(limit, assistantMessagesDefaultLimit, assistantMessagesMaxLimit)

	var messages []*models.AssistantMessage
	if err := db.
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list assistant messages: %w", err)
	}
	return messages, nil
}

func (r *assistantSessionRepository) FindMessageByClientID(ctx context.Context, sessionID uuid.UUID, clientID string) (*models.AssistantMessage, error) {
	if sessionID == uuid.Nil || clientID == "" {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	var msg models.AssistantMessage
	err := db.WithContext(ctx).
		Where("session_id = ? AND client_message_id = ?", sessionID, clientID).
		First(&msg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, fmt.Errorf("find assistant message by client_id: %w", err)
	}
	return &msg, nil
}

// --- busy lifecycle ---

func (r *assistantSessionRepository) AcquireBusy(ctx context.Context, sessionID, userID uuid.UUID) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	// CAS в одной SQL-statement — никаких read-then-write окон.
	// busy_since задаём через NOW() в SET — это удовлетворяет CHECK-констрейнту
	// chk_assistant_sessions_busy_consistency (см. миграцию 045).
	res := db.WithContext(ctx).Exec(`
		UPDATE assistant_sessions
		   SET busy = TRUE,
		       busy_since = NOW(),
		       updated_at = NOW()
		 WHERE id = ?
		   AND user_id = ?
		   AND status = 'active'
		   AND busy = FALSE`,
		sessionID, userID)
	if res.Error != nil {
		return fmt.Errorf("acquire busy: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// Может быть либо «уже занята», либо «нет/архивная/чужая».
		// Различение — service-слой через GetSession (бизнес-логика).
		// Здесь репо возвращает «busy», т.к. это самый частый кейс при гонке
		// SendMessage'ов; ErrAssistantSessionNotFound разрулится в service.
		return ErrAssistantSessionBusy
	}
	return nil
}

func (r *assistantSessionRepository) ReleaseBusy(ctx context.Context, sessionID uuid.UUID) error {
	if sessionID == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	// Идемпотентно: повторный defer (например, после ParkOnConfirm + резюм) не должен
	// возвращать ошибку. CHECK chk_assistant_sessions_busy_consistency требует пары
	// (busy=FALSE, busy_since=NULL) — сбрасываем оба плюс pending_tool_call_id.
	if err := db.WithContext(ctx).Exec(`
		UPDATE assistant_sessions
		   SET busy = FALSE,
		       busy_since = NULL,
		       pending_tool_call_id = NULL,
		       updated_at = NOW()
		 WHERE id = ? AND busy = TRUE`,
		sessionID).Error; err != nil {
		return fmt.Errorf("release busy: %w", err)
	}
	return nil
}

func (r *assistantSessionRepository) ParkOnConfirm(ctx context.Context, sessionID uuid.UUID, toolCallID string) error {
	if sessionID == uuid.Nil || toolCallID == "" {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	// Паркуем только из состояния (busy=TRUE, pending=NULL) — двойной park
	// на одну сессию недопустим: вторая destructive операция в рамках одной петли
	// должна сперва пройти через ConfirmAndClosePending первой.
	res := db.WithContext(ctx).Exec(`
		UPDATE assistant_sessions
		   SET pending_tool_call_id = ?,
		       updated_at = NOW()
		 WHERE id = ?
		   AND busy = TRUE
		   AND pending_tool_call_id IS NULL`,
		toolCallID, sessionID)
	if res.Error != nil {
		return fmt.Errorf("park on confirm: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrAssistantSessionNotBusy
	}
	return nil
}

func (r *assistantSessionRepository) ResetStaleBusy(ctx context.Context, staleThreshold time.Duration) (int64, error) {
	if staleThreshold <= 0 {
		return 0, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	// `make_interval(secs => ?)` принимает float секунд — это даёт точность без
	// необходимости форматировать INTERVAL-строку (избегаем SQL-инъекции
	// через текстовый конкат). pending_tool_call_id IS NULL — петля висит на
	// сетевом IO, а не ждёт человека: confirm-парк трогать нельзя.
	res := db.WithContext(ctx).Exec(`
		UPDATE assistant_sessions
		   SET busy = FALSE,
		       busy_since = NULL,
		       updated_at = NOW()
		 WHERE busy = TRUE
		   AND pending_tool_call_id IS NULL
		   AND busy_since < NOW() - make_interval(secs => ?)`,
		staleThreshold.Seconds())
	if res.Error != nil {
		return 0, fmt.Errorf("reset stale busy: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *assistantSessionRepository) GetPendingToolMessage(ctx context.Context, sessionID uuid.UUID, toolCallID string) (*models.AssistantMessage, error) {
	if sessionID == uuid.Nil || toolCallID == "" {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	var msg models.AssistantMessage
	err := db.WithContext(ctx).
		Where("session_id = ? AND tool_call_id = ? AND role = ? AND tool_result IS NULL",
			sessionID, toolCallID, models.AssistantMessageRoleTool).
		First(&msg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, fmt.Errorf("get pending tool message: %w", err)
	}
	return &msg, nil
}

// --- confirm flow ---

func (r *assistantSessionRepository) ConfirmAndClosePending(
	ctx context.Context,
	sessionID, userID uuid.UUID,
	toolCallID string,
	resultJSON []byte,
) error {
	if sessionID == uuid.Nil || userID == uuid.Nil || toolCallID == "" {
		return ErrInvalidInput
	}
	if len(resultJSON) == 0 {
		// jsonb-колонка NOT NULL не объявлена, но writing «null»::jsonb ломает
		// семантику «tool_result IS NULL» как «pending». Пустой payload — баг service'а.
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Лок сессии под условие (owner+busy+pending). Raw().Scan возвращает
		// nil err + RowsAffected==0 на пустом результате — проверяем оба явно,
		// иначе нулевая sess пройдёт дальше как валидная (план §4.1).
		var sess models.AssistantSession
		res := tx.Raw(`
			SELECT * FROM assistant_sessions
			 WHERE id = ?
			   AND user_id = ?
			   AND status = 'active'
			   AND busy = TRUE
			   AND pending_tool_call_id = ?
			 FOR UPDATE`,
			sessionID, userID, toolCallID).Scan(&sess)
		if res.Error != nil {
			return fmt.Errorf("lock pending session: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrAssistantNoPendingConfirmation
		}

		// 2) Закрытие tool-row: WHERE tool_result IS NULL → ровно один параллельный
		// confirm побеждает. resultJSON уже []byte (service сериализовал через
		// json.Marshal); ::jsonb-каст явный, чтобы планировщик не ругался на
		// implicit text→jsonb. role='tool' дублирует фильтр partial UNIQUE из миграции.
		upd := tx.Exec(`
			UPDATE assistant_messages
			   SET tool_result = ?::jsonb,
			       updated_at = NOW()
			 WHERE tool_call_id = ?
			   AND role = 'tool'
			   AND tool_result IS NULL`,
			resultJSON, toolCallID)
		if upd.Error != nil {
			return fmt.Errorf("close pending tool result: %w", upd.Error)
		}
		if upd.RowsAffected == 0 {
			return ErrAssistantAlreadyConfirmed
		}

		// 3) Снимаем pending_tool_call_id. busy=TRUE остаётся —
		// resumed agent-loop снимет его в defer (план §3.1, §4.1).
		if err := tx.Exec(`
			UPDATE assistant_sessions
			   SET pending_tool_call_id = NULL,
			       updated_at = NOW()
			 WHERE id = ?`,
			sessionID).Error; err != nil {
			return fmt.Errorf("clear pending_tool_call_id: %w", err)
		}
		return nil
	})
}
