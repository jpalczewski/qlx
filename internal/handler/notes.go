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

	h.resp.Respond(w, r, http.StatusCreated, note, "note", nil)
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

	h.resp.Respond(w, r, http.StatusOK, note, "note", nil)
}

// Delete handles DELETE /notes/{id}.
func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.notes.DeleteNote(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

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
	h.resp.Respond(w, r, http.StatusOK, notes, "notes", nil)
}

// ItemNotes handles GET /items/{id}/notes.
func (h *NoteHandler) ItemNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.inventory.GetItem(id) == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	notes := h.notes.ItemNotes(id)
	h.resp.Respond(w, r, http.StatusOK, notes, "notes", nil)
}
