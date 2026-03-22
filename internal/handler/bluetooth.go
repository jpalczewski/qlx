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
	unique := deduplicateBLE(results)
	webutil.LogInfo("BLE scan found %d devices (%d raw)", len(unique), len(results))
	webutil.JSON(w, http.StatusOK, unique)
}

// deduplicateBLE removes duplicate scan results by address, keeping the strongest RSSI.
func deduplicateBLE(results []transport.BLEScanResult) []transport.BLEScanResult {
	best := make(map[string]transport.BLEScanResult)
	for _, r := range results {
		if existing, ok := best[r.Address]; !ok || r.RSSI > existing.RSSI {
			best[r.Address] = r
		}
	}
	unique := make([]transport.BLEScanResult, 0, len(best))
	for _, r := range best {
		unique = append(unique, r)
	}
	return unique
}
