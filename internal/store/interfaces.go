package store

// ContainerStore defines container-related store operations.
type ContainerStore interface {
	GetContainer(id string) *Container
	CreateContainer(parentID, name, desc, color, icon string) *Container
	UpdateContainer(id, name, desc, color, icon string) (*Container, error)
	DeleteContainer(id string) (string, error) // returns parentID
	MoveContainer(id, newParentID string) error
	ContainerChildren(parentID string) []Container
	ContainerItems(containerID string) []Item
	ContainerPath(id string) []Container
	AllContainers() []Container
}

// ItemStore defines item-related store operations.
type ItemStore interface {
	GetItem(id string) *Item
	CreateItem(containerID, name, desc string, qty int, color, icon string) *Item
	UpdateItem(id, name, desc string, qty int, color, icon string) (*Item, error)
	DeleteItem(id string) (string, error) // returns containerID
	MoveItem(id, containerID string) error
}

// TagStore defines tag-related store operations.
type TagStore interface {
	GetTag(id string) *Tag
	CreateTag(parentID, name, color, icon string) *Tag
	UpdateTag(id, name, color, icon string) (*Tag, error)
	DeleteTag(id string) (string, error) // returns parentID
	MoveTag(id, newParentID string) error
	AllTags() []Tag
	TagChildren(parentID string) []Tag
	TagPath(id string) []Tag
	TagDescendants(id string) []string
	AddItemTag(itemID, tagID string) error
	RemoveItemTag(itemID, tagID string) error
	AddContainerTag(containerID, tagID string) error
	RemoveContainerTag(containerID, tagID string) error
	ItemsByTag(tagID string) []Item
	ContainersByTag(tagID string) []Container
	ResolveTagIDs(ids []string) []Tag
	TagItemStats(id string) (int, int, error)
}

// BulkStore defines bulk operation store methods.
type BulkStore interface {
	BulkMove(itemIDs, containerIDs []string, targetID string) []BulkError
	BulkDelete(itemIDs, containerIDs []string) ([]string, []BulkError)
	BulkAddTag(itemIDs, containerIDs []string, tagID string) error
}

// SearchStore defines search-related store operations.
type SearchStore interface {
	SearchContainers(query string) []Container
	SearchItems(query string) []Item
	SearchTags(query string) []Tag
}

// PrinterStore defines printer-related store operations.
type PrinterStore interface {
	AllPrinters() []PrinterConfig
	GetPrinter(id string) *PrinterConfig
	AddPrinter(name, encoder, model, transport, address string) *PrinterConfig
	DeletePrinter(id string) error
	UpdatePrinterOffset(id string, offsetX, offsetY int) error
}

// TemplateStore defines template-related store operations.
type TemplateStore interface {
	AllTemplates() []Template
	GetTemplate(id string) *Template
	CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
	UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error)
	DeleteTemplate(id string) error
}

// ExportStore defines export-related store operations.
type ExportStore interface {
	ExportData() (map[string]*Container, map[string]*Item)
	AllItems() []Item
	AllContainers() []Container
	ExportItems(containerID string, recursive bool) ([]ExportItem, error)
	ExportContainerTree(containerID string) ([]Container, error)
}
