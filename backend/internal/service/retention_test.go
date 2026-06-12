package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStuckIndexingReleaser struct {
	released  int64
	err       error
	calls     int
	gotCutoff time.Time
}

func (f *fakeStuckIndexingReleaser) ReleaseStuckIndexing(_ context.Context, cutoff time.Time) (int64, error) {
	f.calls++
	f.gotCutoff = cutoff
	return f.released, f.err
}

func TestRunOnceStuckIndexing_ReleasesProjectsAndRepos(t *testing.T) {
	pr := &fakeStuckIndexingReleaser{released: 2}
	rr := &fakeStuckIndexingReleaser{released: 1}
	svc := NewRetentionService(nil, nil, nil, pr, rr, nil,
		RetentionConfig{IndexingStuckAge: 30 * time.Minute})

	n, err := svc.RunOnceStuckIndexing(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, 1, pr.calls)
	assert.Equal(t, 1, rr.calls)
	assert.WithinDuration(t, time.Now().Add(-30*time.Minute), pr.gotCutoff, 5*time.Second)
}

func TestRunOnceStuckIndexing_NilReleasersNoop(t *testing.T) {
	svc := NewRetentionService(nil, nil, nil, nil, nil, nil, RetentionConfig{})

	n, err := svc.RunOnceStuckIndexing(context.Background())

	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestRunOnceStuckIndexing_ErrorInProjectsDoesNotSkipRepos(t *testing.T) {
	pr := &fakeStuckIndexingReleaser{err: errors.New("boom")}
	rr := &fakeStuckIndexingReleaser{released: 1}
	svc := NewRetentionService(nil, nil, nil, pr, rr, nil, RetentionConfig{})

	n, err := svc.RunOnceStuckIndexing(context.Background())

	require.Error(t, err)
	assert.Equal(t, int64(1), n)
	assert.Equal(t, 1, rr.calls)
}
