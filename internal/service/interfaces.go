package service

import (
	"errors"

	"github.com/erxyi/qlx/internal/store"
)

// ErrNotFound is a generic "not found" error for use in handlers.
var ErrNotFound = errors.New("not found")

// Saveable describes a store that can persist its state to disk.
type Saveable interface {
	Save() error
}

// ItemStore defines item-related store operations.
type ItemStore interface {
	GetItem(id string) *store.Item
	CreateItem(containerID, name, desc string, qty int, color, icon string) *store.Item
	UpdateItem(id, name, desc string, qty int, color, icon string) (*store.Item, error)
	DeleteItem(id string) error
	MoveItem(id, containerID string) error
}

// ContainerStore defines container-related store operations.
type ContainerStore interface {
	GetContainer(id string) *store.Container
	CreateContainer(parentID, name, desc, color, icon string) *store.Container
	UpdateContainer(id, name, desc, color, icon string) (*store.Container, error)
	DeleteContainer(id string) error
	MoveContainer(id, newParentID string) error
	ContainerChildren(parentID string) []store.Container
	ContainerItems(containerID string) []store.Item
	ContainerPath(id string) []store.Container
	AllContainers() []store.Container
}

// TagStore defines tag-related store operations.
type TagStore interface {
	GetTag(id string) *store.Tag
	CreateTag(parentID, name, color, icon string) *store.Tag
	UpdateTag(id, name, color, icon string) (*store.Tag, error)
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
	ItemsByTag(tagID string) []store.Item
	ContainersByTag(tagID string) []store.Container
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

// TemplateStore defines template-related store operations.
type TemplateStore interface {
	AllTemplates() []store.Template
	GetTemplate(id string) *store.Template
	CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) *store.Template
	SaveTemplate(t store.Template)
	DeleteTemplate(id string)
}

// AssetStore defines asset-related store operations.
type AssetStore interface {
	SaveAsset(name, mimeType string, data []byte) (*store.Asset, error)
	GetAsset(id string) *store.Asset
	AssetData(id string) ([]byte, error)
}

// ExportStore defines export-related store operations.
type ExportStore interface {
	ExportData() (map[string]*store.Container, map[string]*store.Item)
	AllItems() []store.Item
	AllContainers() []store.Container
}
