package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/gitprovider"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// --- mocks ---

type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Create(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *MockProjectRepository) List(ctx context.Context, filter repository.ProjectFilter) ([]models.Project, int64, error) {
	args := m.Called(ctx, filter)
	var projects []models.Project
	if v := args.Get(0); v != nil {
		projects = v.([]models.Project)
	}
	return projects, args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectRepository) ListByUserID(ctx context.Context, userID uuid.UUID, filter repository.ProjectFilter) ([]models.Project, int64, error) {
	args := m.Called(ctx, userID, filter)
	var projects []models.Project
	if v := args.Get(0); v != nil {
		projects = v.([]models.Project)
	}
	return projects, args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectRepository) Update(ctx context.Context, project *models.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockTeamRepository struct {
	mock.Mock
}

func (m *MockTeamRepository) Create(ctx context.Context, team *models.Team) error {
	args := m.Called(ctx, team)
	return args.Error(0)
}

func (m *MockTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamRepository) Update(ctx context.Context, team *models.Team) error {
	args := m.Called(ctx, team)
	return args.Error(0)
}

func (m *MockTeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockGitCredentialRepository struct {
	mock.Mock
}

func (m *MockGitCredentialRepository) Create(ctx context.Context, cred *models.GitCredential) error {
	args := m.Called(ctx, cred)
	return args.Error(0)
}

func (m *MockGitCredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GitCredential, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GitCredential), args.Error(1)
}

func (m *MockGitCredentialRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.GitCredential, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GitCredential), args.Error(1)
}

func (m *MockGitCredentialRepository) ListByUserIDAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitCredentialProvider) ([]models.GitCredential, error) {
	args := m.Called(ctx, userID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GitCredential), args.Error(1)
}

func (m *MockGitCredentialRepository) Update(ctx context.Context, cred *models.GitCredential) error {
	args := m.Called(ctx, cred)
	return args.Error(0)
}

func (m *MockGitCredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockFactory — мок gitprovider.Factory.
type MockFactory struct {
	mock.Mock
}

func (m *MockFactory) Create(providerType string, creds gitprovider.Credentials) (gitprovider.GitProvider, error) {
	args := m.Called(providerType, creds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(gitprovider.GitProvider), args.Error(1)
}

// MockGitProvider встраивает GitProvider; явно реализованы только методы, нужные тестам.
type MockGitProvider struct {
	gitprovider.GitProvider
	mock.Mock
}

func (m *MockGitProvider) ValidateAccess(ctx context.Context, repoURL string) error {
	return m.Called(ctx, repoURL).Error(0)
}

func (m *MockGitProvider) GetRepoInfo(ctx context.Context, repoURL string) (*gitprovider.RepoInfo, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*gitprovider.RepoInfo), args.Error(1)
}

func (m *MockGitProvider) Clone(ctx context.Context, repoURL string, opts gitprovider.CloneOptions) error {
	return m.Called(ctx, repoURL, opts).Error(0)
}

func (m *MockGitProvider) ProviderType() string {
	return m.Called().String(0)
}

func (m *MockGitProvider) SupportsPullRequests() bool {
	return m.Called().Bool(0)
}

// MockDecryptor — мок Decryptor.
type MockDecryptor struct {
	mock.Mock
}

func (m *MockDecryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	args := m.Called(ciphertext)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).([]byte), nil
}

var _ gitprovider.GitProvider = (*MockGitProvider)(nil)

// stubTxManager кладёт в ctx одну и ту же «транзакцию», как TransactionManager.
type stubTxManager struct{ tx *gorm.DB }

func (s *stubTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(repository.WithGormTx(ctx, s.tx))
}

type noopTxManager struct{}

