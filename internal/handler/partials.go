package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
)

// PartialsHandler handles HTTP requests for HTML partial fragments (tree nodes).
type PartialsHandler struct {
	inventory *service.InventoryService
	search    *service.SearchService
	tags      *service.TagService
	resp      *HTMLResponder
}

// NewPartialsHandler creates a new PartialsHandler.
func NewPartialsHandler(inv *service.InventoryService, search *service.SearchService, tags *service.TagService, resp *HTMLResponder) *PartialsHandler {
	return &PartialsHandler{inventory: inv, search: search, tags: tags, resp: resp}
}

// RegisterRoutes registers partial routes on the given mux.
func (h *PartialsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /partials/tree", h.TreeChildren)
	mux.HandleFunc("GET /partials/tree/search", h.TreeSearch)
	mux.HandleFunc("GET /partials/tag-tree", h.TagTreeChildren)
	mux.HandleFunc("GET /partials/tag-tree/search", h.TagTreeSearch)
}

// TreeChildren handles GET /partials/tree?parent_id= — returns container tree children as HTML.
func (h *PartialsHandler) TreeChildren(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	children := h.inventory.ContainerChildren(parentID)
	h.resp.RenderPartial(w, r, "containers", "tree-children", children)
}

// TreeSearch handles GET /partials/tree/search?q= — searches containers and returns HTML tree nodes.
func (h *PartialsHandler) TreeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results := h.search.SearchContainers(q)
	h.resp.RenderPartial(w, r, "containers", "tree-children", results)
}

// TagTreeChildren handles GET /partials/tag-tree?parent_id= — returns tag tree children as HTML.
func (h *PartialsHandler) TagTreeChildren(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	children := h.tags.TagChildren(parentID)
	h.resp.RenderPartial(w, r, "tags", "tag-tree-children", children)
}

// TagTreeSearch handles GET /partials/tag-tree/search?q= — searches tags and returns HTML tree nodes.
func (h *PartialsHandler) TagTreeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results := h.search.SearchTags(q)
	h.resp.RenderPartial(w, r, "tags", "tag-tree-children", results)
}
