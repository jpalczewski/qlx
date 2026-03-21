package api

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		webutil.JSON(w, http.StatusOK, map[string]any{
			"containers": []any{},
			"items":      []any{},
			"tags":       []any{},
		})
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]any{
		"containers": s.search.SearchContainers(q),
		"items":      s.search.SearchItems(q),
		"tags":       s.search.SearchTags(q),
	})
}