func (noopTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestProjectService(
	pr *MockProjectRepository,
	tr *MockTeamRepository,
	gr *MockGitCredentialRepository,
	txMgr repository.TransactionManager,
) ProjectService {
	return newProjectServiceWithGitDeps(pr, tr, gr, txMgr, new(MockFactory), NoopDecryptor{}, "")
}

func newProjectServiceWithGitDeps(
	pr *MockProjectRepository,
	tr *MockTeamRepository,
	gr *MockGitCredentialRepository,
	txMgr repository.TransactionManager,
	gf *MockFactory,
	dec Decryptor,
	importDir string,
) ProjectService {
	if dec == nil {
		dec = NoopDecryptor{}
	}
	if gf == nil {
		gf = new(MockFactory)
	}
	return &projectService{
		projectRepo:  pr,
		teamRepo:     tr,
		gitCredRepo:  gr,
		transactions: txMgr,
		gitFactory:   gf,
		decryptor:    dec,
		importDir:    importDir,
	}
}

func assignProjectIDOnCreate(args mock.Arguments) {
	p := args.Get(1).(*models.Project)
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
}

func ctxHasGormTx(ctx context.Context) bool {
	_, ok := repository.GormTxFromContext(ctx)
	return ok
}

func TestProjectService_Create_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gr := new(MockGitCredentialRepository)
	svc := newTestProjectService(pr, tr, gr, &stubTxManager{tx: sharedTx})

	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.MatchedBy(func(team *models.Team) bool {
		return team.Name == devTeamDefaultName && team.Type == models.TeamTypeDevelopment
	})).Return(nil)

	out, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "  My App  "})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "My App", out.Name)
	assert.Equal(t, userID, out.UserID)
	assert.Equal(t, models.GitProviderLocal, out.GitProvider)
	assert.Equal(t, models.ProjectStatusActive, out.Status)
	assert.Equal(t, "main", out.GitDefaultBranch)
	pr.AssertExpectations(t)
	tr.AssertExpectations(t)
}

func TestProjectService_Create_WithGitCredential(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	credID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gr := new(MockGitCredentialRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, tr, gr, &stubTxManager{tx: sharedTx}, gf, NoopDecryptor{}, "")

	gr.On("GetByID", ctx, credID).Return(&models.GitCredential{
		ID: credID, UserID: userID, AuthType: models.GitCredentialAuthToken, EncryptedValue: []byte("tok"),
	}, nil)
	gf.On("Create", "github", mock.MatchedBy(func(c gitprovider.Credentials) bool { return c.Token == "tok" })).Return(mp, nil)
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name:            "P",
		GitProvider:     "github",
		GitCredentialID: &credID,
	})
	require.NoError(t, err)
	gr.AssertExpectations(t)
	gf.AssertExpectations(t)
	mp.AssertNotCalled(t, "ValidateAccess", mock.Anything, mock.Anything)
}

func TestProjectService_Create_GitCredentialForbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	otherUser := uuid.New()
	credID := uuid.New()
	gr := new(MockGitCredentialRepository)
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	svc := newTestProjectService(pr, tr, gr, noopTxManager{})

	gr.On("GetByID", ctx, credID).Return(&models.GitCredential{ID: credID, UserID: otherUser}, nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name:            "P",
		GitProvider:     "github",
		GitCredentialID: &credID,
	})
	require.ErrorIs(t, err, ErrGitCredentialForbidden)
	pr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	tr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestProjectService_Create_GitCredentialNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	credID := uuid.New()
	gr := new(MockGitCredentialRepository)
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), gr, noopTxManager{})

	gr.On("GetByID", ctx, credID).Return(nil, repository.ErrGitCredentialNotFound)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name:            "P",
		GitProvider:     "github",
		GitCredentialID: &credID,
	})
	require.ErrorIs(t, err, ErrGitCredentialNotFound)
	pr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestProjectService_Create_DuplicateName(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	svc := newTestProjectService(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx})

	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Return(repository.ErrProjectNameExists)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "Dup"})
	require.ErrorIs(t, err, ErrProjectNameExists)
	tr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestProjectService_Create_TeamAutoCreated(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	svc := newTestProjectService(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx})

	var projectID uuid.UUID
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(func(args mock.Arguments) {
			assignProjectIDOnCreate(args)
			projectID = args.Get(1).(*models.Project).ID
		}).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.MatchedBy(func(team *models.Team) bool {
		return team.Name == devTeamDefaultName &&
			team.Type == models.TeamTypeDevelopment &&
			team.ProjectID == projectID
	})).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "X"})
	require.NoError(t, err)
	tr.AssertExpectations(t)
}

