package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/print/label"
	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/store"
)

// ItemHandler handles HTTP requests for item CRUD operations.
type ItemHandler struct {
	inventory *service.InventoryService
	templates *service.TemplateService
	printers  *service.PrinterService
	resp      Responder
}

// NewItemHandler creates a new ItemHandler.
func NewItemHandler(inv *service.InventoryService, tmpl *service.TemplateService, prn *service.PrinterService, resp Responder) *ItemHandler {
	return &ItemHandler{inventory: inv, templates: tmpl, printers: prn, resp: resp}
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

	if req.Quantity == 0 {
		req.Quantity = 1
	}

	item, err := h.inventory.CreateItem(req.ContainerID, req.Name, req.Description, req.Quantity, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
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

	item := h.inventory.GetItem(id)
	if item == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}
	containerID := item.ContainerID

	if err := h.inventory.DeleteItem(id); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]any{"ok": true}, "containers", func() any {
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

// itemDetailVM builds the full view model for the item detail page.
func (h *ItemHandler) itemDetailVM(item *store.Item) ItemDetailData {
	vm := ItemDetailData{
		Item:    item,
		Path:    h.inventory.ContainerPath(item.ContainerID),
		Schemas: label.SchemaNames(),
	}
	if h.printers != nil {
		vm.Printers = h.printers.AllPrinters()
	}
	if h.templates != nil {
		vm.Templates = h.templates.AllTemplates()
	}
	return vm
}
