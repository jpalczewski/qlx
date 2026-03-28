package print

import "testing"

func TestConnState_StringValues(t *testing.T) {
	tests := []struct {
		state ConnState
		want  string
	}{
		{StateIdle, "idle"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateDisconnected, "disconnected"},
		{StateReconnecting, "reconnecting"},
		{StateError, "error"},
	}
	for _, tt := range tests {
		if string(tt.state) != tt.want {
			t.Errorf("ConnState %q != %q", tt.state, tt.want)
		}
	}
}
