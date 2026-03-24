package store

import "time"

// ExportItem is a denormalized item for export — includes resolved tag names.
type ExportItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    int       `json:"quantity"`
	ContainerID string    `json:"container_id"`
	TagNames    []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
}
