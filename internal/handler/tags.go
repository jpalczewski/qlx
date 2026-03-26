package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// TagHandler handles HTTP requests for tag CRUD and assignment operations.
type TagHandler struct {
	tags      *service.TagService
	inventory *service.InventoryService
	resp      Responder
}

// NewTagHandler creates a new TagHandler.
func NewTagHandler(tags *service.TagService, inv *service.InventoryService, resp Responder) *TagHandler {
	return &TagHandler{tags: tags, inventory: inv, resp: resp}
}

// RegisterRoutes registers tag routes on the given mux.
func (h *TagHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /tags", h.List)
	mux.HandleFunc("POST /tags", h.Create)
	mux.HandleFunc("GET /tags/{id}", h.Detail)
	mux.HandleFunc("PUT /tags/{id}", h.Update)
	mux.HandleFunc("DELETE /tags/{id}", h.Delete)
	mux.HandleFunc("PATCH /tags/{id}/move", h.Move)
	mux.HandleFunc("GET /tags/{id}/descendants", h.Descendants)
	mux.HandleFunc("POST /items/{id}/tags", h.AddItemTag)
	mux.HandleFunc("DELETE /items/{id}/tags/{tag_id}", h.RemoveItemTag)
	mux.HandleFunc("POST /containers/{id}/tags", h.AddContainerTag)
	mux.HandleFunc("DELETE /containers/{id}/tags/{tag_id}", h.RemoveContainerTag)
}

// List handles GET /tags?parent_id=X.
func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")

	var data any
	if parentID != "" || r.URL.Query().Has("parent_id") {
		data = h.tags.TagChildren(parentID)
	} else {
		data = h.tags.AllTags()
	}

	h.resp.Respond(w, r, http.StatusOK, data, "tags", func() any {
		return h.tagTreeVM(parentID)
	})
}

// Create handles POST /tags.
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req UpsertTagRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	tag, err := h.tags.CreateTag(req.ParentID, req.Name, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	// Quick-entry: HX-Target is "tag-list" with beforeend swap — return single <li>
	if webutil.IsHTMX(r) && r.Header.Get("HX-Target") == "tag-list" {
		if h.resp.RenderPartial(w, r, "tags", "tag-list-item", tag) {
			return
		}
	}

	h.resp.Respond(w, r, http.StatusCreated, tag, "tags", func() any {
		return h.tagTreeVM(req.ParentID)
	})
}

// Detail handles GET /tags/{id}.
func (h *TagHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tag := h.tags.GetTag(id)
	if tag == nil {
		h.resp.RespondError(w, r, store.ErrTagNotFound)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, tag, "tag-detail", func() any {
		items := h.tags.ItemsByTag(id)
		containers := h.tags.ContainersByTag(id)
		totalQty := 0
		for _, it := range items {
			totalQty += it.Quantity
		}
		return TagDetailData{
			Tag:        *tag,
			Path:       h.tags.TagPath(id),
			Children:   h.tags.TagChildren(id),
			Items:      items,
			Containers: containers,
			Stats: TagStats{
				ItemCount:      len(items),
				ContainerCount: len(containers),
				TotalQuantity:  totalQty,
			},
		}
	})
}

// Update handles PUT /tags/{id}.
func (h *TagHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpsertTagRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	tag, err := h.tags.UpdateTag(id, req.Name, req.Color, req.Icon)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, tag, "tags", func() any {
		return h.tagTreeVM(tag.ParentID)
	})
}

// Delete handles DELETE /tags/{id}.
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	parentID, err := h.tags.DeleteTag(id)
	if err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	redirectURL := "/tags"
	if parentID != "" {
		redirectURL = "/tags?parent_id=" + parentID
	}

	h.resp.Redirect(w, r, redirectURL, map[string]any{"ok": true})
}

