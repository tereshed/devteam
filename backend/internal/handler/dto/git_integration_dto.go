package dto

import "time"

// GitIntegrationInitRequest — тело POST /integrations/{provider}/auth/init.
// Для GitHub и GitLab.com все BYO-поля игнорируются (могут отсутствовать).
// Для self-hosted GitLab (provider=gitlab) host/byo_client_id/byo_client_secret обязательны.
type GitIntegrationInitRequest struct {
	// RedirectURI — куда провайдер вернёт пользователя после consent. Должен совпадать
	// с client_secret-конфигом OAuth App у провайдера.
	RedirectURI string `json:"redirect_uri" binding:"required"`
	// Host — только для self-hosted GitLab. Пусто = shared.
	Host string `json:"host,omitempty"`
	// ByoClientID / ByoClientSecret — только для self-hosted GitLab.
	ByoClientID     string `json:"byo_client_id,omitempty"`
	ByoClientSecret string `json:"byo_client_secret,omitempty"`
}

// GitIntegrationInitResponse — ответ POST /integrations/{provider}/auth/init.
type GitIntegrationInitResponse struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

// GitIntegrationCallbackRequest — тело POST /integrations/{provider}/auth/callback.
// state и code приходят от провайдера через redirect_uri; в этом proxy-callback
// фронт передаёт их в JSON, чтобы избежать прямого webhook'а на бэкенд.
type GitIntegrationCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state" binding:"required"`
	// Error — если провайдер вернул ?error=... (например access_denied).
	Error string `json:"error,omitempty"`
}

// GitIntegrationStatusResponse — публичный статус подключения.
type GitIntegrationStatusResponse struct {
	Provider     string     `json:"provider"`
	Connected    bool       `json:"connected"`
	Host         string     `json:"host,omitempty"`
	AccountLogin string     `json:"account_login,omitempty"`
	Scopes       string     `json:"scopes,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	ConnectedAt  *time.Time `json:"connected_at,omitempty"`
}

// GitIntegrationCallbackResponse — ответ POST /integrations/{provider}/auth/callback.
type GitIntegrationCallbackResponse struct {
	Provider string                       `json:"provider"`
	Status   GitIntegrationStatusResponse `json:"status"`
}

// GitIntegrationRevokeResponse — ответ DELETE /integrations/{provider}.
type GitIntegrationRevokeResponse struct {
	Provider           string `json:"provider"`
	RemoteRevokeFailed bool   `json:"remote_revoke_failed,omitempty"`
}
