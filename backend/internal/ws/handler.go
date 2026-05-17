package ws

import (
	"context"
	"net/http"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log/slog"
	"time"
)

// getUserID/getUserRole — локальные копии middleware-helper'ов, чтобы избежать
// import-cycle: ws → middleware → service → ws. Поля контекста идентичны
// тем, что выставляет AuthMiddleware (см. middleware/auth_middleware.go).
func getUserID(c *gin.Context) (uuid.UUID, bool) {
	v, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		panic("userID in context is not of type uuid.UUID")
	}
	return id, true
}

func getUserRole(c *gin.Context) (string, bool) {
	v, exists := c.Get("userRole")
	if !exists {
		return "", false
	}
	r, ok := v.(string)
	if !ok {
		panic("userRole in context is not of type string")
	}
	return r, true
}

// ProjectAccessor — узкий интерфейс для проверки доступа к проекту перед
// upgrade в WS. Введён, чтобы избежать import cycle: пакет `service` теперь
// импортирует `ws` для типизированных Marshal-помощников assistant-событий
// (Sprint 21 §7), поэтому `ws` сам зависеть от `service` не может.
//
// Адаптер живёт в cmd/api/main.go — он мостит service.ProjectService.HasAccess
// в этот интерфейс и транслирует доменные ошибки `ErrProjectNotFound` /
// `ErrProjectForbidden` в (allowed=false, denied=true).
type ProjectAccessor interface {
	// HasAccess возвращает:
	//   allowed=true             — пользователь допущен.
	//   allowed=false, denied=true — доступ запрещён по ABAC (либо проект не существует).
	//                                Handler ответит 403 без раскрытия причины.
	//   allowed=false, err!=nil  — внутренняя ошибка (БД и т.п.). Handler ответит 500.
	HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (allowed, denied bool, err error)
}

// WebSocketHandler — HTTP-обработчик upgrade в WS + регистрация в Hub.
// ВСЯ проверка прав доступа к project_id происходит здесь (Hub их не валидирует).
type WebSocketHandler struct {
	hub            *Hub
	projectAccess  ProjectAccessor
	upgrader       websocket.Upgrader
	cfg            HandlerConfig
	log            *slog.Logger
}

// HandlerConfig — настройки handler'а (allowed origins, лимиты, таймауты).
type HandlerConfig struct {
	AllowedOrigins         []string
	MaxConnsPerUserProject int
	ReadBufferSize         int
	WriteBufferSize        int
}

func NewWebSocketHandler(hub *Hub, access ProjectAccessor, cfg HandlerConfig, log *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub:           hub,
		projectAccess: access,
		cfg:           cfg,
		log:           log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.ReadBufferSize,
			WriteBufferSize: cfg.WriteBufferSize,
			// CheckOrigin (КРИТИЧНО — CSWSH):
			// Разрешаем пустой Origin (не-браузерные клиенты)
			// Для не-пустого Origin проверяем по whitelist.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, allowed := range cfg.AllowedOrigins {
					if strings.EqualFold(origin, allowed) {
						return true
					}
				}
				return false
			},
			// Subprotocols НЕ задаём здесь, т.к. Gorilla делает строгое сравнение.
			// Мы будем эхать subprotocol вручную в методе Connect.
			Subprotocols: nil,
		},
	}
}

// Connect — обработчик GET /api/v1/projects/:id/ws (см. 7.7).
//
// @Summary      Подключение к WebSocket стриму проекта
// @Description  Стримит события task_status, task_message, agent_log, error для проекта в реальном времени
// @Tags         websocket
// @Security     BearerAuth
// @Param        id   path  string  true  "Project ID (UUID)"
// @Success      101
// @Failure      400  {object}  apierror.ErrorResponse
// @Failure      401  {object}  apierror.ErrorResponse
// @Failure      403  {object}  apierror.ErrorResponse
// @Failure      429  {object}  apierror.ErrorResponse
// @Router       /projects/{id}/ws [get]
func (h *WebSocketHandler) Connect(c *gin.Context) {
	// 1. Аутентификация: userID и role уже в контексте благодаря AuthMiddleware
	userID, ok := getUserID(c)
	if !ok {
		apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenRequired, "User ID not found in context")
		return
	}

	userRoleStr, ok := getUserRole(c)
	if !ok {
		apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenRequired, "User role not found in context")
		return
	}
	userRole := models.UserRole(userRoleStr)

	// 2. Валидация project_id
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		apierror.AbortJSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}

	// 3. Авторизация (доступ к project_id)
	// КРИТИЧНО — IDOR: Hub не знает о пользователях.
	// Используем запросный контекст c.Request.Context(), он валиден ДО Upgrade().
	allowed, denied, err := h.projectAccess.HasAccess(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		h.log.Error("failed to check project access", "err", err, "userID", userID, "projectID", projectID)
		apierror.AbortJSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Internal server error")
		return
	}
	if !allowed {
		// denied=true → проект не найден или ABAC запрещает. В обоих случаях 403,
		// чтобы не утечь факт существования.
		_ = denied
		apierror.AbortJSON(c, http.StatusForbidden, apierror.ErrForbidden, "Access to project denied")
		return
	}

	// 4. Предварительная проверка лимита подключений (DoS protection)
	// Чтобы не делать Upgrade() зря.
	if count := h.hub.CountUserConnections(userID.String(), projectID.String()); count >= h.cfg.MaxConnsPerUserProject {
		apierror.AbortJSON(c, http.StatusTooManyRequests, apierror.ErrTooManyRequests, "Connection limit exceeded")
		return
	}

	// 5. Подготовка к Upgrade: эхо Sec-WebSocket-Protocol
	// Находим bearer.<jwt> в запросе, чтобы эхнуть его в ответе.
	var negotiated string
	for _, p := range websocket.Subprotocols(c.Request) {
		if strings.HasPrefix(p, "bearer.") {
			negotiated = p
			break
		}
	}

	respHdr := http.Header{}
	if negotiated != "" {
		respHdr.Set("Sec-WebSocket-Protocol", negotiated)
	}

	// 6. Upgrade HTTP → WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, respHdr)
	if err != nil {
		// Upgrader уже написал ответ клиенту (400/403).
		// После Upgrade() вызывать apierror.AbortJSON ЗАПРЕЩЕНО.
		h.log.Warn("ws upgrade failed", "err", err, "remote", c.ClientIP())
		return
	}

	// 7. Регистрация в Hub и запуск pump'ов
	// КРИТИЧНО: запросный контекст c.Request.Context() ПОСЛЕ Upgrade() использовать ЗАПРЕЩЕНО.
	client := NewClient(uuid.NewString(), userID.String(), conn, h.hub)

	// Финальная атомарная проверка лимита и регистрация
	if ok := h.hub.RegisterIfUnderLimit(client, []string{projectID.String()}, h.cfg.MaxConnsPerUserProject); !ok {
		// Лимит превышен (race condition между CountUserConnections и Register)
		// Upgrade() УЖЕ случился → шлём close-фрейм.
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection limit exceeded"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		return
	}

	// Запуск горутин для чтения и записи
	go client.WritePump()
	go client.ReadPump()

	h.log.Info("ws client connected", "clientID", client.ID, "userID", userID, "projectID", projectID)
}