func TestProjectService_GetByID_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{ID: pid, UserID: userID, Name: "A"}, nil)

	p, err := svc.GetByID(ctx, userID, models.RoleUser, pid)
	require.NoError(t, err)
	assert.Equal(t, "A", p.Name)
}

func TestProjectService_GetByID_Forbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{ID: pid, UserID: other, Name: "A"}, nil)

	_, err := svc.GetByID(ctx, userID, models.RoleUser, pid)
	require.ErrorIs(t, err, ErrProjectForbidden)
}

func TestProjectService_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(nil, repository.ErrProjectNotFound)

	_, err := svc.GetByID(ctx, uuid.New(), models.RoleUser, pid)
	require.ErrorIs(t, err, ErrProjectNotFound)
}

func TestProjectService_List_DefaultPagination(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("ListByUserID", ctx, userID, mock.MatchedBy(func(f repository.ProjectFilter) bool {
		return f.Limit == 20 && f.Offset == 0
	})).Return([]models.Project{}, int64(0), nil)

	_, _, err := svc.List(ctx, userID, models.RoleUser, dto.ListProjectsRequest{})
	require.NoError(t, err)
}

func TestProjectService_List_MaxLimit(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("ListByUserID", ctx, userID, mock.MatchedBy(func(f repository.ProjectFilter) bool {
		return f.Limit == 100
	})).Return([]models.Project{}, int64(0), nil)

	_, _, err := svc.List(ctx, userID, models.RoleUser, dto.ListProjectsRequest{Limit: 500})
	require.NoError(t, err)
}

func TestProjectService_List_FiltersByUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("ListByUserID", ctx, userID, mock.Anything).Return([]models.Project{}, int64(0), nil)

	_, _, err := svc.List(ctx, userID, models.RoleUser, dto.ListProjectsRequest{Limit: 10})
	require.NoError(t, err)
	pr.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
}

func TestProjectService_List_AdminUsesList(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("List", ctx, mock.Anything).Return([]models.Project{}, int64(0), nil)

	_, _, err := svc.List(ctx, userID, models.RoleAdmin, dto.ListProjectsRequest{})
	require.NoError(t, err)
	pr.AssertNotCalled(t, "ListByUserID", mock.Anything, mock.Anything, mock.Anything)
}

func TestProjectService_Update_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	desc := "new desc"
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	existing := &models.Project{
		ID:               pid,
		UserID:           userID,
		Name:             "Old",
		Description:      "old",
		GitProvider:      models.GitProviderLocal,
		GitDefaultBranch: "main",
		Status:           models.ProjectStatusActive,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.Name == "Old" && p.Description == desc
	})).Return(nil)

	out, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, desc, out.Description)
}

func TestProjectService_Update_Forbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	x := "x"
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{ID: pid, UserID: other}, nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{Name: &x})
	require.ErrorIs(t, err, ErrProjectForbidden)
	pr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProjectService_Update_ChangeCredential(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	newCred := uuid.New()
	pr := new(MockProjectRepository)
	gr := new(MockGitCredentialRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), gr, noopTxManager{}, gf, NoopDecryptor{}, "")

	existing := &models.Project{
		ID:               pid,
		UserID:           userID,
		Name:             "P",
		GitDefaultBranch: "main",
		GitProvider:      models.GitProviderGitHub,
		GitURL:           "https://github.com/o/r",
		Status:           models.ProjectStatusActive,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	gr.On("GetByID", ctx, newCred).Return(&models.GitCredential{ID: newCred, UserID: userID}, nil)
	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, "https://github.com/o/r").Return(nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.GitCredentialsID != nil && *p.GitCredentialsID == newCred
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitCredentialID: &newCred})
	require.NoError(t, err)
}

