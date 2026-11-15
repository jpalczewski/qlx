package transport

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIOContext(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := ioContext(context.Background(), 20*time.Millisecond)
		defer cancel()
		dl, ok := ctx.Deadline()
		if !ok || time.Until(dl) > 20*time.Millisecond {
			t.Fatalf("deadline = %v, ok=%v; want within 20ms", dl, ok)
		}
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("Err = %v", ctx.Err())
		}
	})
	t.Run("close aborts", func(t *testing.T) {
		parent, closeAll := context.WithCancel(context.Background())
		ctx, cancel := ioContext(parent, time.Hour)
		defer cancel()
		closeAll()
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
			t.Fatal("cancelling the parent (Close) did not abort the I/O context")
		}
	})
	t.Run("nil parent", func(t *testing.T) {
		ctx, cancel := ioContext(nil, time.Hour) //nolint:staticcheck // nil parent is supported on purpose
		defer cancel()
		if ctx.Err() != nil {
			t.Fatalf("Err = %v", ctx.Err())
		}
	})
}

func TestClassifyIOError(t *testing.T) {
	underlying := errors.New("libusb: transfer cancelled")

	expired, c1 := ioContext(context.Background(), time.Nanosecond)
	defer c1()
	<-expired.Done()

	parent, closeAll := context.WithCancel(context.Background())
	closed, c2 := ioContext(parent, time.Hour)
	defer c2()
	closeAll()
	<-closed.Done()

	live, c3 := ioContext(context.Background(), time.Hour)
	defer c3()

	tests := []struct {
		name        string
		ctx         context.Context
		err         error
		wantTimeout bool
		wantNil     bool
	}{
		{"deadline expired is a timeout", expired, underlying, true, false},
		{"aborted by Close is not a timeout", closed, underlying, false, false},
		{"other error is untouched", live, underlying, false, false},
		{"nil stays nil", expired, nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyIOError("op", tt.ctx, time.Second, tt.err)
			if (got == nil) != tt.wantNil {
				t.Fatalf("got %v, wantNil %v", got, tt.wantNil)
			}
			if errors.Is(got, ErrTimeout) != tt.wantTimeout {
				t.Fatalf("Is(ErrTimeout) = %v, want %v (err=%v)", !tt.wantTimeout, tt.wantTimeout, got)
			}
		})
	}
}
