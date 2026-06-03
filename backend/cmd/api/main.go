package main

import (
	"github.com/devteam/backend/docs"
	"github.com/devteam/backend/internal/app"
)

// Инициализируем документацию (для swagger). Аннотации @title и docs.SwaggerInfo держим
// здесь — это файл, на который указывает `swag init -g cmd/api/main.go`.
func init() {
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
}

// @title           Backend API
// @version         1.0
// @description     Backend API с авторизацией на JWT токенах
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Long-lived API key for programmatic access. Format: wibe_<key>

// @securityDefinitions.oauth2.password OAuth2Password
// @tokenUrl /api/v1/auth/login

// main — основной бинарь. Роль берётся из APP_ROLE (api|worker|scheduler|all); по умолчанию
// all — единый процесс (обратная совместимость с одноинстансным деплоем). Для разделения
// ролей либо выставьте APP_ROLE, либо используйте выделенные бинари cmd/worker, cmd/scheduler.
func main() {
	app.Run(app.RoleFromEnv())
}
