package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
	"github.com/erxyi/qlx/internal/shared/webutil"
)

// SearchHandler handles HTTP requests for search operations.
type SearchHandler struct {
	search *service.SearchService
	resp   Responder
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(search *service.SearchService, resp Responder) *SearchHandler {
	return &SearchHandler{search: search, resp: resp}
}

// RegisterRoutes registers search routes on the given mux.
func (h *SearchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", h.Search)
	mux.HandleFunc("GET /api/search/items", h.SearchItems)
}

// Search handles GET /search?q=query.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	if q == "" {
		data := map[string]any{
			"containers": []any{},
			"items":      []any{},
			"tags":       []any{},
			"notes":      []any{},
		}
		h.resp.Respond(w, r, http.StatusOK, data, "search", func() any {
			return SearchResultsData{Query: q}
		})
		return
	}

	containers := h.search.SearchContainers(q)
	items := h.search.SearchItems(q)
	tags := h.search.SearchTags(q)
	notes := h.search.SearchNotes(q)

	data := map[string]any{
		"containers": containers,
		"items":      items,
		"tags":       tags,
		"notes":      notes,
	}

	h.resp.Respond(w, r, http.StatusOK, data, "search", func() any {
		return SearchResultsData{
			Query:      q,
			Containers: containers,
			Items:      items,
			Tags:       tags,
			Notes:      notes,
		}
	})
}

const (
	defaultItemSearchLimit = 12
	maxItemSearchLimit     = 50
)

// itemSearchJSON is one hit of GET /api/search/items.
type itemSearchJSON struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Quantity      int    `json:"quantity"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Path          string `json:"path"`
}

// SearchItems handles GET /api/search/items?q=&limit=12: item name matches
// with their container path, for the start-screen wide search.
func (h *SearchHandler) SearchItems(w http.ResponseWriter, r *http.Request) {
	limit := limitParam(r, "limit", defaultItemSearchLimit, maxItemSearchLimit)
	matches := h.search.SearchItemsWithPath(r.URL.Query().Get("q"), limit)
	out := make([]itemSearchJSON, 0, len(matches))
	for _, m := range matches {
		out = append(out, itemSearchJSON{
			ID: m.Item.ID, Name: m.Item.Name, Quantity: m.Item.Quantity,
			ContainerID: m.Item.ContainerID, ContainerName: m.ContainerName, Path: m.Path,
		})
	}
	webutil.JSON(w, http.StatusOK, out)
}
