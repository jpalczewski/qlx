package transport

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// RemoteTransport handles HTTP communication to another QLX instance.
type RemoteTransport struct {
	address string
	client  *http.Client
}

func (t *RemoteTransport) Name() string {
	return "remote"
}

func (t *RemoteTransport) Open(address string) error {
	t.address = address
	t.client = &http.Client{}
	return nil
}

func (t *RemoteTransport) Write(data []byte) (int, error) {
	resp, err := t.client.Post(
		t.address+"/api/print",
		"application/octet-stream",
		bytes.NewReader(data),
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("remote print failed: HTTP %d", resp.StatusCode)
	}
	return len(data), nil
}

func (t *RemoteTransport) Read(buf []byte) (int, error) {
	return 0, io.EOF
}

func (t *RemoteTransport) Close() error {
	return nil
}
