//go:build ble

package api

import (
	"net/http"

	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

func (s *Server) HandleBluetoothScan(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) registerBluetoothRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/bluetooth/scan", s.HandleBluetoothScan)
}
