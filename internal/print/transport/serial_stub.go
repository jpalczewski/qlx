//go:build minimal

package transport

import (
	"context"
	"errors"
)

type SerialTransport struct{}

func (t *SerialTransport) Name() string { return "serial" }
func (t *SerialTransport) Open(_ context.Context, address string) error {
	return errors.New("serial not supported in minimal build")
}
func (t *SerialTransport) Write(data []byte) (int, error) {
	return 0, errors.New("serial not supported")
}
func (t *SerialTransport) Read(buf []byte) (int, error) { return 0, errors.New("serial not supported") }
func (t *SerialTransport) Close() error                 { return nil }
