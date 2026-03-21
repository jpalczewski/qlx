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
