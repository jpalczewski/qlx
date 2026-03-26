package service

import "github.com/erxyi/qlx/internal/store"

// SearchService provides search functionality (thin passthrough, no mutations).
type SearchService struct {
	store store.SearchStore
}

// NewSearchService creates a new SearchService backed by the given store.
func NewSearchService(s store.SearchStore) *SearchService {
	return &SearchService{store: s}
}

// SearchContainers searches containers by name.
func (s *SearchService) SearchContainers(query string) []store.Container {
	return s.store.SearchContainers(query)
}

// SearchItems searches items by name.
func (s *SearchService) SearchItems(query string) []store.Item {
	return s.store.SearchItems(query)
}

// SearchTags searches tags by name.
func (s *SearchService) SearchTags(query string) []store.Tag {
	return s.store.SearchTags(query)
}

// SearchNotes searches notes by title and content.
func (s *SearchService) SearchNotes(query string) []store.Note {
	return s.store.SearchNotes(query)
}