func TestProjectService_Update_RemoveGitCredential(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	cred := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
		GitCredentialsID: &cred,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.GitCredentialsID == nil
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{RemoveGitCredential: true})
	require.NoError(t, err)
}

func TestProjectService_Update_RemoveGitCredentialConflict(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	c := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
	}, nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{
		RemoveGitCredential: true,
		GitCredentialID:     &c,
	})
	require.ErrorIs(t, err, ErrUpdateProjectGitCredentialConflict)
	pr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProjectService_Update_ClearTechStack(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
		TechStack: datatypes.JSON([]byte(`{"x":1}`)),
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return string(p.TechStack) == "{}"
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{ClearTechStack: true})
	require.NoError(t, err)
}

func TestProjectService_Update_ClearSettings(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
		Settings: datatypes.JSON([]byte(`{"a":1}`)),
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return string(p.Settings) == "{}"
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{ClearSettings: true})
	require.NoError(t, err)
}

func TestProjectService_Update_ClearTechStackConflict(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	ts := datatypes.JSON([]byte(`{}`))
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
	}, nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{
		ClearTechStack: true,
		TechStack:      &ts,
	})
	require.ErrorIs(t, err, ErrUpdateProjectTechStackConflict)
}

func TestProjectService_Delete_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{ID: pid, UserID: userID}, nil)
	pr.On("Delete", ctx, pid).Return(nil)

	err := svc.Delete(ctx, userID, models.RoleUser, pid)
	require.NoError(t, err)
}

func TestProjectService_Delete_Forbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{ID: pid, UserID: other}, nil)

	err := svc.Delete(ctx, userID, models.RoleUser, pid)
	require.ErrorIs(t, err, ErrProjectForbidden)
	pr.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestProjectService_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(nil, repository.ErrProjectNotFound)

	err := svc.Delete(ctx, uuid.New(), models.RoleUser, pid)
	require.ErrorIs(t, err, ErrProjectNotFound)
}

func TestProjectService_Create_SameTxForProjectAndTeam(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	svc := newTestProjectService(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx})

	var captured *gorm.DB
	pr.On("Create", mock.MatchedBy(func(c context.Context) bool {
		tx, ok := repository.GormTxFromContext(c)
		if !ok {
			return false
		}
		captured = tx
		return true
	}), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(func(c context.Context) bool {
		tx, ok := repository.GormTxFromContext(c)
		return ok && tx == captured
	}), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "T"})
	require.NoError(t, err)
}

func TestProjectService_Update_ChangeCredentialForbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	other := uuid.New()
	pid := uuid.New()
	newCred := uuid.New()
	pr := new(MockProjectRepository)
	gr := new(MockGitCredentialRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), gr, noopTxManager{})

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: "https://github.com/o/r",
		Status: models.ProjectStatusActive,
	}, nil)
	gr.On("GetByID", ctx, newCred).Return(&models.GitCredential{ID: newCred, UserID: other}, nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitCredentialID: &newCred})
	require.ErrorIs(t, err, ErrGitCredentialForbidden)
	pr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProjectService_Update_TechStackPartial(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
		TechStack: datatypes.JSON([]byte(`{}`)),
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	newStack := datatypes.JSON([]byte(`{"go":true}`))
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return string(p.TechStack) == `{"go":true}`
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{TechStack: &newStack})
	require.NoError(t, err)
}

func TestCreate_LocalProvider_NoGitOps(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "L", GitProvider: "local"})
	require.NoError(t, err)
	gf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_GitHub_ValidateAccess_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/r"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(nil, errors.New("skip"))
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	out, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "G", GitProvider: "github", GitURL: url})
	require.NoError(t, err)
	assert.Equal(t, url, out.GitURL)
	mp.AssertCalled(t, "ValidateAccess", ctx, url)
}

