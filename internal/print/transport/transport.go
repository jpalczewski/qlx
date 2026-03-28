package transport

import "context"

// Transport abstracts communication with a printer device.
type Transport interface {
	Name() string
	Open(ctx context.Context, address string) error
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
	Close() error
}
