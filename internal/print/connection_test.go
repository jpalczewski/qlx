package print

import (
	"context"
	"testing"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/store"
)

func testPrinterCfg(id string) store.PrinterConfig {
	return store.PrinterConfig{
		ID:        id,
		Name:      "Test " + id,
		Encoder:   "mock",
		Model:     "mock-model",
		Transport: "mock",
		Address:   "/dev/null",
	}
}

func newTestCM(t *testing.T) *ConnectionManager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cm := NewConnectionManager(
		func(name string) transport.Transport {
			if name == "mock" {
				return &transport.MockTransport{}
			}
			return nil
		},
		func(name string) encoder.Encoder {
			if name == "mock" {
				return &mockEncoder{}
			}
			return nil
		},
	)
	cm.Start(ctx)
	t.Cleanup(func() { cancel(); cm.Stop() })
	return cm
}

func TestConnectionManager_AddTransitionsToConnecting(t *testing.T) {
	cm := newTestCM(t)
	cfg := testPrinterCfg("p1")

	if err := cm.Add(cfg); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Give the goroutine a moment to transition from idle.
	time.Sleep(50 * time.Millisecond)

	state := cm.State("p1")
	// With MockTransport + mockEncoder (no StatusQuerier), the session connects
	// but immediately closes stopped (no heartbeat), so the loop cycles rapidly.
	// State should be one of: connecting, connected, disconnected, or reconnecting.
	switch state {
	case StateConnecting, StateConnected, StateDisconnected, StateReconnecting:
		// ok
	default:
		t.Fatalf("unexpected state after Add: %s", state)
	}
}

func TestConnectionManager_MockConnects(t *testing.T) {
	cm := newTestCM(t)
	cfg := testPrinterCfg("p1")

	if err := cm.Add(cfg); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// The session will reach connected then immediately disconnect (no heartbeat).
	// We subscribe and wait for at least one StateConnected event.
	ch := cm.Subscribe()
	defer cm.Unsubscribe(ch)

	deadline := time.After(2 * time.Second)
	sawConnected := false
	for !sawConnected {
		select {
		case evt := <-ch:
			if evt.PrinterID == "p1" && evt.State == StateConnected {
				sawConnected = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for StateConnected")
		}
	}
}

func TestConnectionManager_Remove(t *testing.T) {
	cm := newTestCM(t)
	cfg := testPrinterCfg("p1")

	if err := cm.Add(cfg); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := cm.Remove("p1"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	state := cm.State("p1")
	if state != "" {
		t.Fatalf("expected empty state after Remove, got %s", state)
	}
}

func TestConnectionManager_SubscribeSnapshot(t *testing.T) {
	cm := newTestCM(t)
	cfg := testPrinterCfg("p1")

	if err := cm.Add(cfg); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Wait a moment for the loop to run.
	time.Sleep(100 * time.Millisecond)

	ch := cm.Subscribe()
	defer cm.Unsubscribe(ch)

	// The snapshot should arrive immediately (buffered channel).
	select {
	case evt := <-ch:
		if evt.PrinterID != "p1" {
			t.Fatalf("expected printer p1 in snapshot, got %s", evt.PrinterID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for snapshot event")
	}
}

func TestConnectionManager_Reconnect(t *testing.T) {
	cm := newTestCM(t)
	cfg := testPrinterCfg("p1")

	if err := cm.Add(cfg); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Let it cycle a bit.
	time.Sleep(100 * time.Millisecond)

	// Subscribe to watch for reconnection events.
	ch := cm.Subscribe()
	defer cm.Unsubscribe(ch)

	if err := cm.Reconnect("p1"); err != nil {
		t.Fatalf("Reconnect() error: %v", err)
	}

	// After Reconnect, we should see at least one connecting event.
	deadline := time.After(2 * time.Second)
	sawConnecting := false
	for !sawConnecting {
		select {
		case evt := <-ch:
			if evt.PrinterID == "p1" && evt.State == StateConnecting {
				sawConnecting = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for StateConnecting after Reconnect")
		}
	}
}
