package ui

import (
	"net/http"
)

// HandleSearch handles GET /ui/search. Searches containers, items, and tags
// for the given query string and renders the search results page.
func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	data := SearchResultsData{
		Query: q,
	}

	if q != "" {
		data.Containers = s.search.SearchContainers(q)
		data.Items = s.search.SearchItems(q)
		data.Tags = s.search.SearchTags(q)
	}

	s.render(w, r, "search", data)
}
