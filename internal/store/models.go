package store

import "time"

type Container struct {
	ID          string    `json:"id"`
	ParentID    string    `json:"parent_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Item struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type PrinterConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Encoder   string `json:"encoder"`
	Model     string `json:"model"`
	Transport string `json:"transport"`
	Address   string `json:"address"`
}

// Template defines a reusable label layout.
type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Tags      []string  `json:"tags"`
	Target    string    `json:"target"`     // "universal" or "printer:B1"
	WidthMM   float64   `json:"width_mm"`   // universal only
	HeightMM  float64   `json:"height_mm"`  // universal only
	WidthPx   int       `json:"width_px"`   // printer-specific only
	HeightPx  int       `json:"height_px"`  // printer-specific only
	Elements  string    `json:"elements"`   // JSON array of QLX elements
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Asset holds metadata for an uploaded image. Binary data stored on disk.
type Asset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}
