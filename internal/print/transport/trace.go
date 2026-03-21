package transport

import (
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// TraceTransport wraps a Transport with hex trace logging.
type TraceTransport struct {
	Inner Transport
}

func (t *TraceTransport) Name() string { return t.Inner.Name() }

func (t *TraceTransport) Open(address string) error {
	webutil.LogTrace("%s: open %s", t.Inner.Name(), address)
	err := t.Inner.Open(address)
	if err != nil {
		webutil.LogTrace("%s: open error: %v", t.Inner.Name(), err)
	} else {
		webutil.LogTrace("%s: opened", t.Inner.Name())
	}
	return err
}

func (t *TraceTransport) Write(data []byte) (int, error) {
	webutil.LogTrace("%s: >> %s", t.Inner.Name(), webutil.HexDump(data, 64))
	n, err := t.Inner.Write(data)
	if err != nil {
		webutil.LogTrace("%s: write error: %v", t.Inner.Name(), err)
	}
	return n, err
}

func (t *TraceTransport) Read(buf []byte) (int, error) {
	n, err := t.Inner.Read(buf)
	if n > 0 {
		webutil.LogTrace("%s: << %s", t.Inner.Name(), webutil.HexDump(buf[:n], 64))
	}
	if err != nil {
		webutil.LogTrace("%s: read error: %v", t.Inner.Name(), err)
	}
	return n, err
}

func (t *TraceTransport) Close() error {
	webutil.LogTrace("%s: close", t.Inner.Name())
	return t.Inner.Close()
}
