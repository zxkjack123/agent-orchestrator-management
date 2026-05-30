package events

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBusAsyncSubscriberReceivesEvent(t *testing.T) {
	var b Bus
	var got []Event
	var mu sync.Mutex

	done := make(chan struct{})
	b.SubscribeAsync(func(e Event) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
		close(done)
	})

	e := Event{Type: TaskDone, TaskID: "TASK-1", Title: "Ship it"}
	if err := b.Emit(e); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async subscriber not called within 2s")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].TaskID != "TASK-1" {
		t.Errorf("got %+v, want one event with TaskID=TASK-1", got)
	}
}

func TestBusSyncSubscriberBlocksOnError(t *testing.T) {
	var b Bus
	asyncCalled := false

	b.SubscribeSync(func(e Event) error {
		return errors.New("approval required")
	})
	b.SubscribeAsync(func(e Event) {
		asyncCalled = true
	})

	err := b.Emit(Event{Type: TaskApprovalNeeded})
	if err == nil {
		t.Fatal("want error from sync subscriber, got nil")
	}
	if asyncCalled {
		t.Error("async subscriber must not fire when sync subscriber blocks")
	}
}

func TestBusSyncSubscriberAllowsWhenNil(t *testing.T) {
	var b Bus
	called := false

	b.SubscribeSync(func(e Event) error { return nil })
	b.SubscribeAsync(func(e Event) { called = true })

	if err := b.Emit(Event{Type: TaskReady}); err != nil {
		t.Fatalf("Emit with passing sync handler: %v", err)
	}

	// Give goroutine a moment.
	time.Sleep(50 * time.Millisecond)
	if !called {
		t.Error("async subscriber should fire when sync handler passes")
	}
}

func TestBusZeroValueIsSafe(t *testing.T) {
	var b Bus
	// Zero value should not panic.
	if err := b.Emit(Event{Type: SessionSpawned}); err != nil {
		t.Fatalf("zero-value bus Emit: %v", err)
	}
}