// Move handles PATCH /tags/{id}/move.
func (h *TagHandler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req MoveRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.tags.MoveTag(id, req.ParentID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	h.resp.Respond(w, r, http.StatusOK, map[string]any{"ok": true}, "", nil)
}

// Descendants handles GET /tags/{id}/descendants.
func (h *TagHandler) Descendants(w http.ResponseWriter, r *http.Request) {
	h.resp.Respond(w, r, http.StatusOK, h.tags.TagDescendants(r.PathValue("id")), "", nil)
}

// AddItemTag handles POST /items/{id}/tags.
func (h *TagHandler) AddItemTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req TagAssignRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.tags.AddItemTag(id, req.TagID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	item := h.inventory.GetItem(id)
	if item == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	chips := TagChipsData{
		ObjectID:   id,
		ObjectType: "item",
		Tags:       h.resolveTagIDs(item.TagIDs),
	}

	h.respondTagChips(w, r, chips)
}

// RemoveItemTag handles DELETE /items/{id}/tags/{tag_id}.
func (h *TagHandler) RemoveItemTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.PathValue("tag_id")

	if err := h.tags.RemoveItemTag(id, tagID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	item := h.inventory.GetItem(id)
	if item == nil {
		h.resp.RespondError(w, r, store.ErrItemNotFound)
		return
	}

	chips := TagChipsData{
		ObjectID:   id,
		ObjectType: "item",
		Tags:       h.resolveTagIDs(item.TagIDs),
	}

	h.respondTagChips(w, r, chips)
}

// AddContainerTag handles POST /containers/{id}/tags.
func (h *TagHandler) AddContainerTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req TagAssignRequest
	if err := BindRequest(r, &req); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	if err := h.tags.AddContainerTag(id, req.TagID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	chips := TagChipsData{
		ObjectID:   id,
		ObjectType: "container",
		Tags:       h.resolveTagIDs(container.TagIDs),
	}

	h.respondTagChips(w, r, chips)
}

// RemoveContainerTag handles DELETE /containers/{id}/tags/{tag_id}.
func (h *TagHandler) RemoveContainerTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.PathValue("tag_id")

	if err := h.tags.RemoveContainerTag(id, tagID); err != nil {
		h.resp.RespondError(w, r, err)
		return
	}

	container := h.inventory.GetContainer(id)
	if container == nil {
		h.resp.RespondError(w, r, store.ErrContainerNotFound)
		return
	}

	chips := TagChipsData{
		ObjectID:   id,
		ObjectType: "container",
		Tags:       h.resolveTagIDs(container.TagIDs),
	}

	h.respondTagChips(w, r, chips)
}

// respondTagChips sends tag chips as JSON or renders the tag-chips partial HTML.
func (h *TagHandler) respondTagChips(w http.ResponseWriter, r *http.Request, chips TagChipsData) {
	if webutil.WantsJSON(r) {
		webutil.JSON(w, http.StatusOK, chips)
		return
	}
	// For HTMX and plain fetch: render the tag-chips partial.
	// Any page template works since all include the partial via layout.
	if h.resp.RenderPartial(w, r, "containers", "tag-chips", chips) {
		return
	}
	webutil.JSON(w, http.StatusOK, chips)
}

// resolveTagIDs looks up each tag ID and returns the corresponding Tag objects.
func (h *TagHandler) resolveTagIDs(ids []string) []store.Tag {
	tags := make([]store.Tag, 0, len(ids))
	for _, id := range ids {
		if t := h.tags.GetTag(id); t != nil {
			tags = append(tags, *t)
		}
	}
	return tags
}

// tagTreeVM builds the full view model for the tag tree page.
func (h *TagHandler) tagTreeVM(parentID string) TagTreeData {
	tags := h.tags.TagChildren(parentID)
	vm := TagTreeData{
		Tags:         tags,
		ChildCounts:  make(map[string]int, len(tags)),
		DefaultColor: palette.RandomColor().Name,
		DefaultIcon:  palette.RandomIcon().Name,
	}
	for _, t := range tags {
		vm.ChildCounts[t.ID] = len(h.tags.TagChildren(t.ID))
	}
	if parentID != "" {
		parent := h.tags.GetTag(parentID)
		if parent != nil {
			vm.Parent = parent
			fullPath := h.tags.TagPath(parentID)
			if len(fullPath) > 0 {
				vm.Path = fullPath[:len(fullPath)-1]
			}
		}
	}
	return vm
}
