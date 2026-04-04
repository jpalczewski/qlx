package handler

import (
	"net/http"
	"sort"

	"github.com/erxyi/qlx/internal/service"
)

// StatsHandler serves the /stats page with inventory and tag usage statistics.
type StatsHandler struct {
	inventory *service.InventoryService
	tags      *service.TagService
	resp      Responder
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(inv *service.InventoryService, tags *service.TagService, resp Responder) *StatsHandler {
	return &StatsHandler{inventory: inv, tags: tags, resp: resp}
}

// RegisterRoutes registers the stats route.
func (h *StatsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", h.Page)
}

// Page handles GET /stats.
func (h *StatsHandler) Page(w http.ResponseWriter, r *http.Request) {
	vm := h.buildVM()
	h.resp.Respond(w, r, http.StatusOK, vm, "stats", func() any { return vm })
}

// buildVM computes statistics from the current store state.
func (h *StatsHandler) buildVM() StatsViewModel {
	containers := h.inventory.AllContainers()
	items := h.inventory.AllItems()
	allTags := h.tags.AllTags()

	rootCount := 0
	for _, c := range containers {
		if c.ParentID == "" {
			rootCount++
		}
	}

	totalQty := 0
	for _, it := range items {
		totalQty += it.Quantity
	}

	tagStats := make([]TagStat, 0, len(allTags))
	for _, t := range allTags {
		// TagItemStats returns (itemCount, totalQty, err); qty is not relevant for the stats page.
		itemCount, _, err := h.tags.TagItemStats(t.ID)
		if err != nil {
			continue
		}
		// TODO: replace per-tag ContainersByTag calls with a bulk query when tag counts grow large.
		containerCount := len(h.tags.ContainersByTag(t.ID))
		tagStats = append(tagStats, TagStat{
			Name:           t.Name,
			Color:          t.Color,
			Icon:           t.Icon,
			ItemCount:      itemCount,
			ContainerCount: containerCount,
			TotalUses:      itemCount + containerCount,
		})
	}

	sort.SliceStable(tagStats, func(i, j int) bool {
		return tagStats[i].TotalUses > tagStats[j].TotalUses
	})
	if len(tagStats) > 20 {
		tagStats = tagStats[:20]
	}

	return StatsViewModel{
		Containers:     len(containers),
		RootContainers: rootCount,
		Items:          len(items),
		TotalQty:       totalQty,
		Tags:           tagStats,
	}
}
