package api

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
)

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	webutil.JSON(w, http.StatusOK, map[string]any{
		"containers": s.store.SearchContainers(q),
		"items":      s.store.SearchItems(q),
		"tags":       s.store.SearchTags(q),
	})
}
