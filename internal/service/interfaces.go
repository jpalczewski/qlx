package service

import "github.com/erxyi/qlx/internal/store"

// Saveable describes a store that can persist its state to disk.
type Saveable interface {
	Save() error
}

// ItemStore defines item-related store operations.
type ItemStore interface {
	GetItem(id string) *store.Item
	CreateItem(containerID, name, desc string, qty int) *store.Item
	UpdateItem(id, name, desc string, qty int) (*store.Item, error)
	DeleteItem(id string) error
	MoveItem(id, containerID string) error
}

// ContainerStore defines container-related store operations.
type ContainerStore interface {
	GetContainer(id string) *store.Container
	CreateContainer(parentID, name, desc string) *store.Container
	UpdateContainer(id, name, desc string) (*store.Container, error)
	DeleteContainer(id string) error
	MoveContainer(id, newParentID string) error
	ContainerChildren(parentID string) []store.Container
	ContainerItems(containerID string) []store.Item
	ContainerPath(id string) []store.Container
}

// TagStore defines tag-related store operations.
type TagStore interface {
	GetTag(id string) *store.Tag
	CreateTag(parentID, name string) *store.Tag
	UpdateTag(id, name string) (*store.Tag, error)
	DeleteTag(id string) error
	MoveTag(id, newParentID string) error
	AllTags() []store.Tag
	TagChildren(parentID string) []store.Tag
	TagPath(id string) []store.Tag
	TagDescendants(id string) []string
	AddItemTag(itemID, tagID string) error
	RemoveItemTag(itemID, tagID string) error
	AddContainerTag(containerID, tagID string) error
	RemoveContainerTag(containerID, tagID string) error
}

// BulkStore defines bulk operation store methods.
type BulkStore interface {
	BulkMove(itemIDs, containerIDs []string, targetID string) []store.BulkError
	BulkDelete(itemIDs, containerIDs []string) ([]string, []store.BulkError)
	BulkAddTag(itemIDs, containerIDs []string, tagID string) error
}

// SearchStore defines search-related store operations.
type SearchStore interface {
	SearchContainers(query string) []store.Container
	SearchItems(query string) []store.Item
	SearchTags(query string) []store.Tag
}

// PrinterStore defines printer-related store operations.
type PrinterStore interface {
	AllPrinters() []store.PrinterConfig
	AddPrinter(name, encoder, model, transport, address string) *store.PrinterConfig
	DeletePrinter(id string) error
}