func TestCreate_GitHub_ValidateAccess_AuthFailed(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/priv"
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(new(MockProjectRepository), new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(gitprovider.ErrAuthFailed)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "X", GitProvider: "github", GitURL: url})
	require.ErrorIs(t, err, ErrGitValidationFailed)
}

func TestCreate_GitHub_ValidateAccess_RepoNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/missing"
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(new(MockProjectRepository), new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(gitprovider.ErrRepoNotFound)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "X", GitProvider: "github", GitURL: url})
	require.ErrorIs(t, err, ErrGitValidationFailed)
}

func TestCreate_GitHub_GetRepoInfo_AutoBranch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/r"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(&gitprovider.RepoInfo{DefaultBranch: "master"}, nil)
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.MatchedBy(func(p *models.Project) bool {
		return p.GitDefaultBranch == "master"
	})).Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	out, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "B", GitProvider: "github", GitURL: url})
	require.NoError(t, err)
	assert.Equal(t, "master", out.GitDefaultBranch)
}

func TestCreate_GitHub_GetRepoInfo_Error_FallbackMain(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/r"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(nil, errors.New("api down"))
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.MatchedBy(func(p *models.Project) bool {
		return p.GitDefaultBranch == "main"
	})).Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	out, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "B", GitProvider: "github", GitURL: url})
	require.NoError(t, err)
	assert.Equal(t, "main", out.GitDefaultBranch)
}

func TestCreate_GitHub_WithCredential_Decrypted(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	credID := uuid.New()
	url := "https://github.com/o/r"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gr := new(MockGitCredentialRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	dec := new(MockDecryptor)
	svc := newProjectServiceWithGitDeps(pr, tr, gr, &stubTxManager{tx: sharedTx}, gf, dec, "")

	enc := []byte("secret")
	gr.On("GetByID", ctx, credID).Once().Return(&models.GitCredential{
		ID: credID, UserID: userID, AuthType: models.GitCredentialAuthToken, EncryptedValue: enc,
	}, nil)
	dec.On("Decrypt", enc).Once().Return([]byte("tok"), nil)
	gf.On("Create", "github", mock.MatchedBy(func(c gitprovider.Credentials) bool { return c.Token == "tok" })).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(nil, errors.New("skip"))
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name: "N", GitProvider: "github", GitURL: url, GitCredentialID: &credID,
	})
	require.NoError(t, err)
	gr.AssertExpectations(t)
	dec.AssertExpectations(t)
}

func TestCreate_GitHub_WithCredential_DecryptError(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	credID := uuid.New()
	url := "https://github.com/o/r"
	gr := new(MockGitCredentialRepository)
	gf := new(MockFactory)
	dec := new(MockDecryptor)
	svc := newProjectServiceWithGitDeps(new(MockProjectRepository), new(MockTeamRepository), gr, noopTxManager{}, gf, dec, "")

	enc := []byte("x")
	gr.On("GetByID", ctx, credID).Return(&models.GitCredential{
		ID: credID, UserID: userID, AuthType: models.GitCredentialAuthToken, EncryptedValue: enc,
	}, nil)
	dec.On("Decrypt", enc).Return(nil, errors.New("bad"))

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name: "N", GitProvider: "github", GitURL: url, GitCredentialID: &credID,
	})
	require.ErrorIs(t, err, ErrDecryptionFailed)
	gf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_GitHub_NoCredential_PublicRepo(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/pub"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	gf.On("Create", "github", mock.MatchedBy(func(c gitprovider.Credentials) bool {
		return c.Token == "" && c.SSHKey == ""
	})).Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(nil, errors.New("skip"))
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "P", GitProvider: "github", GitURL: url})
	require.NoError(t, err)
}

func TestCreate_GitHub_NoGitURL_NoGitOps(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gf := new(MockFactory)
	svc := newProjectServiceWithGitDeps(pr, tr, new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx}, gf, nil, "")

	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "N", GitProvider: "github"})
	require.NoError(t, err)
	gf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_GitHub_FactoryError(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	url := "https://github.com/o/r"
	gf := new(MockFactory)
	svc := newProjectServiceWithGitDeps(new(MockProjectRepository), new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, "")

	gf.On("Create", "github", mock.Anything).Return(nil, errors.New("boom"))

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{Name: "N", GitProvider: "github", GitURL: url})
	require.Error(t, err)
}

