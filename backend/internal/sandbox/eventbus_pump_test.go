package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/goleak"
)

type mockLogPublisher struct {
	mock.Mock
}

func (m *mockLogPublisher) Publish(ctx context.Context, projectID, taskID uuid.UUID, sandboxID string, seq int64, entry LogEntry) error {
	args := m.Called(ctx, projectID, taskID, sandboxID, seq, entry)
	return args.Error(0)
}

func TestStreamLogsToBus_Lifecycle(t *testing.T) {
	defer goleak.VerifyNone(t)

	r := &DockerSandboxRunner{
		publisher: &mockLogPublisher{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopCh := make(chan struct{})
	logCh := make(chan LogEntry, 10)
	
	projectID := uuid.New()
	taskID := uuid.New()
	sandboxID := "test-sandbox"

	// Запускаем в фоне
	done := make(chan struct{})
	go func() {
		r.streamLogsToBus(ctx, stopCh, projectID, taskID, sandboxID, logCh)
		close(done)
	}()

	// Отменяем контекст и проверяем завершение
	cancel()
	
	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("streamLogsToBus did not exit on context cancel")
	}
}

func TestStreamLogsToBus_StopCh(t *testing.T) {
	defer goleak.VerifyNone(t)

	r := &DockerSandboxRunner{
		publisher: &mockLogPublisher{},
	}

	ctx := context.Background()
	stopCh := make(chan struct{})
	logCh := make(chan LogEntry, 10)
	
	projectID := uuid.New()
	taskID := uuid.New()
	sandboxID := "test-sandbox"

	done := make(chan struct{})
	go func() {
		r.streamLogsToBus(ctx, stopCh, projectID, taskID, sandboxID, logCh)
		close(done)
	}()

	// Закрываем stopCh и проверяем завершение
	close(stopCh)
	
	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("streamLogsToBus did not exit on stopCh close")
	}
}

func TestStreamLogsToBus_Publishing(t *testing.T) {
	publisher := &mockLogPublisher{}
	r := &DockerSandboxRunner{
		publisher: publisher,
	}

	ctx := context.Background()
	stopCh := make(chan struct{})
	logCh := make(chan LogEntry, 10)
	
	projectID := uuid.New()
	taskID := uuid.New()
	sandboxID := "test-sandbox"

	entry1 := LogEntry{Line: "line 1", Stderr: false}
	entry2 := LogEntry{Line: "line 2", Stderr: true}

	assert.NotNil(t, r) // Подавляем ошибку неиспользуемого импорта assert

	publisher.On("Publish", mock.Anything, projectID, taskID, sandboxID, int64(1), entry1).Return(nil).Once()
	publisher.On("Publish", mock.Anything, projectID, taskID, sandboxID, int64(2), entry2).Return(nil).Once()

	done := make(chan struct{})
	go func() {
		r.streamLogsToBus(ctx, stopCh, projectID, taskID, sandboxID, logCh)
		close(done)
	}()

	logCh <- entry1
	logCh <- entry2
	
	// Даем время на обработку
	time.Sleep(100 * time.Millisecond)

	close(logCh) // Закрытие канала логов тоже должно завершить горутину
	
	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("streamLogsToBus did not exit on logCh close")
	}

	publisher.AssertExpectations(t)
}
