package transport

import (
	"context"
	"io"
)

// MockTransport records writes and replays reads for testing.
type MockTransport struct {
	Written  []byte
	ReadData []byte
	readPos  int
}

func (m *MockTransport) Name() string                                 { return "mock" }
func (m *MockTransport) Open(_ context.Context, address string) error { return nil }
func (m *MockTransport) Write(data []byte) (int, error) {
	m.Written = append(m.Written, data...)
	return len(data), nil
}
func (m *MockTransport) Read(buf []byte) (int, error) {
	if m.readPos >= len(m.ReadData) {
		return 0, io.EOF
	}
	n := copy(buf, m.ReadData[m.readPos:])
	m.readPos += n
	return n, nil
}
func (m *MockTransport) Close() error { return nil }

// SetReadData sets data that will be returned by Read calls.
func (m *MockTransport) SetReadData(data []byte) {
	m.ReadData = data
	m.readPos = 0
}
