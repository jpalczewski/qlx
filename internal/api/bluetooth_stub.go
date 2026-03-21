//go:build !ble

package api

import "net/http"

func (s *Server) registerBluetoothRoutes(mux *http.ServeMux) {
	// No BLE routes in non-BLE builds
}
