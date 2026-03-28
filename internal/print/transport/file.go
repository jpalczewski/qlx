package transport

import (
	"context"
	"os"
)

// FileTransport handles USB device communication via file operations.
type FileTransport struct {
	file *os.File
}

func (t *FileTransport) Name() string {
	return "usb"
}

func (t *FileTransport) Open(ctx context.Context, address string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	//nolint:gosec // G304: path from trusted CLI input
	file, err := os.OpenFile(address, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	t.file = file
	return nil
}

func (t *FileTransport) Write(data []byte) (int, error) {
	return t.file.Write(data)
}

func (t *FileTransport) Read(buf []byte) (int, error) {
	return t.file.Read(buf)
}

func (t *FileTransport) Close() error {
	return t.file.Close()
}
