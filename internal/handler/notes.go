package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

// NoteHandler handles HTTP requests for note CRUD operations.
type NoteHandler struct {
	notes     *service.NoteService
	inventory *service.InventoryService
	resp      Responder
}

// NewNoteHandler creates a new NoteHandler.
func NewNoteHandler(notes *service.NoteService, inv *service.InventoryService, resp Responder) *NoteHandler {
	return &NoteHandler{notes: notes, inventory: inv, resp: resp}
}

// RegisterRoutes registers note routes on the given mux.
func (h *NoteHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{id}", h.Detail)
	mux.HandleFunc("POST /notes", h.Create)
	mux.HandleFunc("PUT /notes/{id}", h.Update)
	mux.HandleFunc("DELETE /notes/{id}", h.Delete)
	mux.HandleFunc("GET /containers/{id}/notes", h.ContainerNotes)
	mux.HandleFunc("GET /items/{id}/notes", h.ItemNotes)
}

// Detail handles GET /notes/{id}.
func (h *NoteHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	note := h.notes.GetNote(id)
	if note == nil {
		h.resp.RespondError(w, r, store.ErrNoteNotFound)
		return
	}
	h.resp.Respond(w, r, http.StatusOK, note, "note", nil)
}

// Create handles POST /notes.
func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateNoteRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	note, err := h.notes.CreateNote(req.ContainerID, req.ItemID, req.Title, req.Content, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	w.Header().Set("HX-Trigger", "notes-changed")
	if !h.resp.RenderPartial(w, r, "item", "note-card", note) {
		// JSON fallback (no HTML rendered by RenderPartial)
		h.resp.Respond(w, r, http.StatusCreated, note, "", nil)
	}
}

// Update handles PUT /notes/{id}.
func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateNoteRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	note, err := h.notes.UpdateNote(id, req.Title, req.Content, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if !h.resp.RenderPartial(w, r, "item", "note-card", note) {
		h.resp.Respond(w, r, http.StatusOK, note, "", nil)
	}
}

// Delete handles DELETE /notes/{id}.
func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.notes.DeleteNote(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	w.Header().Set("HX-Trigger", "notes-changed")
	h.resp.Respond(w, r, http.StatusOK, map[string]string{"id": id}, "", nil)
}

// ContainerNotes handles GET /containers/{id}/notes.
func (h *NoteHandler) ContainerNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.inventory.GetContainer(id) == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	notes := h.notes.ContainerNotes(id)
	vm := NotesTabData{Notes: notes, ContainerID: id, ParentType: "container", ParentID: id}
	if !h.resp.RenderPartial(w, r, "containers", "notes-tab", vm) {
		h.resp.Respond(w, r, http.StatusOK, notes, "", nil)
	}
}

// ItemNotes handles GET /items/{id}/notes.
func (h *NoteHandler) ItemNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.inventory.GetItem(id) == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	notes := h.notes.ItemNotes(id)
	vm := NotesTabData{Notes: notes, ItemID: id, ParentType: "item", ParentID: id}
	if !h.resp.RenderPartial(w, r, "item", "notes-tab", vm) {
		h.resp.Respond(w, r, http.StatusOK, notes, "", nil)
	}
}
