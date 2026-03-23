package store

// Store is the aggregate interface for all store operations.
type Store interface {
	ContainerStore
	ItemStore
	TagStore
	BulkStore
	SearchStore
	PrinterStore
	TemplateStore
	ExportStore
	Close() error
}
