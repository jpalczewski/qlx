//go:build !minimal

package transport

import (
	"context"

	"go.bug.st/serial"
)

// SerialTransport handles Bluetooth serial port communication.
type SerialTransport struct {
	port serial.Port
}

func (t *SerialTransport) Name() string {
	return "serial"
}

func (t *SerialTransport) Open(ctx context.Context, address string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	port, err := serial.Open(address, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return err
	}
	t.port = port
	return nil
}

func (t *SerialTransport) Write(data []byte) (int, error) {
	return t.port.Write(data)
}

func (t *SerialTransport) Read(buf []byte) (int, error) {
	return t.port.Read(buf)
}

func (t *SerialTransport) Close() error {
	return t.port.Close()
}
