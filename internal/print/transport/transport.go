package transport

import (
	"context"
	"errors"
)

// ErrTimeout is wrapped by transports when an I/O operation exceeded its
// deadline (for example a USB bulk transfer the printer never answered). It
// signals "no answer yet", not a lost device; callers decide how long to keep
// waiting.
var ErrTimeout = errors.New("transport: i/o timeout")

// Transport abstracts communication with a printer device.
type Transport interface {
	Name() string
	Open(ctx context.Context, address string) error
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close() error
}