func TestCreate_NoBuildProviderCalledTwice(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	credID := uuid.New()
	url := "https://github.com/o/r"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	tr := new(MockTeamRepository)
	gr := new(MockGitCredentialRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	dec := new(MockDecryptor)
	importDir := t.TempDir()
	svc := newProjectServiceWithGitDeps(pr, tr, gr, &stubTxManager{tx: sharedTx}, gf, dec, importDir)

	enc := []byte("e")
	gr.On("GetByID", ctx, credID).Once().Return(&models.GitCredential{
		ID: credID, UserID: userID, AuthType: models.GitCredentialAuthToken, EncryptedValue: enc,
	}, nil)
	dec.On("Decrypt", enc).Once().Return([]byte("t"), nil)
	gf.On("Create", "github", mock.Anything).Once().Return(mp, nil)
	mp.On("ValidateAccess", ctx, url).Return(nil)
	mp.On("GetRepoInfo", ctx, url).Return(nil, errors.New("skip"))
	done := make(chan struct{})
	mp.On("Clone", mock.Anything, url, mock.Anything).Run(func(mock.Arguments) { close(done) }).Return(nil)
	pr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Project")).
		Run(assignProjectIDOnCreate).Return(nil)
	tr.On("Create", mock.MatchedBy(ctxHasGormTx), mock.AnythingOfType("*models.Team")).Return(nil)

	_, err := svc.Create(ctx, userID, dto.CreateProjectRequest{
		Name: "N", GitProvider: "github", GitURL: url, GitCredentialID: &credID,
	})
	require.NoError(t, err)
	<-done
	gr.AssertExpectations(t)
	dec.AssertExpectations(t)
	gf.AssertExpectations(t)
}

func TestCreate_LocalProvider_WithCredential_Error(t *testing.T) {
	ctx := context.Background()
	credID := uuid.New()
	svc := newTestProjectService(new(MockProjectRepository), new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{})

	_, err := svc.Create(ctx, uuid.New(), dto.CreateProjectRequest{
		Name: "L", GitProvider: "local", GitCredentialID: &credID,
	})
	require.ErrorIs(t, err, ErrGitCredentialNotSupportedForLocal)
}

func TestUpdate_GitURLChanged_Revalidated(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	oldU := "https://github.com/o/old"
	newU := "https://github.com/o/new"
	pr := new(MockProjectRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	importDir := t.TempDir()
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, importDir)

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: oldU, Status: models.ProjectStatusActive,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, newU).Return(nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.GitURL == newU
	})).Return(nil)
	cloneDone := make(chan struct{})
	mp.On("Clone", mock.Anything, newU, mock.Anything).Run(func(mock.Arguments) { close(cloneDone) }).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitURL: &newU})
	require.NoError(t, err)
	mp.AssertCalled(t, "ValidateAccess", ctx, newU)
	<-cloneDone
}

func TestUpdate_GitURLChanged_OldCloneRemoved(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	oldU := "https://github.com/o/old"
	newU := "https://github.com/o/new"
	importDir := t.TempDir()
	workDir := filepath.Join(importDir, pid.String())
	require.NoError(t, os.MkdirAll(workDir, 0o755))
	marker := filepath.Join(workDir, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("x"), 0o644))

	pr := new(MockProjectRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, importDir)

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: oldU, Status: models.ProjectStatusActive,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, newU).Return(nil)
	pr.On("Update", ctx, mock.Anything).Return(nil)
	blockClone := make(chan struct{})
	mp.On("Clone", mock.Anything, newU, mock.Anything).Run(func(mock.Arguments) { <-blockClone }).Return(nil).Maybe()

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitURL: &newU})
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	require.True(t, os.IsNotExist(statErr), "old clone dir should be removed before async clone recreates it")
	close(blockClone)
}

