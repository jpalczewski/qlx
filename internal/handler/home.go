package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
)

// homeRecentLimit caps the number of recent containers shown on the start screen.
const homeRecentLimit = 12

// HomeData is the view model of the start screen ("W użyciu"). It holds only
// containers: the start screen never shows items.
type HomeData struct {
	Pins    []service.WorkingSetEntry
	Recents []service.WorkingSetEntry
}

// HomeHandler serves the start screen at "/".
type HomeHandler struct {
	inventory *service.InventoryService
	ws        *service.WorkingSetService
	resp      *HTMLResponder
}

// NewHomeHandler creates a HomeHandler.
func NewHomeHandler(inv *service.InventoryService, ws *service.WorkingSetService, resp *HTMLResponder) *HomeHandler {
	return &HomeHandler{inventory: inv, ws: ws, resp: resp}
}

// RegisterRoutes registers the root route on the given mux.
func (h *HomeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.Home)
}

// Home handles GET /. HTML requests get the working-set start screen; JSON
// requests keep the original contract and receive the root container list.
func (h *HomeHandler) Home(w http.ResponseWriter, r *http.Request) {
	h.resp.Respond(w, r, http.StatusOK, h.inventory.AllContainers(), "home", func() any {
		snap := h.ws.Snapshot(homeRecentLimit)
		return HomeData{Pins: snap.Pins, Recents: snap.Recents}
	})
}
