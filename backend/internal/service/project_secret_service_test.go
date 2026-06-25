package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProjectSecretRepo — in-memory реализация repository.ProjectSecretRepository.
type fakeProjectSecretRepo struct {
	byID map[uuid.UUID]*models.ProjectSecret
}

func newFakeProjectSecretRepo() *fakeProjectSecretRepo {
	return &fakeProjectSecretRepo{byID: map[uuid.UUID]*models.ProjectSecret{}}
}

func (r *fakeProjectSecretRepo) Create(_ context.Context, s *models.ProjectSecret) error {
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

func (r *fakeProjectSecretRepo) Update(_ context.Context, s *models.ProjectSecret) error {
	if _, ok := r.byID[s.ID]; !ok {
		return repository.ErrProjectSecretNotFound
	}
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

func (r *fakeProjectSecretRepo) GetByName(_ context.Context, projectID uuid.UUID, keyName string) (*models.ProjectSecret, error) {
	for _, s := range r.byID {
		if s.ProjectID == projectID && s.KeyName == keyName {
			cp := *s
			return &cp, nil
		}
	}
	return nil, repository.ErrProjectSecretNotFound
}

func (r *fakeProjectSecretRepo) ListByProjectID(_ context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error) {
	var out []models.ProjectSecret
	for _, s := range r.byID {
		if s.ProjectID == projectID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *fakeProjectSecretRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return repository.ErrProjectSecretNotFound
	}
	delete(r.byID, id)
	return nil
}

func (r *fakeProjectSecretRepo) GetAllDecrypted(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error) {
	return r.ListByProjectID(ctx, projectID)
}

func newTestProjectSecretService(t *testing.T) *ProjectSecretService {
	t.Helper()
	enc, err := crypto.NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	return NewProjectSecretService(newFakeProjectSecretRepo(), NewSecretService(enc), slog.Default())
}

func TestProjectSecret_Set_RejectsReservedKey(t *testing.T) {
	svc := newTestProjectSecretService(t)
	_, err := svc.Set(context.Background(), SetProjectSecretInput{
		ProjectID: uuid.New(), KeyName: "GIT_TOKEN", Value: "x", InjectAsEnv: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrProjectSecretReservedKey))
}

func TestProjectSecret_Set_RoundTripsFlagAndDescription(t *testing.T) {
	svc := newTestProjectSecretService(t)
	pid := uuid.New()

	out, err := svc.Set(context.Background(), SetProjectSecretInput{
		ProjectID: pid, KeyName: "DATABASE_URL", Value: "postgres://x",
		InjectAsEnv: true, Description: "тестовая БД",
	})
	require.NoError(t, err)
	assert.True(t, out.InjectAsEnv)
	assert.Equal(t, "тестовая БД", out.Description)

	// Обновление перезаписывает флаг.
	out2, err := svc.Set(context.Background(), SetProjectSecretInput{
		ProjectID: pid, KeyName: "DATABASE_URL", Value: "postgres://y", InjectAsEnv: false,
	})
	require.NoError(t, err)
	assert.Equal(t, out.SecretID, out2.SecretID, "обновление того же ключа сохраняет id")
	assert.False(t, out2.InjectAsEnv)
}

func TestProjectSecret_GetInjectableEnv_FiltersByFlag(t *testing.T) {
	svc := newTestProjectSecretService(t)
	pid := uuid.New()
	ctx := context.Background()

	_, err := svc.Set(ctx, SetProjectSecretInput{ProjectID: pid, KeyName: "DATABASE_URL", Value: "v1", InjectAsEnv: true})
	require.NoError(t, err)
	_, err = svc.Set(ctx, SetProjectSecretInput{ProjectID: pid, KeyName: "MCP_ONLY_SECRET", Value: "v2", InjectAsEnv: false})
	// MCP_ONLY_SECRET зарезервирован (префикс MCP_) — ожидаем ошибку; берём другой ключ.
	require.Error(t, err)
	_, err = svc.Set(ctx, SetProjectSecretInput{ProjectID: pid, KeyName: "INTERNAL_ONLY", Value: "v2", InjectAsEnv: false})
	require.NoError(t, err)

	env, err := svc.GetInjectableEnv(ctx, pid)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"DATABASE_URL": "v1"}, env, "только inject_as_env=true попадает в env")

	adv, err := svc.ListAdvertised(ctx, pid)
	require.NoError(t, err)
	require.Len(t, adv, 1)
	assert.Equal(t, "DATABASE_URL", adv[0].KeyName)
}

func TestBuildProjectEnvPromptBlock(t *testing.T) {
	// Пусто → пустая строка (блок не добавляется).
	assert.Equal(t, "", buildProjectEnvPromptBlock(nil))

	block := buildProjectEnvPromptBlock([]AdvertisedProjectVar{
		{KeyName: "DATABASE_URL", Description: "строка подключения"},
		{KeyName: "OPENAI_API_KEY", Description: ""},
	})
	assert.Contains(t, block, "ДОСТУПНЫЕ ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ")
	assert.Contains(t, block, "- DATABASE_URL — строка подключения")
	assert.Contains(t, block, "- OPENAI_API_KEY")
	// Без описания — не должно быть висящего тире.
	assert.NotContains(t, block, "OPENAI_API_KEY — ")
}

// fakeProjectEnvReader — реализация ProjectEnvSecretReader для теста инъекции в билдере.
type fakeProjectEnvReader struct {
	env map[string]string
	adv []AdvertisedProjectVar
}

func (f fakeProjectEnvReader) GetInjectableEnv(context.Context, uuid.UUID) (map[string]string, error) {
	return f.env, nil
}
func (f fakeProjectEnvReader) ListAdvertised(context.Context, uuid.UUID) ([]AdvertisedProjectVar, error) {
	return f.adv, nil
}

func TestContextBuilder_InjectProjectEnv(t *testing.T) {
	b := &contextBuilder{projectSecrets: fakeProjectEnvReader{
		env: map[string]string{"DATABASE_URL": "postgres://x"},
		adv: []AdvertisedProjectVar{{KeyName: "DATABASE_URL", Description: "БД"}},
	}}
	input := &agent.ExecutionInput{PromptSystem: "base prompt"}
	project := &models.Project{ID: uuid.New()}

	b.injectProjectEnv(context.Background(), input, project)

	assert.Equal(t, map[string]string{"DATABASE_URL": "postgres://x"}, input.ProjectEnv)
	assert.Contains(t, input.PromptSystem, "base prompt")
	assert.Contains(t, input.PromptSystem, "ДОСТУПНЫЕ ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ")
	assert.Contains(t, input.PromptSystem, "DATABASE_URL")
}

func TestContextBuilder_InjectProjectEnv_NilReaderNoop(t *testing.T) {
	b := &contextBuilder{} // projectSecrets == nil
	input := &agent.ExecutionInput{PromptSystem: "base"}
	b.injectProjectEnv(context.Background(), input, &models.Project{ID: uuid.New()})
	assert.Nil(t, input.ProjectEnv)
	assert.Equal(t, "base", input.PromptSystem)
}
