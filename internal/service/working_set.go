package service

import (
	"sort"
	"strings"
	"time"

	"github.com/erxyi/qlx/internal/store"
)

// WorkingSetService manages the "in use" working set: pinned containers,
// recently opened containers and quick-switch search over them.
type WorkingSetService struct {
	store interface {
		store.WorkingSetStore
		store.ContainerStore
	}
}

// NewWorkingSetService creates a WorkingSetService backed by the given store.
func NewWorkingSetService(s interface {
	store.WorkingSetStore
	store.ContainerStore
}) *WorkingSetService {
	return &WorkingSetService{store: s}
}

// WorkingSetEntry is a container in the working set with display data.
type WorkingSetEntry struct {
	Container    store.Container
	Path         string // "Parent / Child / Name"
	ItemCount    int
	Pinned       bool
	LastOpenedAt time.Time // zero for pins that were never opened
	OpenCount    int
}

// ParentPath returns Path without the container's own name ("Parent / Child"),
// or "" for a root container.
func (e WorkingSetEntry) ParentPath() string {
	if !strings.HasSuffix(e.Path, e.Container.Name) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(e.Path, e.Container.Name), " / ")
}

// WorkingSetSnapshot is the data needed to render the working set.
type WorkingSetSnapshot struct {
	Pins    []WorkingSetEntry
	Recents []WorkingSetEntry // excludes pinned containers
}

// Pin pins a container.
func (s *WorkingSetService) Pin(id string) error {
	return s.store.PinContainer(id)
}

// Unpin removes a pin.
func (s *WorkingSetService) Unpin(id string) error {
	return s.store.UnpinContainer(id)
}

// ReorderPins sets the display order of pinned containers.
func (s *WorkingSetService) ReorderPins(ids []string) error {
	return s.store.ReorderPins(ids)
}

// RecordOpen notes that a container was opened now.
func (s *WorkingSetService) RecordOpen(id string) error {
	return s.store.RecordContainerOpen(id, time.Now())
}

// IsPinned reports whether the container is pinned. It reads the store on
// every call (a small indexed query), so it is always consistent with pin,
// unpin, reorder and container deletion.
func (s *WorkingSetService) IsPinned(id string) bool {
	for _, p := range s.store.Pins() {
		if p == id {
			return true
		}
	}
	return false
}

// Snapshot returns pins (in order) and up to recentLimit recents (newest
// first, pinned ones excluded), each with item counts.
func (s *WorkingSetService) Snapshot(recentLimit int) WorkingSetSnapshot {
	pinIDs := s.store.Pins()
	pinSet := make(map[string]bool, len(pinIDs))
	for _, id := range pinIDs {
		pinSet[id] = true
	}

	allRecents := s.store.RecentContainers(store.MaxRecentContainers)
	meta := make(map[string]store.RecentContainer, len(allRecents))
	var recents []store.RecentContainer
	for _, r := range allRecents {
		meta[r.ContainerID] = r
		if !pinSet[r.ContainerID] && len(recents) < recentLimit {
			recents = append(recents, r)
		}
	}

	ids := append([]string(nil), pinIDs...)
	for _, r := range recents {
		ids = append(ids, r.ContainerID)
	}
	counts := s.store.ContainerItemCounts(ids)
	build := func(id string, pinned bool) (WorkingSetEntry, bool) {
		c := s.store.GetContainer(id)
		if c == nil {
			return WorkingSetEntry{}, false
		}
		e := WorkingSetEntry{
			Container: *c,
			Path:      s.pathString(id),
			ItemCount: counts[id],
			Pinned:    pinned,
		}
		if r, ok := meta[id]; ok {
			e.LastOpenedAt, e.OpenCount = r.LastOpenedAt, r.OpenCount
		}
		return e, true
	}

	var snap WorkingSetSnapshot
	for _, id := range pinIDs {
		if e, ok := build(id, true); ok {
			snap.Pins = append(snap.Pins, e)
		}
	}
	for _, r := range recents {
		if e, ok := build(r.ContainerID, false); ok {
			snap.Recents = append(snap.Recents, e)
		}
	}
	return snap
}

func (s *WorkingSetService) pathString(id string) string {
	path := s.store.ContainerPath(id)
	names := make([]string, len(path))
	for i, c := range path {
		names[i] = c.Name
	}
	return strings.Join(names, " / ")
}

