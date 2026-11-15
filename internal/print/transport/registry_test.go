package transport

import (
	"reflect"
	"sort"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		wantNil  bool
		wantName string
	}{
		{"usb", false, "usb"},
		{"gousb", false, "gousb"},
		{"serial", false, "serial"},
		{"ble", false, "ble"},
		{"remote", false, "remote"},
		{"mock", true, ""},
		{"", true, ""},
		{"nope", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.name)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("New(%q) = %v, want nil", tt.name, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("New(%q) = nil", tt.name)
			}
			if got.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", got.Name(), tt.wantName)
			}
		})
	}
}

// TestNamesMatchLegacySwitch pins the names accepted by the former switch in
// print.DefaultTransportFactory; every build-tag variant must register them.
func TestNamesMatchLegacySwitch(t *testing.T) {
	want := []string{"ble", "gousb", "remote", "serial", "usb"}
	got := Names()
	if !sort.StringsAreSorted(got) {
		t.Errorf("Names() not sorted: %v", got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

func TestNewReturnsFreshInstances(t *testing.T) {
	if New("remote") == New("remote") {
		t.Error("New should return a new instance on every call")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("usb", func() Transport { return &FileTransport{} })
}

func TestRegisterCustomAndNamesSorted(t *testing.T) {
	Register("aaa-test", func() Transport { return &MockTransport{} })
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, "aaa-test")
		registryMu.Unlock()
	})
	if New("aaa-test") == nil {
		t.Fatal("custom transport not constructed")
	}
	names := Names()
	if names[0] != "aaa-test" || !sort.StringsAreSorted(names) {
		t.Errorf("Names() = %v", names)
	}
}
