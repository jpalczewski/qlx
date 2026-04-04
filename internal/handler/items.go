package handler

import (
	"fmt"
	"net/http"

	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// ItemHandler handles HTTP requests for item CRUD operations.
type ItemHandler struct {
	inventory *service.InventoryService
	templates *service.TemplateService
	printers  *service.PrinterService
	pm        connectedPrinterProvider
	notes     *service.NoteService
	tags      *service.TagService
	resp      Responder
}

// NewItemHandler creates a new ItemHandler.
func NewItemHandler(inv *service.InventoryService, tmpl *service.TemplateService, prn *service.PrinterService, pm connectedPrinterProvider, notes *service.NoteService, tags *service.TagService, resp Responder) *ItemHandler {
	return &ItemHandler{inventory: inv, templates: tmpl, printers: prn, pm: pm, notes: notes, tags: tags, resp: resp}
}

// RegisterRoutes registers item routes on the given mux.
func (h *ItemHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /items/{id}", h.Detail)
	mux.HandleFunc("POST /items", h.Create)
	mux.HandleFunc("PUT /items/{id}", h.Update)
	mux.HandleFunc("DELETE /items/{id}", h.Delete)
	mux.HandleFunc("PATCH /items/{id}/move", h.Move)
	mux.HandleFunc("GET /items/{id}/edit", h.Edit)
}

// Detail handles GET /items/{id}.
func (h *ItemHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item := h.inventory.GetItem(id)
	if item == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	data := h.itemDetailData(item)

	h.resp.Respond(w, r, http.StatusOK, data, "item", func() any {
		return h.itemDetailVM(item)
	})
}

// Create handles POST /items.
func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateItemRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}
	// BindRequest uses r.FormValue which only returns the first value for multi-value
	// fields; read tag_ids slice directly from the parsed form.
	if r.Form != nil {
		req.TagIDs = r.Form["tag_ids"]
	}

	if req.ContainerID == "" {
		h.resp.RespondError(w, r, fmt.Errorf("%w: container_id is required", store.ErrInvalidContainer))
		return
	}

	if req.Quantity == 0 {
		req.Quantity = 1
	}

	if invalidID := h.findInvalidTagID(req.TagIDs); invalidID != "" {
		h.resp.RespondError(w, r, fmt.Errorf("tag not found: %s", invalidID))
		return
	}

	item, err := h.inventory.CreateItem(req.ContainerID, req.Name, req.Description, req.Quantity, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if h.tags != nil {
		for _, tagID := range req.TagIDs {
			if tagID == "" {
				continue
			}
			if err := h.tags.AddItemTag(item.ID, tagID); err != nil {
				webutil.WriteError(w, http.StatusInternalServerError, fmt.Errorf("assign tag %s: %w", tagID, err))
				return
			}
		}
		// Re-fetch to include tag IDs in response
		item = h.inventory.GetItem(item.ID)
	}

	// Quick-entry: HX-Target is "item-list" with beforeend swap — return single <li>
	if webutil.IsHTMX(r) && r.Header.Get("HX-Target") == "item-list" {
		if h.resp.RenderPartial(w, r, "containers", "item-list-item", item) {
			return
		}
	}

	h.resp.Respond(w, r, http.StatusCreated, item, "containers", func() any {
		return ContainerListData{
			Children: h.inventory.ContainerChildren(req.ContainerID),
			Items:    h.inventory.ContainerItems(req.ContainerID),
		}
	})
}

// Update handles PUT /items/{id}.
func (h *ItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateItemRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	item, err := h.inventory.UpdateItem(id, req.Name, req.Description, req.Quantity, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	data := h.itemDetailData(item)

	h.resp.Respond(w, r, http.StatusOK, data, "item", func() any {
		return h.itemDetailVM(item)
	})
}

// Delete handles DELETE /items/{id}.
func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	containerID, err := h.inventory.DeleteItem(id)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusNoContent, nil, "containers", func() any {
		return ContainerListData{
			Children: h.inventory.ContainerChildren(containerID),
			Items:    h.inventory.ContainerItems(containerID),
		}
	})
}

// Move handles PATCH /items/{id}/move.
func (h *ItemHandler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req MoveRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.inventory.MoveItem(id, req.ContainerID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]any{"ok": true}, "", nil)
}

// Edit handles GET /items/{id}/edit.
func (h *ItemHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item := h.inventory.GetItem(id)
	if item == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, item, "item-form", func() any {
		return ItemFormData{
			Item:        item,
			Path:        h.inventory.ContainerPath(item.ContainerID),
			ContainerID: item.ContainerID,
		}
	})
}

// itemDetailData builds the JSON response data for an item detail.
func (h *ItemHandler) itemDetailData(item *store.Item) map[string]any {
	return map[string]any{
		"item":       item,
		"path":       h.inventory.ContainerPath(item.ContainerID),
		"qr_content": "/items/" + item.ID,
	}
}

// findInvalidTagID returns the first tag ID in ids that does not exist, or empty string if all are valid.
func (h *ItemHandler) findInvalidTagID(ids []string) string {
	if h.tags == nil {
		return ""
	}
	for _, id := range ids {
		if id != "" && h.tags.GetTag(id) == nil {
			return id
		}
	}
	return ""
}

// itemDetailVM builds the full view model for the item detail page.
func (h *ItemHandler) itemDetailVM(item *store.Item) ItemDetailData {
	vm := ItemDetailData{
		Item:    item,
		Path:    h.inventory.ContainerPath(item.ContainerID),
		Schemas: label.SchemaNames(),
	}
	if h.pm != nil {
		vm.Printers = h.pm.ConnectedPrinters()
	}
	if h.templates != nil {
		vm.Templates = h.templates.AllTemplates()
	}
	if h.notes != nil {
		vm.NoteCount = len(h.notes.ItemNotes(item.ID))
	}
	return vm
}
