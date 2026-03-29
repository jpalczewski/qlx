package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// ContainerHandler handles HTTP requests for container CRUD operations.
type ContainerHandler struct {
	inventory *service.InventoryService
	templates *service.TemplateService
	printers  *service.PrinterService
	pm        connectedPrinterProvider
	notes     *service.NoteService
	resp      Responder
}

// NewContainerHandler creates a new ContainerHandler.
func NewContainerHandler(inv *service.InventoryService, tmpl *service.TemplateService, prn *service.PrinterService, pm connectedPrinterProvider, notes *service.NoteService, resp Responder) *ContainerHandler {
	return &ContainerHandler{inventory: inv, templates: tmpl, printers: prn, pm: pm, notes: notes, resp: resp}
}

// RegisterRoutes registers container routes on the given mux.
func (h *ContainerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /containers", h.List)
	mux.HandleFunc("GET /containers/{id}", h.Detail)
	mux.HandleFunc("POST /containers", h.Create)
	mux.HandleFunc("PUT /containers/{id}", h.Update)
	mux.HandleFunc("DELETE /containers/{id}", h.Delete)
	mux.HandleFunc("GET /containers/{id}/items", h.Items)
	mux.HandleFunc("PATCH /containers/{id}/move", h.Move)
	mux.HandleFunc("GET /containers/{id}/edit", h.Edit)
}

// List handles GET /containers?parent_id=X.
func (h *ContainerHandler) List(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")

	var data any
	if parentID == "" {
		data = h.inventory.AllContainers()
	} else {
		data = h.inventory.ContainerChildren(parentID)
	}

	h.resp.Respond(w, r, http.StatusOK, data, "containers", func() any {
		return h.containerListVM(parentID)
	})
}

// Detail handles GET /containers/{id}.
func (h *ContainerHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	data := map[string]any{
		"container": container,
		"children":  h.inventory.ContainerChildren(id),
		"path":      h.inventory.ContainerPath(id),
	}

	h.resp.Respond(w, r, http.StatusOK, data, "containers", func() any {
		return h.containerListVM(id)
	})
}

// Create handles POST /containers.
func (h *ContainerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateContainerRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container, err := h.inventory.CreateContainer(req.ParentID, req.Name, req.Description, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	// Quick-entry: HX-Target is "container-list" with beforeend swap — return single <li>
	if webutil.IsHTMX(r) && r.Header.Get("HX-Target") == "container-list" {
		if h.resp.RenderPartial(w, r, "containers", "container-list-item", container) {
			return
		}
	}

	h.resp.Respond(w, r, http.StatusCreated, container, "containers", func() any {
		return h.containerListVM(req.ParentID)
	})
}

// Update handles PUT /containers/{id}.
func (h *ContainerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateContainerRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container, err := h.inventory.UpdateContainer(id, req.Name, req.Description, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, container, "containers", func() any {
		return h.containerListVM(container.ParentID)
	})
}

// Delete handles DELETE /containers/{id}.
func (h *ContainerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	parentID, err := h.inventory.DeleteContainer(id)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusNoContent, nil, "containers", func() any {
		return h.containerListVM(parentID)
	})
}

// Items handles GET /containers/{id}/items.
func (h *ContainerHandler) Items(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	data := map[string]any{
		"items": h.inventory.ContainerItems(id),
		"path":  h.inventory.ContainerPath(id),
	}

	h.resp.Respond(w, r, http.StatusOK, data, "containers", func() any {
		return h.containerListVM(id)
	})
}

// Move handles PATCH /containers/{id}/move.
func (h *ContainerHandler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req MoveRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.inventory.MoveContainer(id, req.ParentID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]any{"ok": true}, "", nil)
}

// Edit handles GET /containers/{id}/edit.
func (h *ContainerHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, container, "container-form", func() any {
		return ContainerFormData{
			Container: container,
			Path:      h.inventory.ContainerPath(id),
			ParentID:  container.ParentID,
		}
	})
}

// containerListVM builds the full view model for the container list page.
func (h *ContainerHandler) containerListVM(parentID string) ContainerListData {
	vm := ContainerListData{
		Children: h.inventory.ContainerChildren(parentID),
		Schemas:  label.SchemaNames(),
	}
	if h.pm != nil {
		vm.Printers = h.pm.ConnectedPrinters()
	}
	if h.templates != nil {
		vm.Templates = h.templates.AllTemplates()
	}
	if parentID != "" {
		container := h.inventory.GetContainer(parentID)
		if container != nil {
			vm.Container = container
			vm.Items = h.inventory.ContainerItems(parentID)
			vm.Path = h.inventory.ContainerPath(parentID)
			if h.notes != nil {
				vm.NoteCount = len(h.notes.ContainerNotes(parentID))
			}
		}
	}
	return vm
}
