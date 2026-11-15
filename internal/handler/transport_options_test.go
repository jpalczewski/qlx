package handler

import (
	"reflect"
	"testing"
)

func TestTransportOptions(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []TransportOption
	}{
		{"all known keep legacy order", []string{"ble", "gousb", "remote", "serial", "usb"}, []TransportOption{
			{"gousb", "USB (bidirectional)"},
			{"usb", "USB (file)"},
			{"serial", "Serial/Bluetooth"},
			{"ble", "Bluetooth (BLE)"},
			{"remote", "Remote QLX"},
		}},
		{"subset", []string{"remote", "usb"}, []TransportOption{
			{"usb", "USB (file)"},
			{"remote", "Remote QLX"},
		}},
		{"unknown appended with name label", []string{"foo", "usb"}, []TransportOption{
			{"usb", "USB (file)"},
			{"foo", "foo"},
		}},
		{"empty", nil, []TransportOption{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transportOptions(tt.names); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintersVM_Transports(t *testing.T) {
	h, _ := newTestPrintHandler(t)
	got := h.printersVM().Transports
	if len(got) != 5 || got[0].Value != "gousb" {
		t.Errorf("unexpected transports: %v", got)
	}
}
