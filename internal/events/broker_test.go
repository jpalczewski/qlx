package events

import (
	"testing"
	"time"
)

func TestBrokerPublishAndUnsubscribe(t *testing.T) {
	b := NewBroker[int](4)

	ch := b.Subscribe()
	b.Publish(7)

	select {
	case got := <-ch:
		if got != 7 {
			t.Fatalf("got %d, want 7", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}

	b.Unsubscribe(ch)
	if _, ok := <-ch; ok {
		t.Fatal("expected unsubscribed channel to be closed")
	}
}

func TestBrokerSubscribeWithSnapshotRegistersAfterFill(t *testing.T) {
	b := NewBroker[int](1)

	ch := b.SubscribeWithSnapshot(func(out chan<- int) {
		out <- 1
	}, 2)

	select {
	case got := <-ch:
		if got != 1 {
			t.Fatalf("got %d, want 1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot event")
	}

	b.Publish(2)
	select {
	case got := <-ch:
		if got != 2 {
			t.Fatalf("got %d, want 2", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live event")
	}
}

func TestBrokerDropsForSlowSubscribers(t *testing.T) {
	b := NewBroker[int](1)
	ch := b.Subscribe()

	b.Publish(1)
	b.Publish(2)

	select {
	case got := <-ch:
		if got != 1 {
			t.Fatalf("got %d, want first buffered event", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for buffered event")
	}
}