// QuickSwitchResult is one ranked quick-switch match.
type QuickSwitchResult struct {
	Container store.Container
	Path      string
	Pinned    bool
	Score     int
}

// Ranking weights for QuickSwitch.
const (
	scoreNameExact     = 1000
	scoreNamePrefix    = 800
	scoreNameWord      = 650
	scoreNameSubstring = 500
	scorePathSubstring = 200
	boostPinned        = 300
	boostRecent        = 100
)

// QuickSwitch ranks containers against q. Matching is case- and
// diacritics-insensitive (e.g. "zolty" matches "Żółty") and tiers are: exact
// name, name prefix, word-prefix in name, name substring, path substring.
// Pinned and recently opened containers are boosted. An empty query returns
// the pinned containers followed by recents.
func (s *WorkingSetService) QuickSwitch(q string, limit int) []QuickSwitchResult {
	if limit <= 0 {
		return nil
	}
	nq := normalizeSearch(q)

	pinSet := map[string]bool{}
	for _, id := range s.store.Pins() {
		pinSet[id] = true
	}
	recentRank := map[string]int{} // 0 = most recent
	for i, r := range s.store.RecentContainers(store.MaxRecentContainers) {
		recentRank[r.ContainerID] = i
	}

	all := s.store.AllContainers()
	byID := make(map[string]store.Container, len(all))
	for _, c := range all {
		byID[c.ID] = c
	}
	pathOf := func(c store.Container) string {
		var names []string
		seen := map[string]bool{}
		for cur, ok := c, true; ok && !seen[cur.ID]; cur, ok = byID[cur.ParentID] {
			seen[cur.ID] = true
			names = append([]string{cur.Name}, names...)
		}
		return strings.Join(names, " / ")
	}

	var results []QuickSwitchResult
	for _, c := range all {
		pinned := pinSet[c.ID]
		rank, isRecent := recentRank[c.ID]
		path := pathOf(c)

		score := 0
		if nq == "" {
			if !pinned && !isRecent {
				continue
			}
		} else {
			score = matchScore(nq, normalizeSearch(c.Name), normalizeSearch(path))
			if score == 0 {
				continue
			}
		}
		score += workingSetBoost(pinned, isRecent, rank)
		results = append(results, QuickSwitchResult{Container: c, Path: path, Pinned: pinned, Score: score})
	}

	sortQuickSwitch(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// matchScore returns the best match tier for normalised query q against
// normalised name and path, or 0 when nothing matches.
func matchScore(q, name, path string) int {
	switch {
	case name == q:
		return scoreNameExact
	case strings.HasPrefix(name, q):
		return scoreNamePrefix
	case hasWordPrefix(name, q):
		return scoreNameWord
	case strings.Contains(name, q):
		return scoreNameSubstring
	case strings.Contains(path, q):
		return scorePathSubstring
	}
	return 0
}

func hasWordPrefix(s, q string) bool {
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '/' || r == '.'
	}) {
		if strings.HasPrefix(w, q) {
			return true
		}
	}
	return false
}

var diacriticFold = strings.NewReplacer(
	"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n", "ó", "o", "ś", "s", "ź", "z", "ż", "z",
	"ä", "a", "ö", "o", "ü", "u", "ß", "ss", "é", "e", "è", "e", "á", "a", "à", "a", "í", "i",
	"ñ", "n", "ç", "c", "č", "c", "š", "s", "ž", "z", "ř", "r",
)

// normalizeSearch lowercases, trims and folds diacritics (Polish and common
// Latin accents) so that "zolty" matches "Żółty".
func normalizeSearch(s string) string {
	return diacriticFold.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// workingSetBoost returns the ranking bonus for pinned and recent containers.
// Newer recents (lower rank) get a slightly larger boost.
func workingSetBoost(pinned, isRecent bool, rank int) int {
	boost := 0
	if pinned {
		boost += boostPinned
	}
	if isRecent {
		boost += boostRecent - rank*boostRecent/(store.MaxRecentContainers+1)
	}
	return boost
}

// sortQuickSwitch orders by score, then shorter name, then name.
func sortQuickSwitch(results []QuickSwitchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if len(a.Container.Name) != len(b.Container.Name) {
			return len(a.Container.Name) < len(b.Container.Name)
		}
		return a.Container.Name < b.Container.Name
	})
}
