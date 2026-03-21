package dto

// BulkIDEntry identifies a single entity in a bulk operation.
type BulkIDEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "container" or "item"
}

// BulkMoveRequest is the request body for bulk move operations.
type BulkMoveRequest struct {
	IDs               []BulkIDEntry `json:"ids"`
	TargetContainerID string        `json:"target_container_id"`
}

// BulkDeleteRequest is the request body for bulk delete operations.
type BulkDeleteRequest struct {
	IDs []BulkIDEntry `json:"ids"`
}

// BulkTagsRequest is the request body for bulk tag operations.
type BulkTagsRequest struct {
	IDs   []BulkIDEntry `json:"ids"`
	TagID string        `json:"tag_id"`
}

// SplitBulkIDs separates a slice of BulkIDEntry into item IDs and container IDs.
func SplitBulkIDs(entries []BulkIDEntry) (itemIDs, containerIDs []string) {
	for _, e := range entries {
		switch e.Type {
		case "item":
			itemIDs = append(itemIDs, e.ID)
		case "container":
			containerIDs = append(containerIDs, e.ID)
		}
	}
	return
}
