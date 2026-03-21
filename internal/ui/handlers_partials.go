package ui

import (
	"net/http"
)

// HandleTreePartial handles GET /ui/partials/tree?parent_id=.
// Returns the direct children of a container as HTML tree nodes.
func (s *Server) HandleTreePartial(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	children := s.store.ContainerChildren(parentID)
	s.renderPartial(w, r, "containers", "tree-children", children)
}

// HandleTreeSearchPartial handles GET /ui/partials/tree/search?q=.
// Searches containers and returns results as HTML tree nodes.
func (s *Server) HandleTreeSearchPartial(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results := s.store.SearchContainers(q)
	s.renderPartial(w, r, "containers", "tree-children", results)
}

// HandleTagTreePartial handles GET /ui/partials/tag-tree?parent_id=.
// Returns the direct children of a tag as HTML tree nodes.
func (s *Server) HandleTagTreePartial(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent_id")
	children := s.store.TagChildren(parentID)
	s.renderPartial(w, r, "tags", "tag-tree-children", children)
}

// HandleTagTreeSearchPartial handles GET /ui/partials/tag-tree/search?q=.
// Searches tags and returns results as HTML tree nodes.
func (s *Server) HandleTagTreeSearchPartial(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results := s.store.SearchTags(q)
	s.renderPartial(w, r, "tags", "tag-tree-children", results)
}
