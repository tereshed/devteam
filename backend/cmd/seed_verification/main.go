package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/devteam/backend/pkg/password"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Warning: could not open %s: %v\n", path, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes if present
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func main() {
	// 0. Load .env file
	loadEnvFile(".env")

	// 1. Load config
	os.Setenv("CONFIG_DIR", ".")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. Fetch fresh access token using refresh token
	refreshToken := os.Getenv("ANTIGRAVITY_REFRESH_TOKEN")
	if refreshToken == "" {
		fmt.Println("Error: ANTIGRAVITY_REFRESH_TOKEN environment variable is not set")
		os.Exit(1)
	}

	clientID := cfg.AntigravityOAuth.ClientID
	if clientID == "" {
		clientID = os.Getenv("ANTIGRAVITY_OAUTH_CLIENT_ID")
	}
	clientSecret := cfg.AntigravityOAuth.ClientSecret
	if clientSecret == "" {
		clientSecret = os.Getenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		fmt.Println("Error: Antigravity OAuth credentials are not configured in environment or config file")
		os.Exit(1)
	}

	fmt.Println("Refreshing access token...")
	accessToken, expiresIn, err := refreshAccessToken(clientID, clientSecret, refreshToken)
	if err != nil {
		fmt.Printf("Failed to refresh access token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully refreshed! Access token length: %d, expires in: %d seconds\n", len(accessToken), expiresIn)

	// 3. Connect to database
	dsn := cfg.Database.DSN()
	// Override host to localhost for running from host machine if needed
	if strings.Contains(dsn, "host=yugabytedb") {
		dsn = strings.Replace(dsn, "host=yugabytedb", "host=localhost", 1)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	// 4. Initialize encryptor
	var encryptor service.Encryptor
	if len(cfg.Encryption.Key) == 32 {
		aesEnc, err := crypto.NewAESEncryptor(cfg.Encryption.Key)
		if err != nil {
			fmt.Printf("aes: %v\n", err)
			os.Exit(1)
		}
		encryptor = aesEnc
	} else {
		fmt.Printf("ENCRYPTION_KEY length is %d (expected 32 bytes hex)\n", len(cfg.Encryption.Key))
		os.Exit(1)
	}

	// 5. Seed user
	userID := uuid.MustParse("18fec79a-0d8d-4669-95d4-570be1157afd")
	passwordHash, err := password.Hash("userpassword")
	if err != nil {
		fmt.Printf("Hash password: %v\n", err)
		os.Exit(1)
	}
	user := models.User{
		ID:            userID,
		Email:         "semisopsitica@gmail.com",
		PasswordHash:  passwordHash,
		Role:          models.RoleUser,
		EmailVerified: true,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&user).Error; err != nil {
		fmt.Printf("Failed to create/get user: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("User seeded/verified: %s\n", user.ID)

	// 6. Encrypt and insert subscription
	accessAAD := []byte("antigravity_subscription:access:" + userID.String())
	refreshAAD := []byte("antigravity_subscription:refresh:" + userID.String())

	accessEnc, err := encryptor.Encrypt([]byte(accessToken), accessAAD)
	if err != nil {
		fmt.Printf("Encrypt access: %v\n", err)
		os.Exit(1)
	}
	refreshEnc, err := encryptor.Encrypt([]byte(refreshToken), refreshAAD)
	if err != nil {
		fmt.Printf("Encrypt refresh: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)
	sub := models.AntigravitySubscription{
		UserID:               userID,
		OAuthAccessTokenEnc:  accessEnc,
		OAuthRefreshTokenEnc: refreshEnc,
		TokenType:            "Bearer",
		Scopes:               "https://www.googleapis.com/auth/experimentsandconfigs https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/aicode https://www.googleapis.com/auth/cclog openid https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.profile",
		ExpiresAt:            &expiresAt,
		LastRefreshedAt:      &now,
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(&sub).Error; err != nil {
		fmt.Printf("Failed to create subscription: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Subscription seeded/updated: %s\n", sub.ID)

	// 6.5. Seed Git Credentials (PAT)
	githubPAT := os.Getenv("GITHUB_PAT")
	if githubPAT == "" {
		fmt.Println("Warning: GITHUB_PAT not found in env, git clone might fail!")
	} else {
		// Seed in git_credentials table
		db.Exec("DELETE FROM git_credentials WHERE user_id = ?", userID)

		credID := uuid.New()
		encryptedPAT, err := encryptor.Encrypt([]byte(githubPAT), []byte(credID.String()))
		if err != nil {
			fmt.Printf("Failed to encrypt GITHUB_PAT: %v\n", err)
			os.Exit(1)
		}

		gitCred := models.GitCredential{
			ID:             credID,
			UserID:         userID,
			Provider:       models.GitCredentialProviderGitHub,
			AuthType:       models.GitCredentialAuthToken,
			EncryptedValue: encryptedPAT,
			Label:          "Seeded GITHUB_PAT",
		}

		if err := db.Create(&gitCred).Error; err != nil {
			fmt.Printf("Failed to create git credential: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Git credential seeded in git_credentials: %s\n", gitCred.ID)

		// Seed in git_integration_credentials table
		db.Exec("DELETE FROM git_integration_credentials WHERE user_id = ?", userID)

		integrationCredID := uuid.New()
		integrationAAD := []byte("git_integration_credential:" + integrationCredID.String())
		encryptedPATInt, err := encryptor.Encrypt([]byte(githubPAT), integrationAAD)
		if err != nil {
			fmt.Printf("Failed to encrypt integration GITHUB_PAT: %v\n", err)
			os.Exit(1)
		}

		gitIntegCred := models.GitIntegrationCredential{
			ID:             integrationCredID,
			UserID:         userID,
			Provider:       models.GitIntegrationProviderGitHub,
			AccessTokenEnc: encryptedPATInt,
			TokenType:      "Bearer",
		}

		if err := db.Create(&gitIntegCred).Error; err != nil {
			fmt.Printf("Failed to create git integration credential: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Git integration credential seeded in git_integration_credentials: %s\n", gitIntegCred.ID)

		// Set this git credential ID for the project
		projectID := uuid.MustParse("782d39b9-6902-452c-9ace-29a27020f245")
		db.Exec("UPDATE projects SET git_credentials_id = ? WHERE id = ?", credID, projectID)
	}

	// 7. Seed project
	projectID := uuid.MustParse("782d39b9-6902-452c-9ace-29a27020f245")
	project := models.Project{
		ID:               projectID,
		Name:             "Devteam",
		Description:      "Devteam project for verification",
		GitProvider:      models.GitProviderGitHub,
		GitURL:           "https://github.com/tereshed/devteam",
		GitDefaultBranch: "main",
		Status:           models.ProjectStatusActive,
		UserID:           userID,
	}
	// Get GitCredentialsID from DB if we just inserted it
	var existingCred models.GitCredential
	if err := db.Where("user_id = ?", userID).First(&existingCred).Error; err == nil {
		project.GitCredentialsID = &existingCred.ID
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&project).Error; err != nil {
		fmt.Printf("Failed to seed project: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Project seeded/verified: %s (GitCredentialsID: %v)\n", project.ID, project.GitCredentialsID)

	// 8. Seed team
	teamID := uuid.MustParse("db47f502-fcff-431e-be37-8fd3429b95e2")
	team := models.Team{
		ID:        teamID,
		ProjectID: projectID,
		Name:      "Development Team",
		Type:      models.TeamTypeDevelopment,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&team).Error; err != nil {
		fmt.Printf("Failed to seed team: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Team seeded/verified: %s\n", team.ID)

	// 9. Seed team agents
	// Delete any existing team agents to avoid duplicate keys/names and get a clean slate
	if err := db.Exec("DELETE FROM agents WHERE team_id = ?", teamID).Error; err != nil {
		fmt.Printf("Failed to clean team agents: %v\n", err)
		os.Exit(1)
	}

	// Fetch defaults from agent_role_prompts
	var rolePrompts []models.AgentRolePrompt
	if err := db.Find(&rolePrompts).Error; err != nil {
		fmt.Printf("Failed to find agent_role_prompts: %v\n", err)
		os.Exit(1)
	}
	promptsByRole := make(map[string]models.AgentRolePrompt)
	for _, rp := range rolePrompts {
		promptsByRole[rp.Role] = rp
	}

	roles := []struct {
		name         string
		role         models.AgentRole
		kind         models.AgentExecutionKind
		providerKind *models.AgentProviderKind
		model        *string
		codeBackend  *models.CodeBackend
		temperature  *float64
		maxTokens    *int
		settings     string
		perms        string
		agentID      *uuid.UUID
	}{
		{
			name: "orchestrator",
			role: models.AgentRoleOrchestrator,
			kind: models.AgentExecutionKindLLM,
		},
		{
			name:         "router",
			role:         models.AgentRoleRouter,
			kind:         models.AgentExecutionKindLLM,
			providerKind: pHelper(models.AgentProviderKindOpenRouter),
			model:        sHelper("deepseek/deepseek-v4-flash"),
			temperature:  fHelper(0.2),
			maxTokens:    iHelper(4096),
		},
		{
			name:         "planner",
			role:         models.AgentRolePlanner,
			kind:         models.AgentExecutionKindLLM,
			providerKind: pHelper(models.AgentProviderKindOpenRouter),
			model:        sHelper("deepseek/deepseek-v4-flash"),
			temperature:  fHelper(0.3),
			maxTokens:    iHelper(8192),
		},
		{
			name:         "decomposer",
			role:         models.AgentRoleDecomposer,
			kind:         models.AgentExecutionKindLLM,
			providerKind: pHelper(models.AgentProviderKindOpenRouter),
			model:        sHelper("deepseek/deepseek-v4-flash"),
			temperature:  fHelper(0.3),
			maxTokens:    iHelper(8192),
		},
		{
			name:        "reviewer",
			role:        models.AgentRoleReviewer,
			kind:        models.AgentExecutionKindSandbox,
			codeBackend: cbHelper(models.CodeBackendClaudeCode),
			settings:    `{"permission_mode": "auto"}`,
			perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
		},
		{
			name:         "developer",
			role:         models.AgentRoleDeveloper,
			kind:         models.AgentExecutionKindSandbox,
			codeBackend:  cbHelper(models.CodeBackendAntigravity),
			providerKind: pHelper(models.AgentProviderKindAntigravityOAuth),
			settings:     `{"permission_mode": "auto"}`,
			perms:        `{"env_secret_keys": ["ANTIGRAVITY_OAUTH_TOKEN"]}`,
			agentID:      uuidHelper(uuid.MustParse("cc6ed01b-2128-4492-ac30-d2c2999b7c2f")),
		},
		{
			name:        "tester",
			role:        models.AgentRoleTester,
			kind:        models.AgentExecutionKindSandbox,
			codeBackend: cbHelper(models.CodeBackendClaudeCode),
			settings:    `{"permission_mode": "auto"}`,
			perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
		},
		{
			name:        "merger",
			role:        models.AgentRoleMerger,
			kind:        models.AgentExecutionKindSandbox,
			codeBackend: cbHelper(models.CodeBackendClaudeCode),
			settings:    `{"permission_mode": "auto"}`,
			perms:       `{"env_secret_keys": ["ANTHROPIC_API_KEY"]}`,
		},
	}

	for _, r := range roles {
		rp, ok := promptsByRole[string(r.role)]
		if !ok {
			fmt.Printf("No default role prompt for %s\n", r.role)
			os.Exit(1)
		}

		var promptContent *string
		if rp.Content != "" {
			promptContent = &rp.Content
		}
		var promptDesc *string
		if rp.Description != nil {
			promptDesc = rp.Description
		} else {
			desc := "Default " + r.name + " agent"
			promptDesc = &desc
		}

		id := uuid.New()
		if r.agentID != nil {
			id = *r.agentID
		}

		agent := models.Agent{
			ID:                  id,
			Name:                r.name,
			Role:                r.role,
			ExecutionKind:       r.kind,
			IsActive:            true,
			SystemPrompt:        promptContent,
			RoleDescription:     promptDesc,
			TeamID:              &teamID,
			ProviderKind:        r.providerKind,
			Model:               r.model,
			CodeBackend:         r.codeBackend,
			Temperature:         r.temperature,
			MaxTokens:           r.maxTokens,
			Skills:              datatypes.JSON([]byte(`[]`)),
			Settings:            datatypes.JSON([]byte(`{}`)),
			ModelConfig:         datatypes.JSON([]byte(`{}`)),
			CodeBackendSettings: datatypes.JSON([]byte(`{}`)),
			SandboxPermissions:  datatypes.JSON([]byte(`{}`)),
		}

		if r.kind == models.AgentExecutionKindSandbox {
			agent.RequiresCodeContext = true
		}
		if r.settings != "" {
			agent.CodeBackendSettings = datatypes.JSON([]byte(r.settings))
		}
		if r.perms != "" {
			agent.SandboxPermissions = datatypes.JSON([]byte(r.perms))
		}

		if err := db.Create(&agent).Error; err != nil {
			fmt.Printf("Failed to create agent %s: %v\n", r.name, err)
			os.Exit(1)
		}
		fmt.Printf("Agent %s seeded with ID: %s\n", r.name, agent.ID)
	}

	// 10. Seed task and enqueue step_req event
	taskID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	// Clean up any existing task
	db.Exec("DELETE FROM tasks WHERE id = ?", taskID)
	db.Exec("DELETE FROM task_events WHERE task_id = ?", taskID)

	task := models.Task{
		ID:            taskID,
		ProjectID:     projectID,
		TeamID:        &teamID,
		Title:         "Создать файл antigravity_test.txt",
		Description:   "Создать пустой файл antigravity_test.txt в корне репозитория, закоммитить и запушить изменения.",
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   userID,
		State:         models.TaskStateActive,
		Context:       datatypes.JSON([]byte(`{}`)),
		Artifacts:     datatypes.JSON([]byte(`{}`)),
	}

	if err := db.Create(&task).Error; err != nil {
		fmt.Printf("Failed to create task: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Task seeded: %s, State: %s\n", task.ID, task.State)

	// Create and enqueue step_req task event
	event := models.TaskEvent{
		TaskID:      taskID,
		Kind:        models.TaskEventKindStepReq,
		Payload:     datatypes.JSON([]byte(`{}`)),
		ScheduledAt: time.Now(),
		Attempts:    0,
		MaxAttempts: 3,
	}

	if err := db.Create(&event).Error; err != nil {
		fmt.Printf("Failed to enqueue step_req: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("TaskEvent step_req enqueued: %d\n", event.ID)
}

func sHelper(s string) *string { return &s }
func fHelper(f float64) *float64 { return &f }
func iHelper(i int) *int { return &i }
func pHelper(p models.AgentProviderKind) *models.AgentProviderKind { return &p }
func cbHelper(cb models.CodeBackend) *models.CodeBackend { return &cb }
func uuidHelper(id uuid.UUID) *uuid.UUID { return &id }

func refreshAccessToken(clientID, clientSecret, refreshToken string) (string, int, error) {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("bad status: %d body: %s", resp.StatusCode, string(body))
	}
	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", 0, err
	}
	return res.AccessToken, res.ExpiresIn, nil
}
