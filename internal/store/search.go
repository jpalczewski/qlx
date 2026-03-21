package store

import "strings"

// SearchContainers returns all containers whose Name contains q (case-insensitive).
func (s *Store) SearchContainers(q string) []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(q)
	var result []Container
	for _, c := range s.containers {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			result = append(result, *c)
		}
	}
	return result
}

// SearchItems returns all items whose Name contains q (case-insensitive).
func (s *Store) SearchItems(q string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(q)
	var result []Item
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item.Name), lower) {
			result = append(result, *item)
		}
	}
	return result
}

// SearchTags returns all tags whose Name contains q (case-insensitive).
func (s *Store) SearchTags(q string) []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(q)
	var result []Tag
	for _, t := range s.tags {
		if strings.Contains(strings.ToLower(t.Name), lower) {
			result = append(result, *t)
		}
	}
	return result
}
