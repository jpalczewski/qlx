package handler

import (
	"testing"

	"github.com/erxyi/qlx/internal/print/transport"
)

func TestTransportOptions_MirrorRegistry(t *testing.T) {
	opts := transportOptions()
	infos := transport.List()
	if len(opts) != len(infos) {
		t.Fatalf("got %d options, registry has %d", len(opts), len(infos))
	}
	for i, in := range infos {
		o := opts[i]
		if o.Value != in.Name || o.Label != in.Label || o.Placeholder != in.AddressPlaceholder || o.Disabled == in.Available {
			t.Errorf("option %d = %+v, registry = %+v", i, o, in)
		}
		if o.Label == "" {
			t.Errorf("option %q has empty label", o.Value)
		}
	}
}

func TestTransportOptions_CoreTransportsPresent(t *testing.T) {
	have := map[string]TransportOption{}
	for _, o := range transportOptions() {
		have[o.Value] = o
	}
	for _, name := range []string{"gousb", "usb", "serial", "ble", "remote"} {
		o, ok := have[name]
		if !ok {
			t.Errorf("missing core transport %q", name)
			continue
		}
		if o.Placeholder == "" {
			t.Errorf("transport %q has no address placeholder", name)
		}
	}
	// The remote transport is pure Go and is available in every build.
	if have["remote"].Disabled {
		t.Error("remote should be available")
	}
}

func TestPrintersVM_Transports(t *testing.T) {
	h, _ := newTestPrintHandler(t)
	got := h.printersVM().Transports
	if len(got) != len(transport.Names()) {
		t.Errorf("VM has %d transports, registry has %d", len(got), len(transport.Names()))
	}
}
