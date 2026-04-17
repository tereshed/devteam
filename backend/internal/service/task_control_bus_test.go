package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestUserTaskControlBus_PublishCommand(t *testing.T) {
	b := NewUserTaskControlBus()
	var got UserTaskControlCommand
	var wg sync.WaitGroup
	wg.Add(1)
	b.SubscribeCommands(func(_ context.Context, c UserTaskControlCommand) {
		got = c
		wg.Done()
	})
	tid := uuid.New()
	uid := uuid.New()
	b.PublishCommand(context.Background(), UserTaskControlCommand{
		Kind:     UserTaskControlPause,
		TaskID:   tid,
		UserID:   uid,
		UserRole: models.RoleAdmin,
	})
	wg.Wait()
	require.Equal(t, UserTaskControlPause, got.Kind)
	require.Equal(t, tid, got.TaskID)
	require.Equal(t, uid, got.UserID)
}
