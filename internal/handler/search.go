package handler

import (
	"net/http"

	"github.com/erxyi/qlx/internal/service"
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
}

// Search handles GET /search?q=query.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	if q == "" {
		data := map[string]any{
			"containers": []any{},
			"items":      []any{},
			"tags":       []any{},
		}
		h.resp.Respond(w, r, http.StatusOK, data, "search", func() any {
			return SearchResultsData{Query: q}
		})
		return
	}

	containers := h.search.SearchContainers(q)
	items := h.search.SearchItems(q)
	tags := h.search.SearchTags(q)

	data := map[string]any{
		"containers": containers,
		"items":      items,
		"tags":       tags,
	}

	h.resp.Respond(w, r, http.StatusOK, data, "search", func() any {
		return SearchResultsData{
			Query:      q,
			Containers: containers,
			Items:      items,
			Tags:       tags,
		}
	})
}
