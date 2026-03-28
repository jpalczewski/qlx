package print

import "time"

// ConnState represents the connection state of a printer.
type ConnState string

const (
	StateIdle         ConnState = "idle"
	StateConnecting   ConnState = "connecting"
	StateConnected    ConnState = "connected"
	StateDisconnected ConnState = "disconnected"
	StateReconnecting ConnState = "reconnecting"
	StateError        ConnState = "error"
)

// StateChange is emitted when a printer's connection state changes.
type StateChange struct {
	PrinterID string    `json:"printer_id"`
	State     ConnState `json:"state"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
