package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

const (
	defaultRecentLimit      = 8
	maxRecentLimit          = 50
	defaultQuickSwitchLimit = 20
	maxQuickSwitchLimit     = 100
	maxReorderBody          = 1 << 20

	// workingSetChangedEvent is the HX-Trigger event fired after the pin list changes.
	workingSetChangedEvent = "workingSetChanged"
)

// WorkingSetHandler serves the working set (pins, recents, quick switch) endpoints.
type WorkingSetHandler struct {
	ws   *service.WorkingSetService
	resp Responder
}

// NewWorkingSetHandler creates a WorkingSetHandler.
func NewWorkingSetHandler(ws *service.WorkingSetService, resp Responder) *WorkingSetHandler {
	return &WorkingSetHandler{ws: ws, resp: resp}
}

// RegisterRoutes registers working set routes on the given mux.
func (h *WorkingSetHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/working-set", h.Snapshot)
	mux.HandleFunc("PUT /api/working-set/pins", h.ReorderPins)
	mux.HandleFunc("POST /api/working-set/pins/{id}", h.Pin)
	mux.HandleFunc("DELETE /api/working-set/pins/{id}", h.Unpin)
	mux.HandleFunc("POST /api/working-set/recents/{id}", h.RecordOpen)
	mux.HandleFunc("GET /api/quick-switch", h.QuickSwitch)
	mux.HandleFunc("GET /partials/rail", h.Rail)
}

// workingSetEntryJSON is the JSON form of a working set container.
type workingSetEntryJSON struct {
	ID           string     `json:"id"`
	ParentID     string     `json:"parent_id"`
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	Color        string     `json:"color"`
	Icon         string     `json:"icon"`
	ItemCount    int        `json:"item_count"`
	Pinned       bool       `json:"pinned"`
	LastOpenedAt *time.Time `json:"last_opened_at"`
	OpenCount    int        `json:"open_count"`
}

type workingSetJSON struct {
	Pins    []workingSetEntryJSON `json:"pins"`
	Recents []workingSetEntryJSON `json:"recents"`
}

type quickSwitchJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Color  string `json:"color"`
	Icon   string `json:"icon"`
	Pinned bool   `json:"pinned"`
	Score  int    `json:"score"`
}

func snapshotJSON(snap service.WorkingSetSnapshot) workingSetJSON {
	return workingSetJSON{Pins: entriesJSON(snap.Pins), Recents: entriesJSON(snap.Recents)}
}

func entriesJSON(entries []service.WorkingSetEntry) []workingSetEntryJSON {
	out := make([]workingSetEntryJSON, 0, len(entries))
	for _, e := range entries {
		j := workingSetEntryJSON{
			ID: e.Container.ID, ParentID: e.Container.ParentID, Name: e.Container.Name,
			Path: e.Path, Color: e.Container.Color, Icon: e.Container.Icon,
			ItemCount: e.ItemCount, Pinned: e.Pinned, OpenCount: e.OpenCount,
		}
		if !e.LastOpenedAt.IsZero() {
			t := e.LastOpenedAt
			j.LastOpenedAt = &t
		}
		out = append(out, j)
	}
	return out
}

// limitParam parses a positive integer query parameter, falling back to def
// for missing or invalid values and clamping to maxVal.
func limitParam(r *http.Request, name string, def, maxVal int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n <= 0 {
		return def
	}
	return min(n, maxVal)
}

// limitedSnapshot returns the working set snapshot with recents capped by ?recent=.
func (h *WorkingSetHandler) limitedSnapshot(r *http.Request) service.WorkingSetSnapshot {
	return h.ws.Snapshot(limitParam(r, "recent", defaultRecentLimit, maxRecentLimit))
}

// Snapshot handles GET /api/working-set?recent=8.
func (h *WorkingSetHandler) Snapshot(w http.ResponseWriter, r *http.Request) {
	webutil.JSON(w, http.StatusOK, snapshotJSON(h.limitedSnapshot(r)))
}

// Pin handles POST /api/working-set/pins/{id}.
func (h *WorkingSetHandler) Pin(w http.ResponseWriter, r *http.Request) {
	h.pinsChanged(w, r, h.ws.Pin(r.PathValue("id")))
}

// Unpin handles DELETE /api/working-set/pins/{id}.
func (h *WorkingSetHandler) Unpin(w http.ResponseWriter, r *http.Request) {
	h.pinsChanged(w, r, h.ws.Unpin(r.PathValue("id")))
}

// ReorderPins handles PUT /api/working-set/pins with {"ids":[...]}.
func (h *WorkingSetHandler) ReorderPins(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs *[]string `json:"ids"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxReorderBody))
	err := dec.Decode(&body)
	if err == nil && dec.More() {
		err = errors.New("unexpected data after JSON value")
	}
	if err == nil && (body.IDs == nil || *body.IDs == nil) {
		err = errors.New(`"ids" is required`)
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			webutil.JSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
			return
		}
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": `invalid body: expected {"ids": [...]}`})
		return
	}
	h.pinsChanged(w, r, h.ws.ReorderPins(*body.IDs))
}

// pinsChanged finishes a pin mutation: error response, or 204 with HX-Trigger.
func (h *WorkingSetHandler) pinsChanged(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}
	w.Header().Set("HX-Trigger", workingSetChangedEvent)
	w.WriteHeader(http.StatusNoContent)
}

// RecordOpen handles POST /api/working-set/recents/{id}.
func (h *WorkingSetHandler) RecordOpen(w http.ResponseWriter, r *http.Request) {
	if err := h.ws.RecordOpen(r.PathValue("id")); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// QuickSwitch handles GET /api/quick-switch?q=&limit=20.
func (h *WorkingSetHandler) QuickSwitch(w http.ResponseWriter, r *http.Request) {
	limit := limitParam(r, "limit", defaultQuickSwitchLimit, maxQuickSwitchLimit)
	results := h.ws.QuickSwitch(r.URL.Query().Get("q"), limit)
	out := make([]quickSwitchJSON, 0, len(results))
	for _, m := range results {
		out = append(out, quickSwitchJSON{
			ID: m.Container.ID, Name: m.Container.Name, Path: m.Path,
			Color: m.Container.Color, Icon: m.Container.Icon, Pinned: m.Pinned, Score: m.Score,
		})
	}
	webutil.JSON(w, http.StatusOK, out)
}

// Rail handles GET /partials/rail — the working set rail HTML fragment.
func (h *WorkingSetHandler) Rail(w http.ResponseWriter, r *http.Request) {
	snap := h.limitedSnapshot(r)
	if !h.resp.RenderPartial(w, r, "rail", "rail", snap) && webutil.WantsJSON(r) {
		webutil.JSON(w, http.StatusOK, snapshotJSON(snap))
	}
}
