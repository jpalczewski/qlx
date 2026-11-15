package service

import (
	"strings"

	"github.com/erxyi/qlx/internal/store"
)

// ItemMatch is an item search hit together with the path of its container.
type ItemMatch struct {
	Item          store.Item
	ContainerName string
	Path          string // full container path, e.g. "Shelf / Blue Box"
}

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

// SearchItemsWithPath searches items by name and annotates each hit with its
// container path. At most limit hits are returned (limit <= 0 returns none).
func (s *SearchService) SearchItemsWithPath(query string, limit int) []ItemMatch {
	if limit <= 0 {
		return nil
	}
	hits := s.store.SearchItems(query)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	type pathInfo struct{ name, path string }
	cache := map[string]pathInfo{}
	out := make([]ItemMatch, 0, len(hits))
	for _, it := range hits {
		p, ok := cache[it.ContainerID]
		if !ok {
			chain := s.store.ContainerPath(it.ContainerID)
			names := make([]string, len(chain))
			for i, c := range chain {
				names[i] = c.Name
			}
			if len(chain) > 0 {
				p.name = chain[len(chain)-1].Name
			}
			p.path = strings.Join(names, " / ")
			cache[it.ContainerID] = p
		}
		out = append(out, ItemMatch{Item: it, ContainerName: p.name, Path: p.path})
	}
	return out
}
