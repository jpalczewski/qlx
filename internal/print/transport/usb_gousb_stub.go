//go:build !usb

package transport

import "errors"

// GoUSBTransport is a stub for builds without the "usb" build tag.
type GoUSBTransport struct{}

func (t *GoUSBTransport) Name() string { return "gousb" }
func (t *GoUSBTransport) Open(address string) error {
	return errors.New("USB (gousb) not supported in this build")
}
func (t *GoUSBTransport) Write(data []byte) (int, error) {
	return 0, errors.New("USB (gousb) not supported")
}
func (t *GoUSBTransport) Read(buf []byte) (int, error) {
	return 0, errors.New("USB (gousb) not supported")
}
func (t *GoUSBTransport) Close() error { return nil }

// USBScanResult represents a discovered USB device.
type USBScanResult struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	VID     uint16 `json:"vid"`
	PID     uint16 `json:"pid"`
	Serial  string `json:"serial"`
}

// ScanUSB returns an error on non-USB builds.
func ScanUSB() ([]USBScanResult, error) {
	return nil, errors.New("USB scanning not supported in this build")
}
