//go:build !ble

package handler

import "net/http"

// BluetoothHandler is a no-op in non-BLE builds.
type BluetoothHandler struct{}

// NewBluetoothHandler creates a new BluetoothHandler.
func NewBluetoothHandler() *BluetoothHandler {
	return &BluetoothHandler{}
}

// RegisterRoutes is a no-op in non-BLE builds.
func (h *BluetoothHandler) RegisterRoutes(_ *http.ServeMux) {}