func TestUpdate_NoGitChange_NoRevalidation(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	n := "Renamed"
	pr := new(MockProjectRepository)
	gf := new(MockFactory)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, "")

	existing := &models.Project{
		ID: pid, UserID: userID, Name: "Old", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: "https://github.com/o/r", Status: models.ProjectStatusActive,
	}
	pr.On("GetByID", ctx, pid).Return(existing, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool { return p.Name == n })).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{Name: &n})
	require.NoError(t, err)
	gf.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestUpdate_RemoveGitCredential_RevalidatesAccess(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	credID := uuid.New()
	u := "https://github.com/o/private"
	pr := new(MockProjectRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, "")

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: u,
		GitCredentialsID: &credID, Status: models.ProjectStatusActive,
	}, nil)
	gf.On("Create", "github", mock.MatchedBy(func(c gitprovider.Credentials) bool {
		return c.Token == "" && c.SSHKey == ""
	})).Return(mp, nil)
	mp.On("ValidateAccess", ctx, u).Return(gitprovider.ErrAuthFailed)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{RemoveGitCredential: true})
	require.ErrorIs(t, err, ErrGitValidationFailed)
	pr.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdate_RemoteToLocal_RemovesImportWorkdir(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	u := "https://github.com/o/repo"
	local := "local"
	importDir := t.TempDir()
	workDir := filepath.Join(importDir, pid.String())
	require.NoError(t, os.MkdirAll(workDir, 0o755))
	marker := filepath.Join(workDir, "old-clone")
	require.NoError(t, os.WriteFile(marker, []byte("x"), 0o644))

	pr := new(MockProjectRepository)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, new(MockFactory), nil, importDir)

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderGitHub, GitURL: u, Status: models.ProjectStatusActive,
	}, nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.GitProvider == models.GitProviderLocal
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitProvider: &local})
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	require.True(t, os.IsNotExist(statErr))
}

func TestUpdate_LocalToRemote_TriggersCloneWithoutURLChange(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	u := "https://github.com/o/repo"
	gp := "github"
	importDir := t.TempDir()
	pr := new(MockProjectRepository)
	gf := new(MockFactory)
	mp := new(MockGitProvider)
	svc := newProjectServiceWithGitDeps(pr, new(MockTeamRepository), new(MockGitCredentialRepository), noopTxManager{}, gf, nil, importDir)

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "P", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, GitURL: u, Status: models.ProjectStatusActive,
	}, nil)
	gf.On("Create", "github", mock.Anything).Return(mp, nil)
	mp.On("ValidateAccess", ctx, u).Return(nil)
	pr.On("Update", ctx, mock.MatchedBy(func(p *models.Project) bool {
		return p.GitProvider == models.GitProviderGitHub && p.GitURL == u
	})).Return(nil)
	done := make(chan struct{})
	mp.On("Clone", mock.Anything, u, mock.Anything).Run(func(mock.Arguments) { close(done) }).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{GitProvider: &gp})
	require.NoError(t, err)
	<-done
}

func TestUpdate_UsesDbTransaction(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	pid := uuid.New()
	desc := "new"
	sharedTx := &gorm.DB{}
	pr := new(MockProjectRepository)
	svc := newTestProjectService(pr, new(MockTeamRepository), new(MockGitCredentialRepository), &stubTxManager{tx: sharedTx})

	pr.On("GetByID", ctx, pid).Return(&models.Project{
		ID: pid, UserID: userID, Name: "O", GitDefaultBranch: "main",
		GitProvider: models.GitProviderLocal, Status: models.ProjectStatusActive,
	}, nil)
	pr.On("Update", mock.MatchedBy(ctxHasGormTx), mock.MatchedBy(func(p *models.Project) bool {
		return p.Description == desc
	})).Return(nil)

	_, err := svc.Update(ctx, userID, models.RoleUser, pid, dto.UpdateProjectRequest{Description: &desc})
	require.NoError(t, err)
	pr.AssertExpectations(t)
}
