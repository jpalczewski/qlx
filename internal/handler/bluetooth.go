//go:build ble

package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// BluetoothHandler handles BLE device scanning.
type BluetoothHandler struct{}

// NewBluetoothHandler creates a new BluetoothHandler.
func NewBluetoothHandler() *BluetoothHandler {
	return &BluetoothHandler{}
}

// RegisterRoutes registers BLE routes on the given mux.
func (h *BluetoothHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /bluetooth/scan", h.Scan)
}

// Scan handles GET /bluetooth/scan.
func (h *BluetoothHandler) Scan(w http.ResponseWriter, r *http.Request) {
	webutil.LogInfo("starting BLE scan...")
	results, err := transport.ScanBLE()
	if err != nil {
		webutil.LogError("BLE scan error: %v", err)
		webutil.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	webutil.LogInfo("BLE scan found %d devices", len(results))
	webutil.JSON(w, http.StatusOK, results)
}
