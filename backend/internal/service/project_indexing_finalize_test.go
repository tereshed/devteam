package service

import (
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// finalizeProjectIndexing обязан при полном успехе записывать SHA primary-репо в
// projects.last_indexed_commit: RunBackgroundReindexing сравнивает именно его, и без
// записи каждый 10-минутный тик заново «детектит изменения» и перезапускает индексацию.

func TestFinalizeProjectIndexing_WritesPrimarySHAOnSuccess(t *testing.T) {
	pr := new(MockProjectRepository)
	svc := &projectService{projectRepo: pr}
	projectID := uuid.New()

	pr.On("UpdateStatusAndCommit", mock.Anything, projectID,
		models.ProjectStatusIndexing, models.ProjectStatusReady, "sha-primary").Return(nil)

	svc.finalizeProjectIndexing(projectID, false, "sha-primary")

	pr.AssertExpectations(t)
	pr.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestFinalizeProjectIndexing_FailureKeepsLastIndexedCommit(t *testing.T) {
	pr := new(MockProjectRepository)
	svc := &projectService{projectRepo: pr}
	projectID := uuid.New()

	pr.On("UpdateStatus", mock.Anything, projectID,
		models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed).Return(nil)

	// SHA primary-репо известен, но при частичном фейле фиксировать его нельзя:
	// иначе change-detect посчитает проект проиндексированным и не перезапустит.
	svc.finalizeProjectIndexing(projectID, true, "sha-primary")

	pr.AssertExpectations(t)
	pr.AssertNotCalled(t, "UpdateStatusAndCommit",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestFinalizeProjectIndexing_NoPrimarySHAFallsBackToStatusOnly(t *testing.T) {
	pr := new(MockProjectRepository)
	svc := &projectService{projectRepo: pr}
	projectID := uuid.New()

	pr.On("UpdateStatus", mock.Anything, projectID,
		models.ProjectStatusIndexing, models.ProjectStatusReady).Return(nil)

	svc.finalizeProjectIndexing(projectID, false, "")

	pr.AssertExpectations(t)
	pr.AssertNotCalled(t, "UpdateStatusAndCommit",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
