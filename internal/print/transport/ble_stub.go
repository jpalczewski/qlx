//go:build !ble

package transport

import (
	"context"
	"errors"
)

type BLETransport struct{}

func (t *BLETransport) Name() string { return "ble" }
func (t *BLETransport) Open(_ context.Context, address string) error {
	return errors.New("BLE not supported in this build")
}
func (t *BLETransport) Write(data []byte) (int, error) { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Read(buf []byte) (int, error)   { return 0, errors.New("BLE not supported") }
func (t *BLETransport) Close() error                   { return nil }

// BLEScanResult represents a discovered BLE device.
type BLEScanResult struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
}

// ScanBLE returns an error on non-BLE builds.
func ScanBLE() ([]BLEScanResult, error) {
	return nil, errors.New("BLE scanning not supported in this build")
}
