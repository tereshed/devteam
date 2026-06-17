package dto

import "github.com/devteam/backend/internal/models"

// UpsertSandboxServiceRequest — создание/обновление декларации сервис-сайдкара
// проекта (PUT, upsert по alias). Пустые поля заполняются дефолтами в сервисе.
// Пароль НЕ принимается и НЕ хранится — генерится случайно на каждый прогон.
type UpsertSandboxServiceRequest struct {
	IsEnabled bool `json:"is_enabled"`
	// Kind — тип сервиса; пусто → postgres.
	Kind string `json:"kind"`
	// Alias — сетевой alias/hostname в bridge-сети прогона (агент: alias:port). Обязателен.
	Alias string `json:"alias" binding:"required"`
	// Image — docker-образ; пусто → дефолт (postgres:16-alpine).
	Image string `json:"image"`
	// DBName / DBUser — имя БД и суперюзера сервис-контейнера.
	DBName string `json:"db_name"`
	DBUser string `json:"db_user"`
	// Port — порт сервиса; 0 → 5432.
	Port int `json:"port"`
	// SeedKind — none | repo_file | inline; пусто → none.
	SeedKind  string `json:"seed_kind"`
	SeedValue string `json:"seed_value"`
	// ReadyTimeoutSeconds — потолок ожидания готовности; 0 → 60.
	ReadyTimeoutSeconds int `json:"ready_timeout_seconds"`
}

// SandboxServiceConfigResponse — декларация сервис-сайдкара проекта.
type SandboxServiceConfigResponse struct {
	ID                  string `json:"id"`
	ProjectID           string `json:"project_id"`
	IsEnabled           bool   `json:"is_enabled"`
	Kind                string `json:"kind"`
	Alias               string `json:"alias"`
	Image               string `json:"image"`
	DBName              string `json:"db_name"`
	DBUser              string `json:"db_user"`
	Port                int    `json:"port"`
	SeedKind            string `json:"seed_kind"`
	SeedValue           string `json:"seed_value"`
	ReadyTimeoutSeconds int    `json:"ready_timeout_seconds"`
}

// SandboxServiceListResponse — список сервис-сайдкаров проекта.
type SandboxServiceListResponse struct {
	Services []SandboxServiceConfigResponse `json:"services"`
	Total    int                            `json:"total"`
}

// ToSandboxServiceConfigResponse маппит модель в DTO.
func ToSandboxServiceConfigResponse(cfg *models.SandboxServiceConfig) SandboxServiceConfigResponse {
	if cfg == nil {
		return SandboxServiceConfigResponse{}
	}
	return SandboxServiceConfigResponse{
		ID:                  cfg.ID.String(),
		ProjectID:           cfg.ProjectID.String(),
		IsEnabled:           cfg.IsEnabled,
		Kind:                string(cfg.Kind),
		Alias:               cfg.Alias,
		Image:               cfg.Image,
		DBName:              cfg.DBName,
		DBUser:              cfg.DBUser,
		Port:                cfg.Port,
		SeedKind:            string(cfg.SeedKind),
		SeedValue:           cfg.SeedValue,
		ReadyTimeoutSeconds: cfg.ReadyTimeoutSeconds,
	}
}

// ToSandboxServiceListResponse оборачивает список деклараций.
func ToSandboxServiceListResponse(items []models.SandboxServiceConfig) SandboxServiceListResponse {
	out := make([]SandboxServiceConfigResponse, 0, len(items))
	for i := range items {
		out = append(out, ToSandboxServiceConfigResponse(&items[i]))
	}
	return SandboxServiceListResponse{Services: out, Total: len(out)}
}
