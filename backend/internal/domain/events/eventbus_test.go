package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestInMemoryBus_Base(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := NewInMemoryBus(nil, nil)
	defer bus.Close()

	projectID := uuid.New()
	ev := TaskStatusChanged{
		ProjectID: projectID,
		TaskID:    uuid.New(),
		Current:   "running",
	}

	ch, unsub := bus.Subscribe("test_sub", 10)
	defer unsub()

	bus.Publish(context.Background(), ev)

	select {
	case received := <-ch:
		assert.Equal(t, ev, received)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestInMemoryBus_SlowSubscriber(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := NewInMemoryBus(nil, nil)
	defer bus.Close()

	projectID := uuid.New()
	
	// Subscriber with 1-slot buffer
	ch, unsub := bus.Subscribe("slow_sub", 1)
	defer unsub()

	// Publish 3 events
	bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "1"})
	bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "2"})
	bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "3"})

	// Should receive "1" (buffered)
	select {
	case ev := <-ch:
		assert.Equal(t, "1", ev.(TaskStatusChanged).Current)
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// Next should be empty because 2 and 3 were dropped
	select {
	case ev := <-ch:
		t.Fatalf("received unexpected event: %v", ev)
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}

func TestInMemoryBus_Unsubscribe(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := NewInMemoryBus(nil, nil)
	defer bus.Close()

	projectID := uuid.New()
	ch, unsub := bus.Subscribe("unsub_test", 10)

	bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "1"})
	unsub()
	bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "2"})

	// Should receive "1"
	select {
	case ev := <-ch:
		assert.Equal(t, "1", ev.(TaskStatusChanged).Current)
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// Should NOT receive "2"
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("received unexpected event after unsubscribe: %v", ev)
		}
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}

func TestInMemoryBus_Close(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := NewInMemoryBus(nil, nil)
	ch, _ := bus.Subscribe("close_test", 10)

	bus.Close()

	// Channel should be closed
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("channel not closed after bus.Close()")
	}

	// Publish after close should not panic
	assert.NotPanics(t, func() {
		bus.Publish(context.Background(), TaskStatusChanged{ProjectID: uuid.New()})
	})
}

func TestInMemoryBus_Concurrency(t *testing.T) {
	defer goleak.VerifyNone(t)

	bus := NewInMemoryBus(nil, nil)
	defer bus.Close()

	const numSubscribers = 10
	const numEvents = 100
	projectID := uuid.New()

	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		go func() {
			defer wg.Done()
			ch, unsub := bus.Subscribe("concurrent_sub", numEvents)
			defer unsub()

			count := 0
			for count < numEvents {
				select {
				case <-ch:
					count++
				case <-time.After(5 * time.Second):
					return
				}
			}
		}()
	}

	// Wait for subscribers to register
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < numEvents; i++ {
		bus.Publish(context.Background(), TaskStatusChanged{ProjectID: projectID, Current: "ev"})
	}

	wg.Wait()
}
