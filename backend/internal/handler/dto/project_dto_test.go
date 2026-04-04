package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestToProjectResponse_AllFields(t *testing.T) {
	t.Parallel()

	credID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	projID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	p := &models.Project{
		ID:               projID,
		Name:             "acme",
		Description:      "desc",
		GitProvider:      models.GitProviderGitHub,
		GitURL:           "https://github.com/org/repo",
		GitDefaultBranch: "main",
		GitCredentialsID: &credID,
		GitCredential: &models.GitCredential{
			ID:             credID,
			Provider:       models.GitCredentialProviderGitHub,
			AuthType:       models.GitCredentialAuthToken,
			Label:          "default",
			EncryptedValue: []byte("secret"),
		},
		VectorCollection: "col-1",
		TechStack:        datatypes.JSON(`{"lang":"go"}`),
		Status:           models.ProjectStatusActive,
		Settings:         datatypes.JSON(`{"k":"v"}`),
		CreatedAt:        ts,
		UpdatedAt:        ts,
	}

	got := ToProjectResponse(p)

	assert.Equal(t, projID.String(), got.ID)
	assert.Equal(t, "acme", got.Name)
	assert.Equal(t, "desc", got.Description)
	assert.Equal(t, string(models.GitProviderGitHub), got.GitProvider)
	assert.Equal(t, "https://github.com/org/repo", got.GitURL)
	assert.Equal(t, "main", got.GitDefaultBranch)
	require.NotNil(t, got.GitCredential)
	assert.Equal(t, credID.String(), got.GitCredential.ID)
	assert.Equal(t, string(models.GitCredentialProviderGitHub), got.GitCredential.Provider)
	assert.Equal(t, string(models.GitCredentialAuthToken), got.GitCredential.AuthType)
	assert.Equal(t, "default", got.GitCredential.Label)
	assert.Equal(t, "col-1", got.VectorCollection)
	assert.JSONEq(t, `{"lang":"go"}`, string(got.TechStack))
	assert.Equal(t, string(models.ProjectStatusActive), got.Status)
	assert.JSONEq(t, `{"k":"v"}`, string(got.Settings))
	assert.True(t, got.CreatedAt.Equal(ts))
	assert.True(t, got.UpdatedAt.Equal(ts))
}

func TestToProjectResponse_NilGitCredential(t *testing.T) {
	t.Parallel()

	p := &models.Project{
		ID:               uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Name:             "solo",
		GitProvider:      models.GitProviderLocal,
		GitDefaultBranch: "main",
		Status:           models.ProjectStatusPaused,
	}
	resp := ToProjectResponse(p)
	assert.Nil(t, resp.GitCredential)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))
	_, hasCred := m["git_credential"]
	assert.False(t, hasCred, "при nil GitCredential поле не сериализуется (json omitempty)")
}

func TestToProjectResponse_WithGitCredential(t *testing.T) {
	t.Parallel()

	cid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	p := &models.Project{
		ID:               uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Name:             "p",
		GitProvider:      models.GitProviderLocal,
		GitDefaultBranch: "main",
		Status:           models.ProjectStatusActive,
		GitCredential: &models.GitCredential{
			ID:             cid,
			Provider:       models.GitCredentialProviderGitLab,
			AuthType:       models.GitCredentialAuthSSHKey,
			Label:          "ci",
			EncryptedValue: []byte("x"),
		},
	}
	resp := ToProjectResponse(p)
	require.NotNil(t, resp.GitCredential)
	assert.Equal(t, cid.String(), resp.GitCredential.ID)
	assert.Equal(t, "gitlab", resp.GitCredential.Provider)
	assert.Equal(t, "ssh_key", resp.GitCredential.AuthType)
	assert.Equal(t, "ci", resp.GitCredential.Label)
}

func TestToProjectListResponse(t *testing.T) {
	t.Parallel()

	projects := []models.Project{
		{
			ID:               uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			Name:             "a",
			GitProvider:      models.GitProviderLocal,
			GitDefaultBranch: "main",
			Status:           models.ProjectStatusActive,
		},
		{
			ID:               uuid.MustParse("77777777-7777-7777-7777-777777777777"),
			Name:             "b",
			GitProvider:      models.GitProviderLocal,
			GitDefaultBranch: "main",
			Status:           models.ProjectStatusArchived,
		},
	}
	got := ToProjectListResponse(projects, 42, 10, 5)
	assert.Len(t, got.Projects, 2)
	assert.Equal(t, int64(42), got.Total)
	assert.Equal(t, 10, got.Limit)
	assert.Equal(t, 5, got.Offset)
	assert.Equal(t, "a", got.Projects[0].Name)
	assert.Equal(t, "b", got.Projects[1].Name)
}

func TestToProjectListResponse_Empty(t *testing.T) {
	t.Parallel()

	got := ToProjectListResponse(nil, 0, 20, 0)
	assert.NotNil(t, got.Projects)
	assert.Len(t, got.Projects, 0)
	assert.Equal(t, int64(0), got.Total)
	assert.Equal(t, 20, got.Limit)
	assert.Equal(t, 0, got.Offset)
}

func TestToGitCredentialSummary(t *testing.T) {
	t.Parallel()

	gc := &models.GitCredential{
		ID:             uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		Provider:       models.GitCredentialProviderBitbucket,
		AuthType:       models.GitCredentialAuthOAuth,
		Label:          "oauth1",
		EncryptedValue: []byte("super-secret"),
	}
	s := ToGitCredentialSummary(gc)
	require.NotNil(t, s)
	assert.Equal(t, gc.ID.String(), s.ID)
	assert.Equal(t, "bitbucket", s.Provider)
	assert.Equal(t, "oauth", s.AuthType)
	assert.Equal(t, "oauth1", s.Label)

	raw, err := json.Marshal(s)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(raw), "encrypted"))
	assert.False(t, strings.Contains(string(raw), "super-secret"))
}

func TestToGitCredentialSummary_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ToGitCredentialSummary(nil))
}
